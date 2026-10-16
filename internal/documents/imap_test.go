package documents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sovereign/internal/store"
)

// mockIMAPServer simulates an RFC 3501 IMAP server over plain TCP.
type mockIMAPServer struct {
	listener    net.Listener
	addr        string
	authFail    bool
	unseenCount int
	pdfPayload  []byte
	mu          sync.Mutex
	stopChan    chan struct{}
}

func startMockIMAPServer(t *testing.T, authFail bool, unseenCount int) *mockIMAPServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock imap listener: %v", err)
	}

	pdfData := MakeSamplePDF("Test Invoice Content #1234")

	srv := &mockIMAPServer{
		listener:    ln,
		addr:        ln.Addr().String(),
		authFail:    authFail,
		unseenCount: unseenCount,
		pdfPayload:  pdfData,
		stopChan:    make(chan struct{}),
	}

	go srv.serve()
	return srv
}

func (s *mockIMAPServer) close() {
	close(s.stopChan)
	_ = s.listener.Close()
}

func (s *mockIMAPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *mockIMAPServer) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	// 1. Send initial greeting
	_, _ = io.WriteString(conn, "* OK [CAPABILITY IMAP4rev1] Sovereign Mock IMAP Ready\r\n")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		tag := parts[0]
		cmd := strings.ToUpper(parts[1])

		switch cmd {
		case "LOGIN":
			if s.authFail {
				_, _ = io.WriteString(conn, fmt.Sprintf("%s NO [AUTHENTICATIONFAILED] Invalid credentials\r\n", tag))
			} else {
				_, _ = io.WriteString(conn, fmt.Sprintf("%s OK LOGIN completed\r\n", tag))
			}
		case "SELECT":
			_, _ = io.WriteString(conn, "* 1 EXISTS\r\n* 1 RECENT\r\n")
			_, _ = io.WriteString(conn, fmt.Sprintf("%s OK [READ-WRITE] SELECT completed\r\n", tag))
		case "SEARCH":
			if s.unseenCount > 0 {
				_, _ = io.WriteString(conn, "* SEARCH 1\r\n")
			} else {
				_, _ = io.WriteString(conn, "* SEARCH\r\n")
			}
			_, _ = io.WriteString(conn, fmt.Sprintf("%s OK SEARCH completed\r\n", tag))
		case "FETCH":
			rawEmail := s.buildEmailMessage()
			_, _ = io.WriteString(conn, fmt.Sprintf("* 1 FETCH (BODY[] {%d}\r\n", len(rawEmail)))
			_, _ = conn.Write(rawEmail)
			_, _ = io.WriteString(conn, ")\r\n")
			_, _ = io.WriteString(conn, fmt.Sprintf("%s OK FETCH completed\r\n", tag))
		case "STORE":
			_, _ = io.WriteString(conn, "* 1 FETCH (FLAGS (\\Seen))\r\n")
			_, _ = io.WriteString(conn, fmt.Sprintf("%s OK STORE completed\r\n", tag))
		case "LOGOUT":
			_, _ = io.WriteString(conn, "* BYE Mock server logging out\r\n")
			_, _ = io.WriteString(conn, fmt.Sprintf("%s OK LOGOUT completed\r\n", tag))
			return
		default:
			_, _ = io.WriteString(conn, fmt.Sprintf("%s OK %s completed\r\n", tag, cmd))
		}
	}
}

func (s *mockIMAPServer) buildEmailMessage() []byte {
	boundary := "----=_Part_Boundary_12345"
	b64PDF := base64.StdEncoding.EncodeToString(s.pdfPayload)

	var buf bytes.Buffer
	buf.WriteString("From: billing@utility.com\r\n")
	buf.WriteString("To: alex@family.net\r\n")
	buf.WriteString("Subject: Monthly Electricity Invoice\r\n")
	buf.WriteString("Date: Fri, 16 Oct 2026 08:00:00 +0000\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary))

	// Plain text body
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString("Please find your monthly utility bill attached.\r\n\r\n")

	// PDF Attachment
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: application/pdf; name=\"invoice_october.pdf\"\r\n")
	buf.WriteString("Content-Disposition: attachment; filename=\"invoice_october.pdf\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(b64PDF + "\r\n")

	// Ignored non-document attachment
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: application/octet-stream; name=\"script.sh\"\r\n")
	buf.WriteString("Content-Disposition: attachment; filename=\"script.sh\"\r\n\r\n")
	buf.WriteString("#!/bin/sh\necho hello\r\n")

	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return buf.Bytes()
}

func TestIMAPClient_BasicFlow(t *testing.T) {
	srv := startMockIMAPServer(t, false, 1)
	defer srv.close()

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	if err := client.Connect(ctx, srv.addr, false, nil); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Login
	if err := client.Login(ctx, "alex@family.net", "correct-password"); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Select
	count, err := client.Select(ctx, "INBOX")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 exists message, got %d", count)
	}

	// SearchUnseen
	unseen, err := client.SearchUnseen(ctx)
	if err != nil {
		t.Fatalf("SearchUnseen failed: %v", err)
	}
	if len(unseen) != 1 || unseen[0] != 1 {
		t.Fatalf("expected unseen seq [1], got %v", unseen)
	}

	// FetchMessage
	msgBytes, err := client.FetchMessage(ctx, 1)
	if err != nil {
		t.Fatalf("FetchMessage failed: %v", err)
	}
	if len(msgBytes) == 0 {
		t.Fatalf("expected non-empty message content")
	}

	// ExtractAttachments
	docs, err := ExtractAttachments(msgBytes)
	if err != nil {
		t.Fatalf("ExtractAttachments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document extracted, got %d", len(docs))
	}
	if docs[0].Filename != "invoice_october.pdf" {
		t.Errorf("expected filename 'invoice_october.pdf', got %q", docs[0].Filename)
	}
	if docs[0].ContentType != "application/pdf" {
		t.Errorf("expected contentType 'application/pdf', got %q", docs[0].ContentType)
	}
	if !bytes.HasPrefix(docs[0].Data, []byte("%PDF")) {
		t.Errorf("expected PDF magic bytes, got %q", string(docs[0].Data[:10]))
	}

	// MarkSeen
	if err := client.MarkSeen(ctx, 1); err != nil {
		t.Fatalf("MarkSeen failed: %v", err)
	}

	// Close
	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestIMAPClient_AuthFailure(t *testing.T) {
	srv := startMockIMAPServer(t, true, 0)
	defer srv.close()

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx, srv.addr, false, nil); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	err := client.Login(ctx, "alex@family.net", "wrong-password")
	if err == nil {
		t.Fatalf("expected error on failed login, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected authentication failed in error message, got: %v", err)
	}
}

func TestPoller_HappyPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sovereign-poller-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	imapStore, err := store.NewIMAPStore(db, []byte("32-byte-secret-key-for-test-here"))
	if err != nil {
		t.Fatalf("NewIMAPStore failed: %v", err)
	}

	srv := startMockIMAPServer(t, false, 1)
	defer srv.close()

	host, portStr, _ := net.SplitHostPort(srv.addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	ctx := context.Background()
	cfg := &store.IMAPConfig{
		Host:       host,
		Port:       port,
		Username:   "alex@family.net",
		Password:   "correct-password",
		Folder:     "INBOX",
		SSLEnabled: false,
		Enabled:    true,
	}
	if err := imapStore.Save(ctx, cfg); err != nil {
		t.Fatalf("imapStore.Save failed: %v", err)
	}

	consumeDir := filepath.Join(tempDir, "consume")

	var eventsMu sync.Mutex
	var events []string
	broadcaster := func(event string, data any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	var healthStatus, healthLevel, healthDetail string
	healthSetter := func(serviceID, status, health, detail string) {
		healthStatus = status
		healthLevel = health
		healthDetail = detail
	}

	poller, err := NewPoller(PollerConfig{
		Store:               imapStore,
		ConsumeDir:          consumeDir,
		Broadcaster:         broadcaster,
		ServiceHealthSetter: healthSetter,
		ClientFactory: func() IMAPClient {
			return NewClient()
		},
	})
	if err != nil {
		t.Fatalf("NewPoller failed: %v", err)
	}

	// Execute poll
	if err := poller.PollOnce(ctx); err != nil {
		t.Fatalf("poller.PollOnce failed: %v", err)
	}

	// Verify attachment was written into consumeDir
	entries, err := os.ReadDir(consumeDir)
	if err != nil {
		t.Fatalf("failed to read consume dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in consume dir, found %d", len(entries))
	}
	targetFile := entries[0].Name()
	if !strings.HasSuffix(targetFile, "_invoice_october.pdf") {
		t.Errorf("expected file ending with _invoice_october.pdf, got %s", targetFile)
	}

	// Verify file permissions (0644)
	info, err := os.Stat(filepath.Join(consumeDir, targetFile))
	if err != nil {
		t.Fatalf("failed to stat ingested file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0644 {
		t.Errorf("expected file permissions 0644, got %o", perm)
	}

	// Verify health updated to healthy
	if healthStatus != "running" {
		t.Errorf("expected status 'running', got %q", healthStatus)
	}
	if healthLevel != "healthy" {
		t.Errorf("expected health 'healthy', got %q", healthLevel)
	}
	if !strings.Contains(healthDetail, "Connected") {
		t.Errorf("expected healthDetail containing 'Connected', got %q", healthDetail)
	}

	// Verify events emitted
	eventsMu.Lock()
	defer eventsMu.Unlock()
	hasDocEvent := false
	hasMailboxEvent := false
	for _, e := range events {
		if e == EventDocumentIngested {
			hasDocEvent = true
		}
		if e == EventMailboxStatus {
			hasMailboxEvent = true
		}
	}
	if !hasDocEvent {
		t.Errorf("expected EventDocumentIngested event to be emitted")
	}
	if !hasMailboxEvent {
		t.Errorf("expected EventMailboxStatus event to be emitted")
	}

	// Verify last_polled_at updated in store
	updatedCfg, err := imapStore.Get(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("failed to get updated imap config: %v", err)
	}
	if updatedCfg.LastPolledAt == nil {
		t.Errorf("expected non-nil LastPolledAt after successful poll")
	}
	if updatedCfg.LastError != "" {
		t.Errorf("expected empty LastError, got %q", updatedCfg.LastError)
	}
}

func TestPoller_AuthFailure_DegradesToAmber(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sovereign-poller-fail-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	imapStore, err := store.NewIMAPStore(db, []byte("32-byte-secret-key-for-test-here"))
	if err != nil {
		t.Fatalf("NewIMAPStore failed: %v", err)
	}

	srv := startMockIMAPServer(t, true, 0)
	defer srv.close()

	host, portStr, _ := net.SplitHostPort(srv.addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	ctx := context.Background()
	cfg := &store.IMAPConfig{
		Host:       host,
		Port:       port,
		Username:   "alex@family.net",
		Password:   "wrong-password",
		Folder:     "INBOX",
		SSLEnabled: false,
		Enabled:    true,
	}
	if err := imapStore.Save(ctx, cfg); err != nil {
		t.Fatalf("imapStore.Save failed: %v", err)
	}

	consumeDir := filepath.Join(tempDir, "consume")

	var lastMailboxEvent map[string]any
	broadcaster := func(event string, data any) {
		if event == EventMailboxStatus {
			if m, ok := data.(map[string]any); ok {
				lastMailboxEvent = m
			}
		}
	}

	var healthStatus, healthLevel, healthDetail string
	healthSetter := func(serviceID, status, health, detail string) {
		healthStatus = status
		healthLevel = health
		healthDetail = detail
	}

	poller, err := NewPoller(PollerConfig{
		Store:               imapStore,
		ConsumeDir:          consumeDir,
		Broadcaster:         broadcaster,
		ServiceHealthSetter: healthSetter,
		ClientFactory: func() IMAPClient {
			return NewClient()
		},
	})
	if err != nil {
		t.Fatalf("NewPoller failed: %v", err)
	}

	// Poll should fail and degrade gracefully
	pollErr := poller.PollOnce(ctx)
	if pollErr == nil {
		t.Fatalf("expected error on auth failure, got nil")
	}

	// ServiceCard must transition to Amber (running, degraded)
	if healthStatus != "running" {
		t.Errorf("expected status 'running', got %q", healthStatus)
	}
	if healthLevel != "degraded" {
		t.Errorf("expected health 'degraded' (Amber), got %q", healthLevel)
	}
	if healthDetail != ReassuranceMailboxDisconnected {
		t.Errorf("expected reassurance detail %q, got %q", ReassuranceMailboxDisconnected, healthDetail)
	}

	// Event must show connected=false with consumer reassurance message
	if lastMailboxEvent == nil {
		t.Fatalf("expected dms.mailbox_status event")
	}
	if lastMailboxEvent["connected"] != false {
		t.Errorf("expected connected=false, got %v", lastMailboxEvent["connected"])
	}
	if lastMailboxEvent["error"] != ReassuranceMailboxDisconnected {
		t.Errorf("expected consumer error message, got %v", lastMailboxEvent["error"])
	}

	// Verify store updated with last_error
	updatedCfg, err := imapStore.Get(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("failed to query config: %v", err)
	}
	if !strings.Contains(updatedCfg.LastError, "authentication failed") {
		t.Errorf("expected LastError to record auth failure, got: %s", updatedCfg.LastError)
	}
}

func TestIngestEngine_DirectUpload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sovereign-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	consumeDir := filepath.Join(tempDir, "consume")
	var eventFired bool
	engine, err := NewIngestEngine(consumeDir, func(event string, data any) {
		if event == EventDocumentIngested {
			eventFired = true
		}
	})
	if err != nil {
		t.Fatalf("NewIngestEngine failed: %v", err)
	}

	ctx := context.Background()

	// 1. Valid PDF upload
	pdfData := MakeSamplePDF("Mobile Scanner Uploaded Document")
	doc, err := engine.SaveDocument(ctx, "scan_receipt.pdf", bytes.NewReader(pdfData), "mobile_scanner")
	if err != nil {
		t.Fatalf("SaveDocument failed on valid PDF: %v", err)
	}
	if doc.Filename != "scan_receipt.pdf" {
		t.Errorf("expected filename 'scan_receipt.pdf', got %s", doc.Filename)
	}
	if doc.ContentType != "application/pdf" {
		t.Errorf("expected ContentType 'application/pdf', got %s", doc.ContentType)
	}
	if !eventFired {
		t.Errorf("expected EventDocumentIngested to be broadcast")
	}

	// Verify file permissions (0644)
	st, err := os.Stat(doc.Path)
	if err != nil {
		t.Fatalf("failed to stat saved file: %v", err)
	}
	if st.Mode().Perm() != 0644 {
		t.Errorf("expected permissions 0644, got %o", st.Mode().Perm())
	}

	// 2. Unsupported format (e.g. .exe)
	_, err = engine.SaveDocument(ctx, "malicious.exe", bytes.NewReader([]byte("not a pdf")), "upload")
	if err == nil {
		t.Fatalf("expected error on unsupported extension, got nil")
	}

	// 3. Extension matches .pdf but magic bytes invalid
	_, err = engine.SaveDocument(ctx, "fake.pdf", bytes.NewReader([]byte("not actually a pdf file")), "upload")
	if err == nil {
		t.Fatalf("expected error on invalid magic bytes, got nil")
	}

	// 4. Empty payload
	_, err = engine.SaveDocument(ctx, "empty.pdf", bytes.NewReader([]byte{}), "upload")
	if err == nil {
		t.Fatalf("expected error on empty payload, got nil")
	}

	// 5. Verify OpenAPI fields are set
	if doc.Format != "application/pdf" {
		t.Errorf("expected Format 'application/pdf', got %q", doc.Format)
	}
	if doc.SizeBytes != int64(len(pdfData)) {
		t.Errorf("expected SizeBytes %d, got %d", len(pdfData), doc.SizeBytes)
	}
	if doc.ConsumePath != doc.Path {
		t.Errorf("expected ConsumePath %q, got %q", doc.Path, doc.ConsumePath)
	}
	if doc.IngestedAt == "" {
		t.Errorf("expected IngestedAt timestamp to be populated")
	}
}

func TestExtractAttachments_RFC2047_EncodedFilename(t *testing.T) {
	pdfData := MakeSamplePDF("RFC 2047 invoice test")
	b64Data := base64.StdEncoding.EncodeToString(pdfData)

	boundary := "----=_Part_123_456"
	msg := fmt.Sprintf("MIME-Version: 1.0\r\n"+
		"Content-Type: multipart/mixed; boundary=\"%s\"\r\n"+
		"Subject: Rechnung\r\n\r\n"+
		"--%s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n"+
		"Please find attached your bill.\r\n"+
		"--%s\r\n"+
		"Content-Type: application/pdf\r\n"+
		"Content-Disposition: attachment; filename=\"=?UTF-8?Q?Rechnung_=C3=A4_2026.pdf?=\"\r\n"+
		"Content-Transfer-Encoding: base64\r\n\r\n"+
		"%s\r\n"+
		"--%s--\r\n", boundary, boundary, boundary, b64Data, boundary)

	attachments, err := ExtractAttachments([]byte(msg))
	if err != nil {
		t.Fatalf("ExtractAttachments failed: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	expectedName := "Rechnung ä 2026.pdf"
	if attachments[0].Filename != expectedName {
		t.Errorf("expected decoded filename %q, got %q", expectedName, attachments[0].Filename)
	}
}

func TestExtractAttachments_SinglePartImage(t *testing.T) {
	// Sample PNG magic bytes: 0x89 PNG \r \n 0x1a \n
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	b64Data := base64.StdEncoding.EncodeToString(pngData)

	msg := fmt.Sprintf("MIME-Version: 1.0\r\n"+
		"Content-Type: image/png\r\n"+
		"Content-Transfer-Encoding: base64\r\n\r\n"+
		"%s\r\n", b64Data)

	attachments, err := ExtractAttachments([]byte(msg))
	if err != nil {
		t.Fatalf("ExtractAttachments failed: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].Filename != "document.png" {
		t.Errorf("expected filename 'document.png', got %q", attachments[0].Filename)
	}
	if attachments[0].ContentType != "image/png" {
		t.Errorf("expected ContentType 'image/png', got %q", attachments[0].ContentType)
	}
}
