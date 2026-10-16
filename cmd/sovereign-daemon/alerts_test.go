package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sovereign/internal/notify"
	"sovereign/internal/store"
)

func TestAlerts_Config_GetAndPut_Redaction(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Initial GET on unconfigured server should return 200 with default fields
	req := httptest.NewRequest("GET", "/api/v1/alerts/config", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET config, got %d: %s", w.Code, w.Body.String())
	}

	var initialCfg notify.AlertConfig
	if err := json.Unmarshal(w.Body.Bytes(), &initialCfg); err != nil {
		t.Fatalf("failed to decode alert config: %v", err)
	}
	if initialCfg.MQTTBrokerPort != notify.DefaultMQTTBrokerPort {
		t.Errorf("expected default MQTT port %d, got %d", notify.DefaultMQTTBrokerPort, initialCfg.MQTTBrokerPort)
	}

	// 2. PUT to update configuration with secrets
	updateBody := UpdateAlertConfigRequest{
		HAWebhookURL:           "http://ha.local:8123/api/webhook/sovereign",
		MQTTBrokerHost:         "mqtt.local",
		MQTTBrokerPort:         1883,
		MQTTTopic:              "sovereign/alerts/documents",
		MQTTUsername:           "mqtt_user",
		MQTTPassword:           "my-secret-mqtt-pw",
		AlertEmail:             "alerts@family.net",
		SMTPHost:               "smtp.mailgun.org",
		SMTPPort:               587,
		SMTPUsername:           "smtp_user",
		SMTPPassword:           "my-secret-smtp-pw",
		SMTPFrom:               "alerts@sovereign.local",
		DeadlineAlertThreshold: 14,
	}

	bodyBytes, _ := json.Marshal(updateBody)
	putReq := httptest.NewRequest("PUT", "/api/v1/alerts/config", bytes.NewReader(bodyBytes))
	putW := httptest.NewRecorder()
	srv.router.ServeHTTP(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PUT config, got %d: %s", putW.Code, putW.Body.String())
	}

	var updatedCfg notify.AlertConfig
	if err := json.Unmarshal(putW.Body.Bytes(), &updatedCfg); err != nil {
		t.Fatalf("failed to decode updated config: %v", err)
	}

	// Invariant: Passwords must be redacted
	if updatedCfg.MQTTPassword != notify.PasswordMask {
		t.Errorf("expected redacted MQTT password %q, got %q", notify.PasswordMask, updatedCfg.MQTTPassword)
	}
	if updatedCfg.SMTPPassword != notify.PasswordMask {
		t.Errorf("expected redacted SMTP password %q, got %q", notify.PasswordMask, updatedCfg.SMTPPassword)
	}
	if updatedCfg.HAWebhookURL != "http://ha.local:8123/api/webhook/sovereign" {
		t.Errorf("expected updated HA webhook URL, got %q", updatedCfg.HAWebhookURL)
	}

	// 3. PUT with masked passwords preserves existing passwords
	updateBody2 := UpdateAlertConfigRequest{
		HAWebhookURL:           "http://ha.local:8123/api/webhook/sovereign-v2",
		MQTTBrokerHost:         "mqtt.local",
		MQTTPassword:           notify.PasswordMask, // Masked!
		AlertEmail:             "alerts@family.net",
		SMTPHost:               "smtp.mailgun.org",
		SMTPPassword:           "", // Empty!
		DeadlineAlertThreshold: 20,
	}
	bodyBytes2, _ := json.Marshal(updateBody2)
	putReq2 := httptest.NewRequest("PUT", "/api/v1/alerts/config", bytes.NewReader(bodyBytes2))
	putW2 := httptest.NewRecorder()
	srv.router.ServeHTTP(putW2, putReq2)

	if putW2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on second PUT, got %d: %s", putW2.Code, putW2.Body.String())
	}

	// Directly inspect DB to verify passwords were preserved
	storedCfg, err := notify.LoadConfig(context.Background(), srv.db)
	if err != nil {
		t.Fatalf("LoadConfig from DB failed: %v", err)
	}
	if storedCfg.MQTTPassword != "my-secret-mqtt-pw" {
		t.Errorf("expected preserved MQTT password, got %q", storedCfg.MQTTPassword)
	}
	if storedCfg.SMTPPassword != "my-secret-smtp-pw" {
		t.Errorf("expected preserved SMTP password, got %q", storedCfg.SMTPPassword)
	}
	if storedCfg.DeadlineAlertThreshold != 20 {
		t.Errorf("expected updated threshold 20, got %d", storedCfg.DeadlineAlertThreshold)
	}
}

func TestAlerts_TestWebhook_Endpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	var received bool
	mockHA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockHA.Close()

	// 1. Success with explicit webhook URL
	reqBody := TestWebhookRequest{WebhookURL: mockHA.URL}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/alerts/test/webhook", bytes.NewReader(b))
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if !received {
		t.Errorf("expected mock webhook to receive alert payload")
	}

	var resp AlertTestResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}

	// 2. Failure with empty URL when none configured in DB
	emptyReq := httptest.NewRequest("POST", "/api/v1/alerts/test/webhook", bytes.NewReader([]byte(`{}`)))
	emptyW := httptest.NewRecorder()
	srv.router.ServeHTTP(emptyW, emptyReq)

	if emptyW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when webhook URL unconfigured, got %d", emptyW.Code)
	}

	// 3. Failure with unreachable host
	badReqBody := TestWebhookRequest{WebhookURL: "http://127.0.0.1:54325/unreachable"}
	bBad, _ := json.Marshal(badReqBody)
	badReq := httptest.NewRequest("POST", "/api/v1/alerts/test/webhook", bytes.NewReader(bBad))
	badW := httptest.NewRecorder()
	srv.router.ServeHTTP(badW, badReq)

	if badW.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway on unreachable host, got %d", badW.Code)
	}
}

func TestAlerts_TestMQTT_Endpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Missing host returns 400
	req := httptest.NewRequest("POST", "/api/v1/alerts/test/mqtt", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing broker host, got %d", w.Code)
	}

	// 2. Success with running mock MQTT broker
	host, port, cleanup := startMockBroker(t)
	defer cleanup()

	mqttReq := TestMQTTRequest{
		BrokerHost: host,
		BrokerPort: port,
		Topic:      "sovereign/alerts/documents",
	}
	b, _ := json.Marshal(mqttReq)
	req2 := httptest.NewRequest("POST", "/api/v1/alerts/test/mqtt", bytes.NewReader(b))
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on mock MQTT test, got %d: %s", w2.Code, w2.Body.String())
	}

	// 3. Unreachable host returns 502 Bad Gateway
	badReq := TestMQTTRequest{
		BrokerHost: "127.0.0.1",
		BrokerPort: 54326,
	}
	bBad, _ := json.Marshal(badReq)
	req3 := httptest.NewRequest("POST", "/api/v1/alerts/test/mqtt", bytes.NewReader(bBad))
	w3 := httptest.NewRecorder()
	srv.router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway on unreachable broker, got %d", w3.Code)
	}
}

func TestAlerts_TestEmail_Endpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Missing email or host returns 400
	req := httptest.NewRequest("POST", "/api/v1/alerts/test/email", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing SMTP params, got %d", w.Code)
	}

	// 2. Success with mock SMTP server
	host, port, cleanup := startMockSMTP(t)
	defer cleanup()

	emailReq := TestEmailRequest{
		AlertEmail: "alex@family.net",
		SMTPHost:   host,
		SMTPPort:   port,
		SMTPFrom:   "alerts@sovereign.local",
	}
	b, _ := json.Marshal(emailReq)
	req2 := httptest.NewRequest("POST", "/api/v1/alerts/test/email", bytes.NewReader(b))
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on mock SMTP test, got %d: %s", w2.Code, w2.Body.String())
	}

	// 3. Unreachable SMTP host returns 502 Bad Gateway
	badReq := TestEmailRequest{
		AlertEmail: "alex@family.net",
		SMTPHost:   "127.0.0.1",
		SMTPPort:   54327,
	}
	bBad, _ := json.Marshal(badReq)
	req3 := httptest.NewRequest("POST", "/api/v1/alerts/test/email", bytes.NewReader(bBad))
	w3 := httptest.NewRecorder()
	srv.router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway on unreachable SMTP server, got %d", w3.Code)
	}
}

func TestAlerts_Dispatch_Endpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Add pending deadline within threshold
	amt := 75.00
	dl := &store.DocumentDeadline{
		DocumentID: "doc-gas",
		Title:      "Gas Utility Bill",
		DueDate:    now.AddDate(0, 0, 5).Format("2006-01-02"),
		Amount:     &amt,
		Currency:   "EUR",
		Snippet:    "Total gas bill due: 75.00 EUR",
		Status:     store.DeadlineStatusPending,
	}
	if err := srv.deadlineStore.Save(ctx, dl); err != nil {
		t.Fatalf("failed to save deadline: %v", err)
	}

	// Start mock servers
	var webhookDelivered bool
	mockHA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookDelivered = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockHA.Close()

	mqttHost, mqttPort, cleanupMQTT := startMockBroker(t)
	defer cleanupMQTT()

	smtpHost, smtpPort, cleanupSMTP := startMockSMTP(t)
	defer cleanupSMTP()

	// Save configuration
	cfg := &notify.AlertConfig{
		HAWebhookURL:           mockHA.URL,
		MQTTBrokerHost:         mqttHost,
		MQTTBrokerPort:         mqttPort,
		AlertEmail:             "alex@family.net",
		SMTPHost:               smtpHost,
		SMTPPort:               smtpPort,
		DeadlineAlertThreshold: 14,
	}
	if err := notify.SaveConfig(ctx, srv.db, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Trigger dispatch endpoint
	req := httptest.NewRequest("POST", "/api/v1/alerts/dispatch", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on dispatch, got %d: %s", w.Code, w.Body.String())
	}

	var resp AlertDispatchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal dispatch response: %v", err)
	}
	if resp.DispatchedCount != 1 {
		t.Errorf("expected dispatched_count 1, got %d", resp.DispatchedCount)
	}
	if !webhookDelivered {
		t.Errorf("expected webhook delivery to mock HA server")
	}

	// Verify deadline was updated to 'notified'
	saved, err := srv.deadlineStore.Get(ctx, dl.ID)
	if err != nil {
		t.Fatalf("Get deadline failed: %v", err)
	}
	if saved.Status != store.DeadlineStatusNotified {
		t.Errorf("expected deadline status 'notified', got %q", saved.Status)
	}

	// Second dispatch should dispatch 0 (already notified)
	req2 := httptest.NewRequest("POST", "/api/v1/alerts/dispatch", nil)
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on second dispatch, got %d", w2.Code)
	}
	var resp2 AlertDispatchResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.DispatchedCount != 0 {
		t.Errorf("expected dispatched_count 0 on second run, got %d", resp2.DispatchedCount)
	}
}

// Helpers for mock broker and SMTP
func startMockBroker(t *testing.T) (string, int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock broker: %v", err)
	}

	tcpAddr := listener.Addr().(*net.TCPAddr)
	stopCh := make(chan struct{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stopCh:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))

				// Read CONNECT (header 0x10)
				header := make([]byte, 1)
				if _, err := io.ReadFull(c, header); err != nil || header[0] != 0x10 {
					return
				}
				remLen := decodeTestLen(c)
				body := make([]byte, remLen)
				_, _ = io.ReadFull(c, body)

				// Send CONNACK (0x20, 0x02, 0x00, 0x00)
				_, _ = c.Write([]byte{0x20, 0x02, 0x00, 0x00})

				// Read PUBLISH (header 0x30 or 0x32)
				if _, err := io.ReadFull(c, header); err != nil {
					return
				}
				qos := (header[0] >> 1) & 0x03
				remLen = decodeTestLen(c)
				pubBody := make([]byte, remLen)
				_, _ = io.ReadFull(c, pubBody)

				// If QoS 1, send PUBACK
				if qos == 1 && remLen >= 4 {
					topicLen := binary.BigEndian.Uint16(pubBody[0:2])
					pidOffset := 2 + int(topicLen)
					if pidOffset+2 <= len(pubBody) {
						pid := pubBody[pidOffset : pidOffset+2]
						_, _ = c.Write([]byte{0x40, 0x02, pid[0], pid[1]})
					}
				}

				// Optional DISCONNECT
				_, _ = io.ReadFull(c, header)
			}(conn)
		}
	}()

	cleanup := func() {
		close(stopCh)
		_ = listener.Close()
	}

	return tcpAddr.IP.String(), tcpAddr.Port, cleanup
}

func decodeTestLen(r io.Reader) int {
	multiplier := 1
	value := 0
	b := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, b); err != nil {
			return 0
		}
		digit := b[0]
		value += int(digit&127) * multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier *= 128
	}
	return value
}

func startMockSMTP(t *testing.T) (string, int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock smtp: %v", err)
	}

	tcpAddr := listener.Addr().(*net.TCPAddr)
	stopCh := make(chan struct{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stopCh:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(c)
				writer := bufio.NewWriter(c)

				_, _ = writer.WriteString("220 mock.local SMTP Service Ready\r\n")
				_ = writer.Flush()

				inData := false
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					trimmed := strings.TrimRight(line, "\r\n")
					if inData {
						if trimmed == "." {
							inData = false
							_, _ = writer.WriteString("250 OK: delivered\r\n")
							_ = writer.Flush()
						}
						continue
					}

					cmd := strings.ToUpper(trimmed)
					switch {
					case strings.HasPrefix(cmd, "HELO") || strings.HasPrefix(cmd, "EHLO"):
						_, _ = writer.WriteString("250-mock.local Hello\r\n250 8BITMIME\r\n")
						_ = writer.Flush()
					case strings.HasPrefix(cmd, "MAIL FROM:"):
						_, _ = writer.WriteString("250 2.1.0 Sender OK\r\n")
						_ = writer.Flush()
					case strings.HasPrefix(cmd, "RCPT TO:"):
						_, _ = writer.WriteString("250 2.1.5 Recipient OK\r\n")
						_ = writer.Flush()
					case cmd == "DATA":
						inData = true
						_, _ = writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
						_ = writer.Flush()
					case cmd == "QUIT":
						_, _ = writer.WriteString("221 2.0.0 Bye\r\n")
						_ = writer.Flush()
						return
					default:
						_, _ = writer.WriteString("500 Command unrecognized\r\n")
						_ = writer.Flush()
					}
				}
			}(conn)
		}
	}()

	cleanup := func() {
		close(stopCh)
		_ = listener.Close()
	}

	return tcpAddr.IP.String(), tcpAddr.Port, cleanup
}
