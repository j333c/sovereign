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

// startMockMQTTBroker spawns a lightweight mock MQTT 3.1.1 broker for testing CONNECT, PUBLISH, PUBACK, DISCONNECT.
func startMockMQTTBroker(t *testing.T, expectedUser, expectedPass string) (string, int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock MQTT listener: %v", err)
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

			go handleMockMQTTConn(conn, expectedUser, expectedPass)
		}
	}()

	cleanup := func() {
		close(stopCh)
		_ = listener.Close()
	}

	return tcpAddr.IP.String(), tcpAddr.Port, cleanup
}

func handleMockMQTTConn(conn net.Conn, expectedUser, expectedPass string) {
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

	// If QoS 1, find packet ID and send PUBACK
	if qos == 1 && remLen >= 4 {
		topicLen := binary.BigEndian.Uint16(publishBody[0:2])
		pidOffset := 2 + int(topicLen)
		if pidOffset+2 <= len(publishBody) {
			pid := publishBody[pidOffset : pidOffset+2]
			// Send PUBACK: 0x40, 0x02, pid[0], pid[1]
			puback := []byte{packetPUBACK, 0x02, pid[0], pid[1]}
			_, _ = conn.Write(puback)
		}
	}

	// 3. Optional DISCONNECT
	_, _ = io.ReadFull(conn, header)
}

func TestMQTTClient_PublishQoS1AndErrors(t *testing.T) {
	host, port, cleanup := startMockMQTTBroker(t, "", "")
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

	mqttHost, mqttPort, cleanupMQTT := startMockMQTTBroker(t, "", "")
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
