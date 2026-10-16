package notify

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMQTTTimeout = 5 * time.Second

	// MQTT 3.1.1 Control Packet Types
	packetCONNECT    byte = 0x10
	packetCONNACK    byte = 0x20
	packetPUBLISH    byte = 0x30
	packetPUBACK     byte = 0x40
	packetDISCONNECT byte = 0xE0
)

// MQTTClient is a lightweight, pure-Go MQTT 3.1.1 protocol client with CGO-free networking.
type MQTTClient struct {
	timeout time.Duration
}

// NewMQTTClient creates a pure-Go MQTT 3.1.1 client with an enforced 5s timeout.
func NewMQTTClient() *MQTTClient {
	return &MQTTClient{
		timeout: DefaultMQTTTimeout,
	}
}

// MQTTConnectOptions contains broker connection parameters.
type MQTTConnectOptions struct {
	Host     string
	Port     int
	ClientID string
	Username string
	Password string
}

// PublishQoS1 connects to the broker, sends CONNECT, publishes a QoS 1 message,
// waits for PUBACK, sends DISCONNECT, and closes the connection.
func (c *MQTTClient) PublishQoS1(ctx context.Context, opts MQTTConnectOptions, topic string, payload []byte) error {
	return c.publish(ctx, opts, topic, payload, 1)
}

// PublishQoS0 publishes a QoS 0 message without waiting for PUBACK.
func (c *MQTTClient) PublishQoS0(ctx context.Context, opts MQTTConnectOptions, topic string, payload []byte) error {
	return c.publish(ctx, opts, topic, payload, 0)
}

func (c *MQTTClient) publish(ctx context.Context, opts MQTTConnectOptions, topic string, payload []byte, qos byte) error {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return errors.New("MQTT broker host cannot be empty")
	}
	port := opts.Port
	if port <= 0 {
		port = DefaultMQTTBrokerPort
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		topic = DefaultMQTTTopic
	}

	clientID := strings.TrimSpace(opts.ClientID)
	if clientID == "" {
		randomSuffix := make([]byte, 4)
		_, _ = rand.Read(randomSuffix)
		clientID = "sovereign-" + hex.EncodeToString(randomSuffix)
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	dialer := &net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("connection timed out while reaching MQTT broker")
		}
		return fmt.Errorf("failed to reach MQTT broker at %s: %s", addr, sanitizeNetworkError(err))
	}
	defer conn.Close()

	// Apply strict I/O deadline for all protocol exchanges
	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	// 1. Send CONNECT packet
	connectPacket := buildConnectPacket(clientID, opts.Username, opts.Password)
	if _, err := conn.Write(connectPacket); err != nil {
		return fmt.Errorf("failed to send MQTT CONNECT: %w", err)
	}

	// 2. Read CONNACK packet
	if err := readConnack(conn); err != nil {
		return err
	}

	// 3. Send PUBLISH packet
	var packetID uint16 = 1
	pubPacket := buildPublishPacket(topic, payload, qos, packetID)
	if _, err := conn.Write(pubPacket); err != nil {
		return fmt.Errorf("failed to send MQTT PUBLISH: %w", err)
	}

	// 4. If QoS 1, wait for PUBACK
	if qos == 1 {
		if err := readPuback(conn, packetID); err != nil {
			return err
		}
	}

	// 5. Send DISCONNECT packet cleanly
	disconnectPacket := []byte{packetDISCONNECT, 0x00}
	_, _ = conn.Write(disconnectPacket)

	return nil
}

// buildConnectPacket constructs an MQTT 3.1.1 CONNECT packet.
func buildConnectPacket(clientID, username, password string) []byte {
	var varHeaderAndPayload []byte

	// Variable header: Protocol Name ("MQTT")
	varHeaderAndPayload = append(varHeaderAndPayload, 0x00, 0x04)
	varHeaderAndPayload = append(varHeaderAndPayload, []byte("MQTT")...)
	// Protocol Level: 4 (MQTT 3.1.1)
	varHeaderAndPayload = append(varHeaderAndPayload, 0x04)

	// Connect Flags: Clean Session (bit 1)
	var connectFlags byte = 0x02
	if username != "" {
		connectFlags |= 0x80
		if password != "" {
			connectFlags |= 0x40
		}
	}
	varHeaderAndPayload = append(varHeaderAndPayload, connectFlags)

	// Keep Alive: 60 seconds
	varHeaderAndPayload = append(varHeaderAndPayload, 0x00, 0x3C)

	// Payload: Client Identifier
	clientBytes := []byte(clientID)
	varHeaderAndPayload = appendStringWithLength(varHeaderAndPayload, clientBytes)

	// Payload: Username and Password (password only allowed if username is set per MQTT 3.1.1)
	if username != "" {
		varHeaderAndPayload = appendStringWithLength(varHeaderAndPayload, []byte(username))
		if password != "" {
			varHeaderAndPayload = appendStringWithLength(varHeaderAndPayload, []byte(password))
		}
	}

	// Build fixed header
	var packet []byte
	packet = append(packet, packetCONNECT)
	packet = append(packet, encodeRemainingLength(len(varHeaderAndPayload))...)
	packet = append(packet, varHeaderAndPayload...)

	return packet
}

// buildPublishPacket constructs a PUBLISH packet for QoS 0 or QoS 1.
func buildPublishPacket(topic string, payload []byte, qos byte, packetID uint16) []byte {
	var varHeaderAndPayload []byte

	// Topic Name with 2-byte length
	topicBytes := []byte(topic)
	varHeaderAndPayload = appendStringWithLength(varHeaderAndPayload, topicBytes)

	// Packet Identifier if QoS > 0
	if qos > 0 {
		pidBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(pidBytes, packetID)
		varHeaderAndPayload = append(varHeaderAndPayload, pidBytes...)
	}

	// Message payload
	varHeaderAndPayload = append(varHeaderAndPayload, payload...)

	// Fixed header byte: (3 << 4) | (qos << 1)
	fixedByte := packetPUBLISH | ((qos & 0x03) << 1)

	var packet []byte
	packet = append(packet, fixedByte)
	packet = append(packet, encodeRemainingLength(len(varHeaderAndPayload))...)
	packet = append(packet, varHeaderAndPayload...)

	return packet
}

func readConnack(r io.Reader) error {
	header := make([]byte, 1)
	if _, err := io.ReadFull(r, header); err != nil {
		return fmt.Errorf("failed to read MQTT CONNACK header: %w", err)
	}
	if header[0] != packetCONNACK {
		return fmt.Errorf("unexpected MQTT response packet: expected 0x20 (CONNACK), got 0x%02X", header[0])
	}

	remLen, err := decodeRemainingLength(r)
	if err != nil {
		return fmt.Errorf("failed to read CONNACK length: %w", err)
	}
	if remLen != 2 {
		return fmt.Errorf("invalid CONNACK remaining length: %d (expected 2)", remLen)
	}

	body := make([]byte, remLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return fmt.Errorf("failed to read CONNACK payload: %w", err)
	}

	returnCode := body[1]
	switch returnCode {
	case 0:
		return nil
	case 1:
		return errors.New("MQTT connection refused: unacceptable protocol version")
	case 2:
		return errors.New("MQTT connection refused: client identifier rejected")
	case 3:
		return errors.New("MQTT connection refused: broker server unavailable")
	case 4:
		return errors.New("MQTT connection refused: invalid username or password")
	case 5:
		return errors.New("MQTT connection refused: not authorized")
	default:
		return fmt.Errorf("MQTT connection refused with code %d", returnCode)
	}
}

func readPuback(r io.Reader, expectedPID uint16) error {
	header := make([]byte, 1)
	if _, err := io.ReadFull(r, header); err != nil {
		return fmt.Errorf("failed to read MQTT PUBACK header: %w", err)
	}
	if header[0] != packetPUBACK {
		return fmt.Errorf("unexpected MQTT response packet: expected 0x40 (PUBACK), got 0x%02X", header[0])
	}

	remLen, err := decodeRemainingLength(r)
	if err != nil {
		return fmt.Errorf("failed to read PUBACK length: %w", err)
	}
	if remLen != 2 {
		return fmt.Errorf("invalid PUBACK remaining length: %d (expected 2)", remLen)
	}

	body := make([]byte, remLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return fmt.Errorf("failed to read PUBACK payload: %w", err)
	}

	pid := binary.BigEndian.Uint16(body[0:2])
	if pid != expectedPID {
		return fmt.Errorf("MQTT PUBACK packet ID mismatch: expected %d, got %d", expectedPID, pid)
	}

	return nil
}

func appendStringWithLength(buf []byte, strBytes []byte) []byte {
	lenBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBytes, uint16(len(strBytes)))
	buf = append(buf, lenBytes...)
	buf = append(buf, strBytes...)
	return buf
}

func encodeRemainingLength(length int) []byte {
	var encoded []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encoded = append(encoded, digit)
		if length == 0 {
			break
		}
	}
	return encoded
}

func decodeRemainingLength(r io.Reader) (int, error) {
	multiplier := 1
	value := 0
	b := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, err
		}
		encodedByte := b[0]
		value += int(encodedByte&127) * multiplier
		if (encodedByte & 128) == 0 {
			break
		}
		multiplier *= 128
		if multiplier > 128*128*128 {
			return 0, errors.New("malformed remaining length")
		}
	}
	return value, nil
}
