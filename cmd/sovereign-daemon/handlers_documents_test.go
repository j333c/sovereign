package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sovereign/internal/documents"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServer_DocumentUpload(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	var eventsMu sync.Mutex
	var events []string
	eventCh := make(chan sseEvent, 10)
	srv.eventsMu.Lock()
	srv.eventSubscribers[eventCh] = struct{}{}
	srv.eventsMu.Unlock()
	defer func() {
		srv.eventsMu.Lock()
		delete(srv.eventSubscribers, eventCh)
		srv.eventsMu.Unlock()
	}()

	go func() {
		for evt := range eventCh {
			eventsMu.Lock()
			events = append(events, evt.Event)
			eventsMu.Unlock()
		}
	}()

	// 1. Valid PDF upload -> 201 Created
	pdfBytes := documents.MakeSamplePDF("Test Document Scan Content")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "insurance_policy.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write(pdfBytes); err != nil {
		t.Fatalf("failed writing to form file: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/documents/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var uploadResp documents.IngestedDocument
	if err := json.Unmarshal(w.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	if uploadResp.Filename != "insurance_policy.pdf" {
		t.Errorf("expected filename 'insurance_policy.pdf', got %s", uploadResp.Filename)
	}
	if uploadResp.ContentType != "application/pdf" {
		t.Errorf("expected content_type 'application/pdf', got %s", uploadResp.ContentType)
	}

	// Verify file was written to /data/documents/consume/<uuid>_<name> with 0644 permissions
	consumeDir := filepath.Join(dataDir, "documents", "consume")
	entries, err := os.ReadDir(consumeDir)
	if err != nil {
		t.Fatalf("failed to read consume dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in consume dir, found %d", len(entries))
	}
	targetPath := filepath.Join(consumeDir, entries[0].Name())
	fi, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat ingested document: %v", err)
	}
	if fi.Mode().Perm() != 0644 {
		t.Errorf("expected file permissions 0644, got %o", fi.Mode().Perm())
	}

	// Verify event was emitted
	time.Sleep(50 * time.Millisecond)
	eventsMu.Lock()
	foundEvent := false
	for _, e := range events {
		if e == "dms.document_ingested" {
			foundEvent = true
			break
		}
	}
	eventsMu.Unlock()
	if !foundEvent {
		t.Errorf("expected dms.document_ingested event to be emitted")
	}

	// 2. Unsupported format (e.g. .exe) -> 400 Bad Request
	body.Reset()
	writer = multipart.NewWriter(&body)
	part, _ = writer.CreateFormFile("file", "malware.exe")
	_, _ = part.Write([]byte("not a document"))
	_ = writer.Close()

	req = httptest.NewRequest("POST", "/api/v1/documents/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on unsupported format, got %d", w.Code)
	}

	// 3. Empty payload -> 400 Bad Request
	body.Reset()
	writer = multipart.NewWriter(&body)
	part, _ = writer.CreateFormFile("file", "empty.pdf")
	_ = writer.Close()

	req = httptest.NewRequest("POST", "/api/v1/documents/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty payload, got %d", w.Code)
	}
}

func TestServer_DropzoneCategoryDocuments(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	var eventsMu sync.Mutex
	var events []string
	eventCh := make(chan sseEvent, 10)
	srv.eventsMu.Lock()
	srv.eventSubscribers[eventCh] = struct{}{}
	srv.eventsMu.Unlock()
	defer func() {
		srv.eventsMu.Lock()
		delete(srv.eventSubscribers, eventCh)
		srv.eventsMu.Unlock()
	}()

	go func() {
		for evt := range eventCh {
			eventsMu.Lock()
			events = append(events, evt.Event)
			eventsMu.Unlock()
		}
	}()

	// 1. Dropzone upload with category "documents" and valid PDF -> 200 OK & routes to /data/documents/consume
	pdfBytes := documents.MakeSamplePDF("Utility Bill #98765")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("category", "documents")
	part, _ := writer.CreateFormFile("file", "electric_bill.pdf")
	_, _ = part.Write(pdfBytes)
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/drop/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on dropzone upload, got %d: %s", w.Code, w.Body.String())
	}

	consumeDir := filepath.Join(dataDir, "documents", "consume")
	entries, err := os.ReadDir(consumeDir)
	if err != nil {
		t.Fatalf("failed reading consume directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in consume dir, found %d", len(entries))
	}
	if entries[0].Name() != "electric_bill.pdf" {
		t.Errorf("expected filename 'electric_bill.pdf', got %s", entries[0].Name())
	}

	// Verify permissions (0644)
	fi, _ := os.Stat(filepath.Join(consumeDir, entries[0].Name()))
	if fi.Mode().Perm() != 0644 {
		t.Errorf("expected 0644 permissions, got %o", fi.Mode().Perm())
	}

	// Verify dms.document_ingested event emitted
	time.Sleep(50 * time.Millisecond)
	eventsMu.Lock()
	foundDocEvent := false
	for _, e := range events {
		if e == "dms.document_ingested" {
			foundDocEvent = true
			break
		}
	}
	eventsMu.Unlock()
	if !foundDocEvent {
		t.Errorf("expected dms.document_ingested event from dropzone documents upload")
	}

	// 2. Dropzone upload with category "documents" and incompatible format -> 400 Bad Request
	body.Reset()
	writer = multipart.NewWriter(&body)
	_ = writer.WriteField("category", "documents")
	part, _ = writer.CreateFormFile("file", "song.mp3")
	_, _ = part.Write([]byte("ID3v2 not a document"))
	_ = writer.Close()

	req = httptest.NewRequest("POST", "/api/v1/drop/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for incompatible document format in dropzone, got %d", w.Code)
	}
}

func TestServer_IMAPConfig_CRUD(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Initial GET -> empty response with connected: false
	req := httptest.NewRequest("GET", "/api/v1/documents/imap", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/documents/imap, got %d: %s", w.Code, w.Body.String())
	}
	var getResp IMAPConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed decoding GET response: %v", err)
	}
	if getResp.Connected {
		t.Errorf("expected connected=false on unconfigured mailbox")
	}

	// 2. POST save configuration
	savePayload := map[string]any{
		"host":        "imap.mailserver.com",
		"port":        993,
		"username":    "sarah@family.net",
		"password":    "SuperSecretPassword123!",
		"folder":      "INBOX",
		"ssl_enabled": true,
		"enabled":     true,
	}
	payloadBytes, _ := json.Marshal(savePayload)
	req = httptest.NewRequest("POST", "/api/v1/documents/imap", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from POST /api/v1/documents/imap, got %d: %s", w.Code, w.Body.String())
	}

	var saveResp IMAPConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("failed decoding save response: %v", err)
	}
	if saveResp.Password != "********" {
		t.Errorf("SECURITY: expected password to be masked as '********', got %s", saveResp.Password)
	}
	if saveResp.Host != "imap.mailserver.com" || saveResp.Username != "sarah@family.net" {
		t.Errorf("unexpected fields in save response: %+v", saveResp)
	}

	// Verify password is encrypted at rest in SQLite
	dbPath := filepath.Join(dataDir, "state", "sovereign.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite: %v", err)
	}
	defer db.Close()

	var encryptedBlob []byte
	err = db.QueryRow("SELECT password_encrypted FROM imap_configs WHERE id = ?", saveResp.ID).Scan(&encryptedBlob)
	if err != nil {
		t.Fatalf("failed querying raw password_encrypted: %v", err)
	}
	if bytes.Contains(encryptedBlob, []byte("SuperSecretPassword123!")) {
		t.Fatalf("SECURITY VIOLATION: Plaintext password stored in SQLite database!")
	}

	// 3. GET configured mailbox
	req = httptest.NewRequest("GET", "/api/v1/documents/imap", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET, got %d: %s", w.Code, w.Body.String())
	}
	var getResp2 IMAPConfigResponse
	_ = json.Unmarshal(w.Body.Bytes(), &getResp2)
	if getResp2.Password != "********" {
		t.Errorf("expected masked password '********', got %s", getResp2.Password)
	}
	if getResp2.Host != "imap.mailserver.com" {
		t.Errorf("expected host 'imap.mailserver.com', got %s", getResp2.Host)
	}
}

func startTestIMAPMock(t *testing.T, shouldFail bool) (string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test imap mock: %v", err)
	}

	stopCh := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				_, _ = io.WriteString(c, "* OK [CAPABILITY IMAP4rev1] Mock IMAP Server\r\n")

				for {
					line, err := r.ReadString('\n')
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
						if shouldFail {
							_, _ = io.WriteString(c, fmt.Sprintf("%s NO [AUTHENTICATIONFAILED] Authentication failed\r\n", tag))
						} else {
							_, _ = io.WriteString(c, fmt.Sprintf("%s OK LOGIN completed\r\n", tag))
						}
					case "SELECT":
						_, _ = io.WriteString(c, "* 1 EXISTS\r\n")
						_, _ = io.WriteString(c, fmt.Sprintf("%s OK [READ-WRITE] SELECT completed\r\n", tag))
					case "LOGOUT":
						_, _ = io.WriteString(c, fmt.Sprintf("%s OK LOGOUT completed\r\n", tag))
						return
					default:
						_, _ = io.WriteString(c, fmt.Sprintf("%s OK %s completed\r\n", tag, cmd))
					}
				}
			}(conn)
		}
	}()

	return ln.Addr().String(), func() {
		close(stopCh)
		_ = ln.Close()
	}
}

func TestServer_IMAPTestConnection_SuccessAndDegradation(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()

	// Activate paperless service so it's in running state
	_, err := srv.serviceMgr.ActivateService(ctx, supervisor.ServicePaperless)
	if err != nil {
		t.Fatalf("failed to activate paperless: %v", err)
	}

	st, err := srv.serviceMgr.GetService(ctx, supervisor.ServicePaperless)
	if err != nil || st.Status != supervisor.StatusRunning {
		t.Fatalf("expected paperless running, got %+v (err: %v)", st, err)
	}

	// 1. Successful connection test
	addrSuccess, closeSuccess := startTestIMAPMock(t, false)
	defer closeSuccess()

	host, portStr, _ := net.SplitHostPort(addrSuccess)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	testPayload := map[string]any{
		"host":        host,
		"port":        port,
		"username":    "alex@family.net",
		"password":    "valid_secret",
		"folder":      "INBOX",
		"ssl_enabled": false,
	}
	bodyBytes, _ := json.Marshal(testPayload)
	req := httptest.NewRequest("POST", "/api/v1/documents/imap/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on successful test, got %d: %s", w.Code, w.Body.String())
	}
	var testResp IMAPTestResponse
	_ = json.Unmarshal(w.Body.Bytes(), &testResp)
	if !testResp.Success {
		t.Errorf("expected success=true")
	}

	// 2. Failed connection test (auth failure)
	addrFail, closeFail := startTestIMAPMock(t, true)
	defer closeFail()

	hostFail, portFailStr, _ := net.SplitHostPort(addrFail)
	var portFail int
	fmt.Sscanf(portFailStr, "%d", &portFail)

	failPayload := map[string]any{
		"host":        hostFail,
		"port":        portFail,
		"username":    "alex@family.net",
		"password":    "wrong_secret",
		"folder":      "INBOX",
		"ssl_enabled": false,
	}
	bodyBytes, _ = json.Marshal(failPayload)
	req = httptest.NewRequest("POST", "/api/v1/documents/imap/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway on auth failure, got %d: %s", w.Code, w.Body.String())
	}

	var prob ProblemDetails
	_ = json.Unmarshal(w.Body.Bytes(), &prob)
	if prob.Detail != documents.ReassuranceMailboxDisconnected {
		t.Errorf("expected reassurance microcopy %q, got %q", documents.ReassuranceMailboxDisconnected, prob.Detail)
	}

	// Verify Paperless ServiceCard health degraded to Amber without stopping container
	stAfter, err := srv.serviceMgr.GetService(ctx, supervisor.ServicePaperless)
	if err != nil {
		t.Fatalf("failed to query paperless service: %v", err)
	}
	if stAfter.Status != supervisor.StatusRunning {
		t.Errorf("CRITICAL INVARIANT: Paperless status must remain 'running', got %q", stAfter.Status)
	}
	if stAfter.Health != supervisor.HealthDegraded {
		t.Errorf("expected Paperless health 'degraded' (Amber), got %q", stAfter.Health)
	}
}

func TestServer_Deadlines_QueryAndDismiss(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()
	dStore := srv.DeadlineStore()
	if dStore == nil {
		t.Fatalf("expected initialized deadline store")
	}

	now := time.Now().UTC()
	amt1 := 150.00
	d1 := &store.DocumentDeadline{
		DocumentID: "doc-1",
		Title:      "Electricity Bill",
		DueDate:    now.AddDate(0, 0, 7).Format("2006-01-02"), // 7 days in future -> pending
		Amount:     &amt1,
		Currency:   "USD",
		Snippet:    "Total Due: $150.00 in 7 days",
		Status:     store.DeadlineStatusPending,
	}
	_ = dStore.Save(ctx, d1)

	d2 := &store.DocumentDeadline{
		DocumentID: "doc-2",
		Title:      "Contract Expiration",
		DueDate:    now.AddDate(0, 0, 45).Format("2006-01-02"), // 45 days in future -> future
		Status:     store.DeadlineStatusFuture,
	}
	_ = dStore.Save(ctx, d2)

	// 1. Query pending deadlines within 14 days
	req := httptest.NewRequest("GET", "/api/v1/documents/deadlines?status=pending&within_days=14", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var results []*store.DocumentDeadline
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 pending deadline within 14 days, got %d", len(results))
	}
	if results[0].ID != d1.ID {
		t.Errorf("expected deadline ID %q, got %q", d1.ID, results[0].ID)
	}

	// 1b. Query unparameterized (no query string): defaults to pending within 14 days
	unparamReq := httptest.NewRequest("GET", "/api/v1/documents/deadlines", nil)
	unparamW := httptest.NewRecorder()
	srv.router.ServeHTTP(unparamW, unparamReq)
	if unparamW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on unparameterized query, got %d: %s", unparamW.Code, unparamW.Body.String())
	}
	var unparamResults []*store.DocumentDeadline
	_ = json.Unmarshal(unparamW.Body.Bytes(), &unparamResults)
	if len(unparamResults) != 1 || unparamResults[0].ID != d1.ID {
		t.Errorf("expected default query to return d1, got %+v", unparamResults)
	}

	// 1c. Query future deadlines without within_days: unconstrained search returns d2
	futureReq := httptest.NewRequest("GET", "/api/v1/documents/deadlines?status=future", nil)
	futureW := httptest.NewRecorder()
	srv.router.ServeHTTP(futureW, futureReq)
	if futureW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on future query, got %d: %s", futureW.Code, futureW.Body.String())
	}
	var futureResults []*store.DocumentDeadline
	_ = json.Unmarshal(futureW.Body.Bytes(), &futureResults)
	if len(futureResults) != 1 || futureResults[0].ID != d2.ID {
		t.Errorf("expected status=future unconstrained query to return d2, got %+v", futureResults)
	}

	// 2. Dismiss d1
	dismissReq := httptest.NewRequest("POST", "/api/v1/documents/deadlines/"+d1.ID+"/dismiss", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, dismissReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on dismissal, got %d: %s", w.Code, w.Body.String())
	}

	var dismissed store.DocumentDeadline
	_ = json.Unmarshal(w.Body.Bytes(), &dismissed)
	if dismissed.Status != store.DeadlineStatusDismissed {
		t.Errorf("expected status 'dismissed', got %q", dismissed.Status)
	}

	// Query pending again -> must be empty
	req = httptest.NewRequest("GET", "/api/v1/documents/deadlines?status=pending&within_days=14", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 0 {
		t.Errorf("expected 0 pending deadlines after dismiss, got %d", len(results))
	}

	// 3. Dismiss non-existent ID -> 404 Not Found
	dismissReq404 := httptest.NewRequest("POST", "/api/v1/documents/deadlines/non-existent-id/dismiss", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, dismissReq404)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w.Code)
	}
	var prob ProblemDetails
	_ = json.Unmarshal(w.Body.Bytes(), &prob)
	if prob.Status != http.StatusNotFound {
		t.Errorf("expected ProblemDetails status 404, got %d", prob.Status)
	}
}

func TestServer_Deadlines_ManualScanEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	scanBody := map[string]any{
		"force":       true,
		"document_id": "manual-doc-1",
		"title":       "Invoice #482",
		"text":        "Invoice #482. Total Due: $150.00 by 2026-10-28",
	}
	bodyBytes, _ := json.Marshal(scanBody)

	req := httptest.NewRequest("POST", "/api/v1/documents/deadlines/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp ScanDeadlinesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode scan response: %v", err)
	}

	if resp.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", resp.Status)
	}
	if resp.DeadlinesFound != 1 {
		t.Fatalf("expected 1 deadline found, got %d", resp.DeadlinesFound)
	}
	if resp.Deadlines[0].DueDate != "2026-10-28" {
		t.Errorf("expected due_date '2026-10-28', got %q", resp.Deadlines[0].DueDate)
	}
	if resp.Deadlines[0].Amount == nil || *resp.Deadlines[0].Amount != 150.00 {
		t.Errorf("expected amount 150.00, got %v", resp.Deadlines[0].Amount)
	}

	// 2. Empty body manual scan: should execute batch pass without error
	emptyReq := httptest.NewRequest("POST", "/api/v1/documents/deadlines/scan", nil)
	emptyW := httptest.NewRecorder()
	srv.router.ServeHTTP(emptyW, emptyReq)
	if emptyW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for empty body scan, got %d: %s", emptyW.Code, emptyW.Body.String())
	}
	var emptyResp ScanDeadlinesResponse
	if err := json.Unmarshal(emptyW.Body.Bytes(), &emptyResp); err != nil {
		t.Fatalf("failed to decode empty scan response: %v", err)
	}
	if emptyResp.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", emptyResp.Status)
	}
}

func TestServer_Deadlines_IngestionTrigger(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	now := time.Now().UTC()
	dueISO := now.AddDate(0, 0, 5).Format("2006-01-02")
	pdfContent := documents.MakeSamplePDF(fmt.Sprintf("Invoice #900. Total Due: $350.00 by %s", dueISO))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "invoice_900.pdf")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write(pdfContent)
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/documents/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on upload, got %d: %s", w.Code, w.Body.String())
	}

	// Scanner runs asynchronously upon dms.document_ingested. Wait briefly for scan completion.
	var deadlines []*store.DocumentDeadline
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		reqQuery := httptest.NewRequest("GET", "/api/v1/documents/deadlines?status=pending&within_days=14", nil)
		wQuery := httptest.NewRecorder()
		srv.router.ServeHTTP(wQuery, reqQuery)
		if wQuery.Code == http.StatusOK {
			_ = json.Unmarshal(wQuery.Body.Bytes(), &deadlines)
			if len(deadlines) > 0 {
				break
			}
		}
	}

	if len(deadlines) == 0 {
		t.Fatalf("expected deadline extracted from ingested document, got 0")
	}
	if deadlines[0].DueDate != dueISO {
		t.Errorf("expected due_date %q, got %q", dueISO, deadlines[0].DueDate)
	}
	if deadlines[0].Amount == nil || *deadlines[0].Amount != 350.00 {
		t.Errorf("expected amount 350.00, got %v", deadlines[0].Amount)
	}
}

func TestIMAP_CustomTLS(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Save IMAP configuration with custom TLS parameters (insecure_skip_verify & ca_cert_path)
	savePayload := map[string]any{
		"host":                 "imap.internal.corp",
		"port":                 993,
		"username":             "archivist@internal.corp",
		"password":             "InternalPassword123!",
		"folder":               "INBOX",
		"ssl_enabled":          true,
		"insecure_skip_verify": true,
		"ca_cert_path":         "/etc/ssl/certs/internal-ca.crt",
		"enabled":              true,
	}
	payloadBytes, _ := json.Marshal(savePayload)
	req := httptest.NewRequest("POST", "/api/v1/documents/imap", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK saving IMAP config, got %d: %s", w.Code, w.Body.String())
	}

	var saveResp IMAPConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("failed decoding save response: %v", err)
	}
	if !saveResp.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=true in response")
	}
	if saveResp.CACertPath != "/etc/ssl/certs/internal-ca.crt" {
		t.Errorf("expected CACertPath='/etc/ssl/certs/internal-ca.crt', got %q", saveResp.CACertPath)
	}

	// 2. GET returns custom TLS fields
	req = httptest.NewRequest("GET", "/api/v1/documents/imap", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK getting IMAP config, got %d", w.Code)
	}
	var getResp IMAPConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed decoding get response: %v", err)
	}
	if !getResp.InsecureSkipVerify || getResp.CACertPath != "/etc/ssl/certs/internal-ca.crt" {
		t.Errorf("unexpected TLS fields in GET response: %+v", getResp)
	}

	// 3. Test IMAP connection endpoint with invalid CA path -> 400 Bad Request
	testBadCAPayload := map[string]any{
		"host":         "imap.internal.corp",
		"port":         993,
		"username":     "archivist@internal.corp",
		"password":     "InternalPassword123!",
		"ca_cert_path": "/nonexistent/invalid-ca.crt",
	}
	badCABytes, _ := json.Marshal(testBadCAPayload)
	req = httptest.NewRequest("POST", "/api/v1/documents/imap/test", bytes.NewReader(badCABytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid CA cert path, got %d: %s", w.Code, w.Body.String())
	}
}
