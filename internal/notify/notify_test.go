package notify

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sovereign/internal/store"
)

func TestAlertConfig_PersistenceAndRedaction(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "config_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Initial load on empty settings should yield defaults
	cfg, err := LoadConfig(ctx, db)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.MQTTBrokerPort != DefaultMQTTBrokerPort {
		t.Errorf("expected default MQTT port %d, got %d", DefaultMQTTBrokerPort, cfg.MQTTBrokerPort)
	}
	if cfg.MQTTTopic != DefaultMQTTTopic {
		t.Errorf("expected default MQTT topic %q, got %q", DefaultMQTTTopic, cfg.MQTTTopic)
	}
	if cfg.DeadlineAlertThreshold != DefaultDeadlineAlertThreshold {
		t.Errorf("expected default threshold %d, got %d", DefaultDeadlineAlertThreshold, cfg.DeadlineAlertThreshold)
	}

	// Save initial config with credentials
	cfg.HAWebhookURL = "http://homeassistant.local:8123/api/webhook/sovereign"
	cfg.MQTTBrokerHost = "192.168.1.100"
	cfg.MQTTBrokerPort = 1883
	cfg.MQTTPassword = "super-secret-mqtt-password"
	cfg.AlertEmail = "family@sovereign.local"
	cfg.SMTPHost = "smtp.local"
	cfg.SMTPPassword = "super-secret-smtp-password"

	if err := SaveConfig(ctx, db, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify Redacted() does not leak passwords
	redacted := cfg.Redacted()
	if redacted.MQTTPassword != PasswordMask {
		t.Errorf("expected redacted MQTT password %q, got %q", PasswordMask, redacted.MQTTPassword)
	}
	if redacted.SMTPPassword != PasswordMask {
		t.Errorf("expected redacted SMTP password %q, got %q", PasswordMask, redacted.SMTPPassword)
	}

	// Updating config with masked passwords must preserve original stored passwords
	updatedCfg := &AlertConfig{
		HAWebhookURL:           "http://homeassistant.local:8123/api/webhook/sovereign-updated",
		MQTTBrokerHost:         "192.168.1.101",
		MQTTBrokerPort:         1883,
		MQTTPassword:           PasswordMask, // Masked!
		AlertEmail:             "family2@sovereign.local",
		SMTPHost:               "smtp2.local",
		SMTPPassword:           "", // Empty!
		DeadlineAlertThreshold: 21,
	}

	if err := SaveConfig(ctx, db, updatedCfg); err != nil {
		t.Fatalf("SaveConfig with masked passwords failed: %v", err)
	}

	loaded, err := LoadConfig(ctx, db)
	if err != nil {
		t.Fatalf("LoadConfig after update failed: %v", err)
	}
	if loaded.MQTTPassword != "super-secret-mqtt-password" {
		t.Errorf("expected preserved MQTT password, got %q", loaded.MQTTPassword)
	}
	if loaded.SMTPPassword != "super-secret-smtp-password" {
		t.Errorf("expected preserved SMTP password, got %q", loaded.SMTPPassword)
	}
	if loaded.HAWebhookURL != "http://homeassistant.local:8123/api/webhook/sovereign-updated" {
		t.Errorf("expected updated webhook URL, got %q", loaded.HAWebhookURL)
	}
	if loaded.DeadlineAlertThreshold != 21 {
		t.Errorf("expected updated threshold 21, got %d", loaded.DeadlineAlertThreshold)
	}
}

func TestWebhookClient_SuccessAndErrors(t *testing.T) {
	var receivedPayload WebhookDeadlinePayload
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"received"}`))
	}))
	defer ts.Close()

	client := NewWebhookClient()
	amt := 120.50
	payload := &WebhookDeadlinePayload{
		ID:            "alert-1",
		DocumentID:    "doc-1",
		Title:         "Home Insurance Policy",
		DueDate:       "2026-10-31",
		DaysRemaining: 15,
		Amount:        &amt,
		Currency:      "EUR",
		URL:           "http://sovereign.local/docs/documents/doc-1",
		Snippet:       "Policy renewal due on 2026-10-31 for 120.50 EUR",
	}

	ctx := context.Background()
	if err := client.SendWebhook(ctx, ts.URL, payload); err != nil {
		t.Fatalf("SendWebhook failed: %v", err)
	}

	mu.Lock()
	if receivedPayload.Title != "Home Insurance Policy" || receivedPayload.Event != EventDocumentDeadline {
		t.Errorf("unexpected payload received: %+v", receivedPayload)
	}
	mu.Unlock()

	// Error handling: unreachable target host
	err := client.SendWebhook(ctx, "http://127.0.0.1:54321/nowhere", payload)
	if err == nil {
		t.Errorf("expected error connecting to unreachable host, got nil")
	}
	if !strings.Contains(err.Error(), "webhook endpoint unreachable") {
		t.Errorf("expected sanitized error message, got %q", err.Error())
	}

	// Error handling: HTTP 404
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer errServer.Close()

	err = client.SendWebhook(ctx, errServer.URL, payload)
	if err == nil {
		t.Errorf("expected error on HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error message, got %q", err.Error())
	}
}

// MockMQTTMessage records a published MQTT packet.
type MockMQTTMessage struct {
	Topic   string
	Payload []byte
}

// startMockMQTTBroker spawns a lightweight mock MQTT 3.1.1 broker for testing CONNECT, PUBLISH, PUBACK, DISCONNECT.
func startMockMQTTBroker(t *testing.T, expectedUser, expectedPass string) (string, int, func(), *[]MockMQTTMessage) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock MQTT listener: %v", err)
	}

	tcpAddr := listener.Addr().(*net.TCPAddr)
	stopCh := make(chan struct{})
	var messages []MockMQTTMessage
	var mu sync.Mutex

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

			go handleMockMQTTConn(conn, expectedUser, expectedPass, func(msg MockMQTTMessage) {
				mu.Lock()
				messages = append(messages, msg)
				mu.Unlock()
			})
		}
	}()

	cleanup := func() {
		close(stopCh)
		_ = listener.Close()
	}

	return tcpAddr.IP.String(), tcpAddr.Port, cleanup, &messages
}

func handleMockMQTTConn(conn net.Conn, expectedUser, expectedPass string, onPublish func(MockMQTTMessage)) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// 1. Read CONNECT
	header := make([]byte, 1)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != packetCONNECT {
		return
	}

	remLen, err := decodeRemainingLength(conn)
	if err != nil {
		return
	}

	connectBody := make([]byte, remLen)
	if _, err := io.ReadFull(conn, connectBody); err != nil {
		return
	}

	// Parse flags and return code
	// Protocol Name: 2 bytes len + 4 bytes "MQTT" (idx 0..5), Level: idx 6, Flags: idx 7
	var returnCode byte = 0
	if remLen >= 8 {
		flags := connectBody[7]
		hasUser := (flags & 0x80) != 0
		hasPass := (flags & 0x40) != 0

		if expectedUser != "" && (!hasUser || !hasPass) {
			returnCode = 4 // Bad user name or password
		}
	}

	// Send CONNACK: 0x20, 0x02, 0x00, returnCode
	_, _ = conn.Write([]byte{packetCONNACK, 0x02, 0x00, returnCode})
	if returnCode != 0 {
		return
	}

	// 2. Read PUBLISH
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}

	fixedByte := header[0]
	if (fixedByte >> 4) != 3 { // packetPUBLISH
		return
	}
	qos := (fixedByte >> 1) & 0x03

	remLen, err = decodeRemainingLength(conn)
	if err != nil {
		return
	}

	publishBody := make([]byte, remLen)
	if _, err := io.ReadFull(conn, publishBody); err != nil {
		return
	}

	// If QoS 1, find packet ID and prepare PUBACK
	topicLen := binary.BigEndian.Uint16(publishBody[0:2])
	topic := string(publishBody[2 : 2+int(topicLen)])
	var payload []byte
	var puback []byte
	if qos == 1 && remLen >= 4 {
		pidOffset := 2 + int(topicLen)
		if pidOffset+2 <= len(publishBody) {
			pid := publishBody[pidOffset : pidOffset+2]
			payload = publishBody[pidOffset+2:]
			// Prepare PUBACK: 0x40, 0x02, pid[0], pid[1]
			puback = []byte{packetPUBACK, 0x02, pid[0], pid[1]}
		}
	} else {
		payload = publishBody[2+int(topicLen):]
	}

	if onPublish != nil {
		onPublish(MockMQTTMessage{Topic: topic, Payload: payload})
	}

	if len(puback) > 0 {
		_, _ = conn.Write(puback)
	}

	// 3. Optional DISCONNECT
	_, _ = io.ReadFull(conn, header)
}

func TestMQTTClient_PublishQoS1AndErrors(t *testing.T) {
	host, port, cleanup, _ := startMockMQTTBroker(t, "", "")
	defer cleanup()

	client := NewMQTTClient()
	ctx := context.Background()

	opts := MQTTConnectOptions{
		Host: host,
		Port: port,
	}

	payload := []byte(`{"title":"Upcoming Bill","amount":42.50}`)
	if err := client.PublishQoS1(ctx, opts, "sovereign/alerts/documents", payload); err != nil {
		t.Fatalf("PublishQoS1 to mock broker failed: %v", err)
	}

	// Test QoS 0 as well
	if err := client.PublishQoS0(ctx, opts, "sovereign/alerts/documents", payload); err != nil {
		t.Fatalf("PublishQoS0 to mock broker failed: %v", err)
	}

	// Offline broker error handling
	offlineOpts := MQTTConnectOptions{
		Host: "127.0.0.1",
		Port: 54322,
	}
	err := client.PublishQoS1(ctx, offlineOpts, "test", payload)
	if err == nil {
		t.Errorf("expected error connecting to offline broker, got nil")
	}
	if !strings.Contains(err.Error(), "failed to reach MQTT broker") {
		t.Errorf("expected clean connection error, got %q", err.Error())
	}
}

// startMockSMTPServer spawns a simple local mock SMTP server for testing.
func startMockSMTPServer(t *testing.T, expectedUser, expectedPass string) (string, int, func(), *[]string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock SMTP server: %v", err)
	}

	tcpAddr := listener.Addr().(*net.TCPAddr)
	stopCh := make(chan struct{})
	var receivedMessages []string
	var mu sync.Mutex

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

				// Banner
				_, _ = writer.WriteString("220 sovereign.local Mock SMTP Service Ready\r\n")
				_ = writer.Flush()

				var currentMsg strings.Builder
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
							mu.Lock()
							receivedMessages = append(receivedMessages, currentMsg.String())
							mu.Unlock()
							_, _ = writer.WriteString("250 OK: message queued\r\n")
							_ = writer.Flush()
						} else {
							currentMsg.WriteString(line)
						}
						continue
					}

					cmdUpper := strings.ToUpper(trimmed)
					switch {
					case strings.HasPrefix(cmdUpper, "EHLO") || strings.HasPrefix(cmdUpper, "HELO"):
						_, _ = writer.WriteString("250-sovereign.local Hello\r\n250 8BITMIME\r\n")
						_ = writer.Flush()
					case strings.HasPrefix(cmdUpper, "MAIL FROM:"):
						_, _ = writer.WriteString("250 2.1.0 Sender OK\r\n")
						_ = writer.Flush()
					case strings.HasPrefix(cmdUpper, "RCPT TO:"):
						_, _ = writer.WriteString("250 2.1.5 Recipient OK\r\n")
						_ = writer.Flush()
					case cmdUpper == "DATA":
						inData = true
						currentMsg.Reset()
						_, _ = writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
						_ = writer.Flush()
					case cmdUpper == "QUIT":
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

	return tcpAddr.IP.String(), tcpAddr.Port, cleanup, &receivedMessages
}

func TestEmailClient_DigestAndTestEmail(t *testing.T) {
	host, port, cleanup, receivedMsgs := startMockSMTPServer(t, "", "")
	defer cleanup()

	client := NewEmailClient()
	ctx := context.Background()

	cfg := SMTPConfig{
		Host: host,
		Port: port,
		From: "alerts@sovereign.local",
	}

	amt := 89.90
	items := []DeadlineDigestItem{
		{
			ID:            "item-1",
			DocumentID:    "doc-101",
			Title:         "Broadband Internet Renewal",
			DueDate:       "2026-10-25",
			DaysRemaining: 9,
			Amount:        &amt,
			Currency:      "EUR",
			URL:           "http://sovereign.local/docs/documents/doc-101",
			Snippet:       "Annual fiber renewal rate 89.90 EUR",
		},
	}

	if err := client.SendDigest(ctx, cfg, "alex@family.net", items); err != nil {
		t.Fatalf("SendDigest failed: %v", err)
	}

	if err := client.SendTestEmail(ctx, cfg, "alex@family.net"); err != nil {
		t.Fatalf("SendTestEmail failed: %v", err)
	}

	if len(*receivedMsgs) != 2 {
		t.Fatalf("expected 2 received emails, got %d", len(*receivedMsgs))
	}

	// Verify digest contains HTML and plain-text parts with document link
	digestMail := (*receivedMsgs)[0]
	if !strings.Contains(digestMail, "Broadband Internet Renewal") {
		t.Errorf("digest email missing title")
	}
	if !strings.Contains(digestMail, "http://sovereign.local/docs/documents/doc-101") {
		t.Errorf("digest email missing direct document link")
	}
	if !strings.Contains(digestMail, "multipart/alternative") {
		t.Errorf("expected multipart MIME digest")
	}
}

func TestAlertDispatcher_FullCycleAndEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "dispatcher_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	dStore, err := store.NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	amt := 150.00
	dl := &store.DocumentDeadline{
		DocumentID: "doc-abc",
		Title:      "Electricity Bill",
		DueDate:    "2026-10-23", // 7 days remaining
		Amount:     &amt,
		Currency:   "USD",
		Snippet:    "Total amount due: $150.00",
		Status:     store.DeadlineStatusPending,
	}
	if err := dStore.Save(ctx, dl); err != nil {
		t.Fatalf("failed to save deadline: %v", err)
	}

	dispatcher := NewAlertDispatcher(db, dStore, nil)
	dispatcher.SetNowFunc(func() time.Time { return now })

	// Case 1: No channels configured -> no-op, returns 0, keeps pending
	count, err := dispatcher.DispatchCycle(ctx)
	if err != nil {
		t.Fatalf("DispatchCycle without channels failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 dispatched with no channels configured, got %d", count)
	}
	checkDL, _ := dStore.Get(ctx, dl.ID)
	if checkDL.Status != store.DeadlineStatusPending {
		t.Errorf("expected status to remain 'pending' when no channels configured, got %q", checkDL.Status)
	}

	// Case 2: Configure Webhook, MQTT, and Email channels
	var webhookReceived bool
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	mqttHost, mqttPort, cleanupMQTT, mqttReceived := startMockMQTTBroker(t, "", "")
	defer cleanupMQTT()

	smtpHost, smtpPort, cleanupSMTP, smtpReceived := startMockSMTPServer(t, "", "")
	defer cleanupSMTP()

	cfg := &AlertConfig{
		HAWebhookURL:           webhookServer.URL,
		MQTTBrokerHost:         mqttHost,
		MQTTBrokerPort:         mqttPort,
		AlertEmail:             "alex@family.net",
		SMTPHost:               smtpHost,
		SMTPPort:               smtpPort,
		DeadlineAlertThreshold: 14,
	}
	if err := SaveConfig(ctx, db, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Run dispatch cycle: should deliver to all 3 channels and mark deadline as 'notified'
	count, err = dispatcher.DispatchCycle(ctx)
	if err != nil {
		t.Fatalf("DispatchCycle failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 deadline marked as notified, got %d", count)
	}

	if !webhookReceived {
		t.Errorf("webhook payload was not received")
	}
	if len(*smtpReceived) != 1 {
		t.Errorf("expected 1 SMTP digest received, got %d", len(*smtpReceived))
	}
	if len(*mqttReceived) < 1 {
		t.Errorf("expected at least 1 MQTT message received, got %d", len(*mqttReceived))
	} else {
		msg := (*mqttReceived)[0]
		if msg.Topic != DefaultMQTTTopic {
			t.Errorf("expected MQTT topic %q, got %q", DefaultMQTTTopic, msg.Topic)
		}
		var payload WebhookDeadlinePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Errorf("failed to unmarshal MQTT payload: %v", err)
		} else if payload.Title != dl.Title {
			t.Errorf("expected MQTT payload title %q, got %q", dl.Title, payload.Title)
		}
	}

	// Verify deadline state transitioned to 'notified'
	notifiedDL, err := dStore.Get(ctx, dl.ID)
	if err != nil {
		t.Fatalf("Get deadline failed: %v", err)
	}
	if notifiedDL.Status != store.DeadlineStatusNotified {
		t.Errorf("expected status 'notified', got %q", notifiedDL.Status)
	}

	// Subsequent cycle: deadline is already 'notified' -> 0 dispatched (no duplicate alerts!)
	webhookReceived = false
	*smtpReceived = nil
	count2, err := dispatcher.DispatchCycle(ctx)
	if err != nil {
		t.Fatalf("second DispatchCycle failed: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected 0 dispatched on subsequent cycle, got %d", count2)
	}
	if webhookReceived || len(*smtpReceived) > 0 {
		t.Errorf("duplicate notifications were sent for already notified deadline")
	}

	// Test channel test endpoints
	if err := dispatcher.TestWebhook(ctx, webhookServer.URL); err != nil {
		t.Errorf("TestWebhook failed: %v", err)
	}
	if err := dispatcher.TestMQTT(ctx, mqttHost, mqttPort, "test/topic", "", ""); err != nil {
		t.Errorf("TestMQTT failed: %v", err)
	}
	if err := dispatcher.TestEmail(ctx, "alex@family.net", smtpHost, smtpPort, "", "", "alerts@sovereign.local"); err != nil {
		t.Errorf("TestEmail failed: %v", err)
	}
}

func TestAlertDispatcher_EmailFailureRetainsPending(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "email_fail_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	dStore, err := store.NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	dl := &store.DocumentDeadline{
		DocumentID: "doc-fail",
		Title:      "Water Bill",
		DueDate:    "2026-10-22",
		Status:     store.DeadlineStatusPending,
	}
	if err := dStore.Save(ctx, dl); err != nil {
		t.Fatalf("failed to save deadline: %v", err)
	}

	// Configure email to point to an unreachable SMTP server
	cfg := &AlertConfig{
		AlertEmail: "user@domain.com",
		SMTPHost:   "127.0.0.1",
		SMTPPort:   54323, // Unreachable
	}
	if err := SaveConfig(ctx, db, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	dispatcher := NewAlertDispatcher(db, dStore, nil)
	dispatcher.SetNowFunc(func() time.Time { return now })

	// Cycle should fail email delivery and retain 'pending' status
	count, err := dispatcher.DispatchCycle(ctx)
	if err != nil {
		t.Fatalf("DispatchCycle returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 notified due to email failure, got %d", count)
	}

	dlAfter, _ := dStore.Get(ctx, dl.ID)
	if dlAfter.Status != store.DeadlineStatusPending {
		t.Errorf("expected status 'pending' retained on email failure, got %q", dlAfter.Status)
	}
}

func TestAlertDispatcher_SendRollbackAlert(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "rollback_notify_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	var webhookReceived bool
	var webhookPayload map[string]any
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookReceived = true
		_ = json.NewDecoder(r.Body).Decode(&webhookPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	mqttHost, mqttPort, cleanupMQTT, mqttReceived := startMockMQTTBroker(t, "", "")
	defer cleanupMQTT()

	smtpHost, smtpPort, cleanupSMTP, smtpReceived := startMockSMTPServer(t, "", "")
	defer cleanupSMTP()

	cfg := &AlertConfig{
		HAWebhookURL:   webhookServer.URL,
		MQTTBrokerHost: mqttHost,
		MQTTBrokerPort: mqttPort,
		AlertEmail:     "admin@family.net",
		SMTPHost:       smtpHost,
		SMTPPort:       smtpPort,
	}
	if err := SaveConfig(ctx, db, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	dispatcher := NewAlertDispatcher(db, nil, nil)
	err = dispatcher.SendRollbackAlert(ctx, "paperless", "Paperless-ngx", "paperless:2.4.0", "paperless:2.3.0", "Container probe failure")
	if err != nil {
		t.Fatalf("SendRollbackAlert failed: %v", err)
	}

	// Verify Webhook
	if !webhookReceived {
		t.Errorf("expected webhook alert to be delivered")
	} else {
		if webhookPayload["event"] != "service_rollback" || webhookPayload["service_id"] != "paperless" {
			t.Errorf("unexpected webhook payload: %+v", webhookPayload)
		}
	}

	// Verify SMTP
	if len(*smtpReceived) != 1 {
		t.Errorf("expected 1 email received, got %d", len(*smtpReceived))
	} else {
		body := (*smtpReceived)[0]
		if !strings.Contains(body, "Paperless-ngx") || !strings.Contains(body, "restored your previous working version") {
			t.Errorf("email body missing service or reassurance text: %s", body)
		}
	}

	// Verify MQTT
	deadline := time.Now().Add(500 * time.Millisecond)
	for len(*mqttReceived) < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if len(*mqttReceived) < 1 {
		t.Errorf("expected at least 1 MQTT alert received")
	}
}

func TestAlertDispatcher_PersistentQueue(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "persistent_queue_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	dStore, err := store.NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	qStore, err := store.NewAlertQueueStore(db)
	if err != nil {
		t.Fatalf("NewAlertQueueStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

	dispatcher := NewAlertDispatcher(db, dStore, nil)
	dispatcher.SetNowFunc(func() time.Time { return now })
	dispatcher.SetAlertQueue(qStore)

	// =========================================================================
	// Fault-Injection 1: Webhook Endpoint Temporary Downtime (HTTP 503) & Recovery
	// =========================================================================
	t.Run("WebhookEndpoint_FailureAndRecovery", func(t *testing.T) {
		var webhookMu sync.Mutex
		webhookStatus := http.StatusServiceUnavailable
		var receivedWebhooks []WebhookDeadlinePayload

		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			webhookMu.Lock()
			defer webhookMu.Unlock()
			if webhookStatus != http.StatusOK {
				http.Error(w, "upstream service degraded", webhookStatus)
				return
			}
			var p WebhookDeadlinePayload
			if err := json.NewDecoder(r.Body).Decode(&p); err == nil {
				receivedWebhooks = append(receivedWebhooks, p)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer webhookServer.Close()

		amt := 85.00
		dl := &store.DocumentDeadline{
			DocumentID: "doc-wh-fail",
			Title:      "Health Insurance Renewal",
			DueDate:    "2026-11-16",
			Amount:     &amt,
			Currency:   "EUR",
			Status:     store.DeadlineStatusPending,
		}
		if err := dStore.Save(ctx, dl); err != nil {
			t.Fatalf("failed to save deadline: %v", err)
		}

		cfg := &AlertConfig{
			HAWebhookURL:           webhookServer.URL,
			DeadlineAlertThreshold: 14,
		}
		if err := SaveConfig(ctx, db, cfg); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		// Initial dispatch: webhook returns 503 -> fails delivery
		count, err := dispatcher.DispatchCycle(ctx)
		if err != nil {
			t.Fatalf("DispatchCycle returned unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 notified due to webhook 503, got %d", count)
		}

		// Verify alert was captured in alert_dispatch_queue as 'pending'
		pendingItems, err := qStore.List(ctx, store.AlertQueueStatusPending)
		if err != nil {
			t.Fatalf("List pending failed: %v", err)
		}
		if len(pendingItems) != 1 {
			t.Fatalf("expected 1 pending item in alert_dispatch_queue, got %d", len(pendingItems))
		}
		item := pendingItems[0]
		if item.Channel != "webhook" {
			t.Errorf("expected channel 'webhook', got %q", item.Channel)
		}
		if item.RetryCount != 0 {
			t.Errorf("expected retry_count 0 on initial enqueue, got %d", item.RetryCount)
		}
		if !strings.Contains(item.LastError, "503") {
			t.Errorf("expected last_error to mention 503, got %q", item.LastError)
		}

		// Recovery: remote webhook target recovers and returns 200 OK
		webhookMu.Lock()
		webhookStatus = http.StatusOK
		webhookMu.Unlock()

		// Re-dispatch: DispatchCycle should retry pending queue items and succeed
		deliveredCount, err := dispatcher.DispatchCycle(ctx)
		if err != nil {
			t.Fatalf("DispatchCycle retry failed: %v", err)
		}
		if deliveredCount != 1 {
			t.Fatalf("expected 1 delivered from queue via DispatchCycle, got %d", deliveredCount)
		}

		// Verify target received webhook
		webhookMu.Lock()
		if len(receivedWebhooks) != 1 {
			t.Errorf("expected 1 webhook received after recovery, got %d", len(receivedWebhooks))
		} else if receivedWebhooks[0].Title != dl.Title {
			t.Errorf("expected webhook title %q, got %q", dl.Title, receivedWebhooks[0].Title)
		}
		webhookMu.Unlock()

		// Verify queue status transitioned to 'delivered'
		deliveredItem, err := qStore.Get(ctx, item.ID)
		if err != nil {
			t.Fatalf("Get item failed: %v", err)
		}
		if deliveredItem.Status != store.AlertQueueStatusDelivered {
			t.Errorf("expected queue status 'delivered', got %q", deliveredItem.Status)
		}

		// Verify document deadline transitioned to 'notified'
		updatedDL, _ := dStore.Get(ctx, dl.ID)
		if updatedDL.Status != store.DeadlineStatusNotified {
			t.Errorf("expected deadline status 'notified', got %q", updatedDL.Status)
		}
	})

	// =========================================================================
	// Fault-Injection 2: SMTP Connection Refused & Recovery
	// =========================================================================
	t.Run("SMTPEndpoint_FailureAndRecovery", func(t *testing.T) {
		amt := 199.99
		dlEmail := &store.DocumentDeadline{
			DocumentID: "doc-email-fail",
			Title:      "Quarterly Property Tax",
			DueDate:    "2026-11-20",
			Amount:     &amt,
			Currency:   "USD",
			Status:     store.DeadlineStatusPending,
		}
		if err := dStore.Save(ctx, dlEmail); err != nil {
			t.Fatalf("failed to save deadline: %v", err)
		}

		// Point to closed port to simulate connection failure
		cfg := &AlertConfig{
			AlertEmail:             "homeowner@sovereign.local",
			SMTPHost:               "127.0.0.1",
			SMTPPort:               54329, // Unreachable port
			DeadlineAlertThreshold: 14,
		}
		if err := SaveConfig(ctx, db, cfg); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		// Initial attempt: fails and enqueues to alert_dispatch_queue
		count, err := dispatcher.DispatchCycle(ctx)
		if err != nil {
			t.Fatalf("DispatchCycle returned unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 notified due to unreachable SMTP, got %d", count)
		}

		emailPending, err := qStore.List(ctx, store.AlertQueueStatusPending)
		if err != nil {
			t.Fatalf("List pending failed: %v", err)
		}
		var emailItem *store.AlertQueueItem
		for _, it := range emailPending {
			if it.Channel == "email" {
				emailItem = it
				break
			}
		}
		if emailItem == nil {
			t.Fatalf("expected pending email alert in queue, found none")
		}

		// Recovery: spin up working mock SMTP server and update configuration
		smtpHost, smtpPort, cleanupSMTP, smtpReceived := startMockSMTPServer(t, "", "")
		defer cleanupSMTP()

		cfg.SMTPHost = smtpHost
		cfg.SMTPPort = smtpPort
		if err := SaveConfig(ctx, db, cfg); err != nil {
			t.Fatalf("SaveConfig with recovered SMTP failed: %v", err)
		}

		// Process queue retry
		deliveredCount, err := dispatcher.ProcessQueue(ctx)
		if err != nil {
			t.Fatalf("ProcessQueue failed: %v", err)
		}
		if deliveredCount < 1 {
			t.Fatalf("expected at least 1 delivered item, got %d", deliveredCount)
		}

		// Verify SMTP server received the email digest
		if len(*smtpReceived) != 1 {
			t.Fatalf("expected 1 email received by mock SMTP server, got %d", len(*smtpReceived))
		}
		if !strings.Contains((*smtpReceived)[0], "Quarterly Property Tax") {
			t.Errorf("expected email to contain deadline title, got: %s", (*smtpReceived)[0])
		}

		// Verify queue item is 'delivered'
		itemAfter, err := qStore.Get(ctx, emailItem.ID)
		if err != nil {
			t.Fatalf("Get email item failed: %v", err)
		}
		if itemAfter.Status != store.AlertQueueStatusDelivered {
			t.Errorf("expected email queue item status 'delivered', got %q", itemAfter.Status)
		}
	})

	// =========================================================================
	// Fault-Injection 3: Max Retries (5) Exhaustion Transitions to 'failed'
	// =========================================================================
	t.Run("MaxRetriesExhaustion_TransitionsToFailed", func(t *testing.T) {
		failItem := &store.AlertQueueItem{
			ID:            "alert-fail-exhaustion",
			Channel:       "webhook",
			Recipient:     "http://127.0.0.1:54399/unreachable",
			Payload:       `{"event":"test"}`,
			Status:        store.AlertQueueStatusPending,
			RetryCount:    0,
			MaxRetries:    5,
			NextAttemptAt: now,
		}
		if err := qStore.Enqueue(ctx, failItem); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}

		// Attempt retries 1 through 5 with advancing simulated time
		currentTime := now
		for attempt := 1; attempt <= 5; attempt++ {
			dispatcher.SetNowFunc(func() time.Time { return currentTime })
			delivered, _ := dispatcher.ProcessQueue(ctx)
			if delivered != 0 {
				t.Fatalf("attempt %d: expected 0 delivered to unreachable endpoint, got %d", attempt, delivered)
			}

			// Advance time past next_attempt_at for subsequent attempt
			currentTime = currentTime.Add(time.Hour)
		}

		// Verify item transitioned to 'failed' after 5 attempts
		exhaustedItem, err := qStore.Get(ctx, failItem.ID)
		if err != nil {
			t.Fatalf("Get exhausted item failed: %v", err)
		}
		if exhaustedItem.Status != store.AlertQueueStatusFailed {
			t.Errorf("expected status 'failed' after 5 attempts, got %q", exhaustedItem.Status)
		}
		if exhaustedItem.RetryCount != 5 {
			t.Errorf("expected retry_count 5, got %d", exhaustedItem.RetryCount)
		}

		// Ensure it is not picked up for further retries
		dispatcher.SetNowFunc(func() time.Time { return currentTime.Add(24 * time.Hour) })
		delivered, err := dispatcher.ProcessQueue(ctx)
		if err != nil {
			t.Fatalf("ProcessQueue after exhaustion returned error: %v", err)
		}
		if delivered != 0 {
			t.Errorf("expected 0 delivered after exhaustion, got %d", delivered)
		}
	})
}

