package documents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"sovereign/internal/store"
)

func TestRegexParser_InvoiceUSD_ISO(t *testing.T) {
	parser := NewRegexParser()
	text := "Invoice #482. Total Due: $150.00 by 2026-10-28"

	obligations := parser.ParseObligations(text, "Invoice #482")
	if len(obligations) != 1 {
		t.Fatalf("expected 1 obligation, got %d", len(obligations))
	}

	obl := obligations[0]
	if obl.DueDate != "2026-10-28" {
		t.Errorf("expected due_date '2026-10-28', got %q", obl.DueDate)
	}
	if obl.Amount == nil || *obl.Amount != 150.00 {
		t.Errorf("expected amount 150.00, got %v", obl.Amount)
	}
	if obl.Currency != "USD" {
		t.Errorf("expected currency 'USD', got %q", obl.Currency)
	}
	if obl.Title != "Invoice #482" {
		t.Errorf("expected title 'Invoice #482', got %q", obl.Title)
	}
}

func TestRegexParser_GermanInvoice_EUR(t *testing.T) {
	parser := NewRegexParser()
	text := "Fälligkeitsdatum: 25.10.2026. Gesamtbetrag: 89,50 €"

	obligations := parser.ParseObligations(text, "Rechnung")
	if len(obligations) != 1 {
		t.Fatalf("expected 1 obligation, got %d", len(obligations))
	}

	obl := obligations[0]
	if obl.DueDate != "2026-10-25" {
		t.Errorf("expected due_date '2026-10-25', got %q", obl.DueDate)
	}
	if obl.Amount == nil || *obl.Amount != 89.50 {
		t.Errorf("expected amount 89.50, got %v", obl.Amount)
	}
	if obl.Currency != "EUR" {
		t.Errorf("expected currency 'EUR', got %q", obl.Currency)
	}
}

func TestRegexParser_GermanCancellationTerm(t *testing.T) {
	parser := NewRegexParser()
	text := "Kündigungsfrist bis 31.12.2026. Bitte schriftlich einreichen."

	obligations := parser.ParseObligations(text, "Vertrag")
	if len(obligations) != 1 {
		t.Fatalf("expected 1 obligation, got %d", len(obligations))
	}

	obl := obligations[0]
	if obl.DueDate != "2026-12-31" {
		t.Errorf("expected due_date '2026-12-31', got %q", obl.DueDate)
	}
}

func TestRegexParser_MultilingualMonthNames(t *testing.T) {
	parser := NewRegexParser()

	// English: due by October 25, 2026
	enText := "Please note that payment is due by October 25, 2026 for policy renewal."
	enObls := parser.ParseObligations(enText, "Policy")
	if len(enObls) != 1 {
		t.Fatalf("expected 1 English obligation, got %d", len(enObls))
	}
	if enObls[0].DueDate != "2026-10-25" {
		t.Errorf("expected English due_date '2026-10-25', got %q", enObls[0].DueDate)
	}

	// German: fällig am 15. November 2026
	deText := "Der Rechnungsbetrag ist fällig am 15. November 2026."
	deObls := parser.ParseObligations(deText, "Rechnung")
	if len(deObls) != 1 {
		t.Fatalf("expected 1 German obligation, got %d", len(deObls))
	}
	if deObls[0].DueDate != "2026-11-15" {
		t.Errorf("expected German due_date '2026-11-15', got %q", deObls[0].DueDate)
	}
}

func TestRegexParser_Currencies(t *testing.T) {
	parser := NewRegexParser()

	tests := []struct {
		name     string
		text     string
		expected float64
		currency string
	}{
		{"USD", "Due on 2026-11-01. Balance Due: $120.00", 120.00, "USD"},
		{"EUR Suffix", "Zahlbar bis 01.11.2026: 120,00 €", 120.00, "EUR"},
		{"EUR Prefix", "Due date: 2026-11-01, Total: €120.00", 120.00, "EUR"},
		{"GBP", "Pay by 2026-11-01: £120.00", 120.00, "GBP"},
		{"CHF Suffix", "Fällig zum 01.11.2026: 120.00 CHF", 120.00, "CHF"},
		{"CHF Prefix", "Due by 2026-11-01: CHF 120.00", 120.00, "CHF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obls := parser.ParseObligations(tt.text, "Doc")
			if len(obls) != 1 {
				t.Fatalf("expected 1 obligation, got %d", len(obls))
			}
			if obls[0].Amount == nil || *obls[0].Amount != tt.expected {
				t.Errorf("expected amount %f, got %v", tt.expected, obls[0].Amount)
			}
			if obls[0].Currency != tt.currency {
				t.Errorf("expected currency %s, got %s", tt.currency, obls[0].Currency)
			}
		})
	}
}

func TestRegexParser_InvalidCalendarDates(t *testing.T) {
	parser := NewRegexParser()

	// 31.02.2026 does not exist!
	text := "Due by 31.02.2026. Also due on 2026-02-31. Non-leap year 29.02.2025."
	obls := parser.ParseObligations(text, "Doc")
	if len(obls) != 0 {
		t.Errorf("expected 0 obligations for invalid calendar dates, got %d: %+v", len(obls), obls)
	}
}

func TestGatewayLLMExtractor_SuccessAndHeaders(t *testing.T) {
	var receivedPriority, receivedLane string
	var receivedTemp float64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPriority = r.Header.Get("X-Priority")
		receivedLane = r.Header.Get("X-Lane")

		var req chatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		receivedTemp = req.Temperature

		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatMessage{
						Role: "assistant",
						Content: `{
							"obligations": [
								{
									"title": "Contractual Renewal",
									"due_date": "2026-11-20",
									"amount": 250.00,
									"currency": "EUR",
									"snippet": "Contractual renewal auto-extends unless cancelled by 2026-11-20"
								}
							]
						}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	extractor := NewGatewayLLMExtractor(server.URL, nil)
	obls, err := extractor.ExtractObligations(context.Background(), "Renewal clause...", "Contract")
	if err != nil {
		t.Fatalf("ExtractObligations failed: %v", err)
	}

	if len(obls) != 1 {
		t.Fatalf("expected 1 obligation from LLM, got %d", len(obls))
	}
	if obls[0].DueDate != "2026-11-20" {
		t.Errorf("expected due_date '2026-11-20', got %q", obls[0].DueDate)
	}
	if obls[0].Amount == nil || *obls[0].Amount != 250.00 {
		t.Errorf("expected amount 250.00, got %v", obls[0].Amount)
	}

	// Verify required background lane headers and temperature
	if receivedPriority != "background" {
		t.Errorf("expected X-Priority 'background', got %q", receivedPriority)
	}
	if receivedLane != "batch" {
		t.Errorf("expected X-Lane 'batch', got %q", receivedLane)
	}
	if receivedTemp != 0.1 {
		t.Errorf("expected temperature 0.1, got %f", receivedTemp)
	}
}

func TestGatewayLLMExtractor_FallbackOn503OrMalformedJSON(t *testing.T) {
	// 503 Safe Stub response
	server503 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Service Unavailable - Safe Stub Mode", http.StatusServiceUnavailable)
	}))
	defer server503.Close()

	extractor503 := NewGatewayLLMExtractor(server503.URL, nil)
	_, err := extractor503.ExtractObligations(context.Background(), "Some text", "Doc")
	if err == nil {
		t.Errorf("expected error on 503, got nil")
	}

	// Malformed JSON response
	serverMalformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatMessage{
						Role:    "assistant",
						Content: `Not a JSON { broken `,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer serverMalformed.Close()

	extractorMalformed := NewGatewayLLMExtractor(serverMalformed.URL, nil)
	_, err = extractorMalformed.ExtractObligations(context.Background(), "Some text", "Doc")
	if err == nil {
		t.Errorf("expected error on malformed JSON, got nil")
	}
}

func TestObligationScanner_TwoTier_DaytimeVsNighttime(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "scanner_test.db"))
	if err != nil {
		t.Fatalf("Open store failed: %v", err)
	}
	defer db.Close()

	dStore, err := store.NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	var broadcastedEvents []string
	broadcaster := func(event string, data any) {
		broadcastedEvents = append(broadcastedEvents, event)
	}

	llmCalled := false
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalled = true
		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatMessage{
						Role: "assistant",
						Content: `{"obligations": [{"due_date": "2026-10-25", "amount": 100.00, "currency": "USD", "title": "Refined"}]}`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer llmServer.Close()

	extractor := NewGatewayLLMExtractor(llmServer.URL, nil)
	scanner := NewObligationScanner(dStore, extractor, broadcaster, nil)

	// Scenario 1: Daytime ingestion at 14:00
	daytime := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	scanner.SetNowFunc(func() time.Time { return daytime })

	if scanner.IsNighttime(daytime) {
		t.Errorf("14:00 should not be nighttime")
	}

	docText := "Invoice #482. Total Due: $150.00 by 2026-10-28"
	deadlines, err := scanner.ScanDocumentText(context.Background(), "doc-daytime", "Invoice", docText, false)
	if err != nil {
		t.Fatalf("ScanDocumentText failed: %v", err)
	}

	// LLM should NOT be called during daytime
	if llmCalled {
		t.Errorf("expected LLM not to be called during daytime (14:00)")
	}

	// Document must be queued for nightly pass
	if scanner.QueuedCount() != 1 {
		t.Errorf("expected 1 document queued for nightly pass, got %d", scanner.QueuedCount())
	}

	// Regex results should be returned immediately
	if len(deadlines) != 1 {
		t.Fatalf("expected 1 deadline from Tier 1 regex, got %d", len(deadlines))
	}
	if deadlines[0].DueDate != "2026-10-28" {
		t.Errorf("expected due_date '2026-10-28', got %q", deadlines[0].DueDate)
	}
	if deadlines[0].Status != store.DeadlineStatusPending {
		t.Errorf("expected status 'pending' (due in 12 days), got %q", deadlines[0].Status)
	}

	// SSE event must be broadcast for impending deadline
	if len(broadcastedEvents) != 1 || broadcastedEvents[0] != EventDeadlineDetected {
		t.Errorf("expected dms.deadline_detected SSE event, got %v", broadcastedEvents)
	}

	// Scenario 2: Nighttime execution at 02:30
	nighttime := time.Date(2026, 10, 17, 2, 30, 0, 0, time.UTC)
	scanner.SetNowFunc(func() time.Time { return nighttime })

	if !scanner.IsNighttime(nighttime) {
		t.Errorf("02:30 must be recognized as nighttime")
	}

	llmCalled = false
	deadlinesNight, err := scanner.ScanDocumentText(context.Background(), "doc-night", "Contract", "Ambiguous renewal by 2026-10-25", false)
	if err != nil {
		t.Fatalf("ScanDocumentText nighttime failed: %v", err)
	}

	// LLM MUST be called during nighttime window (01:00-07:00)
	if !llmCalled {
		t.Errorf("expected LLM to be called during nighttime window (02:30)")
	}
	if len(deadlinesNight) != 1 || deadlinesNight[0].Title != "Refined" {
		t.Errorf("expected refined obligation from LLM, got %+v", deadlinesNight)
	}

	// Scenario 3: ProcessNightlyBatch processes queued daytime docs
	processed, err := scanner.ProcessNightlyBatch(context.Background())
	if err != nil {
		t.Fatalf("ProcessNightlyBatch failed: %v", err)
	}
	if processed < 1 {
		t.Errorf("expected at least 1 processed nightly batch item, got %d", processed)
	}
	if scanner.QueuedCount() != 0 {
		t.Errorf("expected queue to be empty after nightly batch, got %d", scanner.QueuedCount())
	}
}

func TestObligationScanner_ThresholdFilteringAndDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "threshold_test.db"))
	if err != nil {
		t.Fatalf("Open store failed: %v", err)
	}
	defer db.Close()

	dStore, err := store.NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	broadcasterEvents := 0
	broadcaster := func(event string, data any) {
		if event == EventDeadlineDetected {
			broadcasterEvents++
		}
	}

	scanner := NewObligationScanner(dStore, nil, broadcaster, nil)
	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	scanner.SetNowFunc(func() time.Time { return now })

	// 1. Deadline in 45 days (2026-11-30) -> beyond 14 day threshold: status must be 'future'
	futureDoc := "Contract renewal notice: due by 2026-11-30."
	d1, err := scanner.ScanDocumentText(context.Background(), "doc-future", "Contract", futureDoc, false)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(d1) != 1 {
		t.Fatalf("expected 1 deadline, got %d", len(d1))
	}
	if d1[0].Status != store.DeadlineStatusFuture {
		t.Errorf("expected status 'future' for 45-day deadline, got %q", d1[0].Status)
	}
	// 'future' deadline must NOT emit SSE event
	if broadcasterEvents != 0 {
		t.Errorf("expected 0 SSE events for future deadline, got %d", broadcasterEvents)
	}

	// 2. Deadline in 10 days (2026-10-26) -> within 14 day threshold: status must be 'pending'
	pendingDoc := "Electricity bill. Payment due: $80.00 on 2026-10-26"
	d2, err := scanner.ScanDocumentText(context.Background(), "doc-pending", "Electricity", pendingDoc, false)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(d2) != 1 {
		t.Fatalf("expected 1 deadline, got %d", len(d2))
	}
	if d2[0].Status != store.DeadlineStatusPending {
		t.Errorf("expected status 'pending' for 10-day deadline, got %q", d2[0].Status)
	}
	// 'pending' deadline MUST emit SSE event
	if broadcasterEvents != 1 {
		t.Errorf("expected 1 SSE event for pending deadline, got %d", broadcasterEvents)
	}

	// 3. Duplicate ingestion of identical document
	d2Dup, err := scanner.ScanDocumentText(context.Background(), "doc-pending", "Electricity", pendingDoc, false)
	if err != nil {
		t.Fatalf("Duplicate scan failed: %v", err)
	}
	if len(d2Dup) != 1 {
		t.Fatalf("expected 1 deadline, got %d", len(d2Dup))
	}
	if d2Dup[0].ID != d2[0].ID {
		t.Errorf("expected deduplicated ID %q, got %q", d2[0].ID, d2Dup[0].ID)
	}

	// List pending: should have exactly 1 record
	list, err := dStore.List(context.Background(), store.DeadlineStatusPending, 14, now)
	if err != nil {
		t.Fatalf("List pending failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 deduplicated pending deadline, got %d", len(list))
	}

	// 4. Past date older than 30 days: ignored
	pastDoc := "Old invoice due by 2026-08-01."
	dPast, err := scanner.ScanDocumentText(context.Background(), "doc-old", "Old Doc", pastDoc, false)
	if err != nil {
		t.Fatalf("Past scan failed: %v", err)
	}
	if len(dPast) != 0 {
		t.Errorf("expected past deadline older than 30 days to be skipped, got %d", len(dPast))
	}
}

func TestRegexParser_NonDueDateOnlyReturnsNil(t *testing.T) {
	parser := NewRegexParser()
	text := "Rechnungsdatum: 12.03.2026. Kundennummer: 98765. Vielen Dank für Ihren Einkauf."

	obligations := parser.ParseObligations(text, "Rechnung")
	if len(obligations) != 0 {
		t.Fatalf("expected 0 obligations when only non-due dates (e.g. invoice date) exist, got %d", len(obligations))
	}
}

func TestRegexParser_TotalAmountSelectionAndThousandsComma(t *testing.T) {
	parser := NewRegexParser()
	text := "Item A: $10.00\nItem B: $25.00\nTotal Due: $1,250 by 2026-11-15"

	obligations := parser.ParseObligations(text, "Invoice #101")
	if len(obligations) != 1 {
		t.Fatalf("expected 1 obligation, got %d", len(obligations))
	}
	obl := obligations[0]
	if obl.Amount == nil || *obl.Amount != 1250.00 {
		t.Errorf("expected parsed total amount 1250.00, got %v", obl.Amount)
	}
	if obl.Currency != "USD" {
		t.Errorf("expected currency 'USD', got %q", obl.Currency)
	}
	if obl.DueDate != "2026-11-15" {
		t.Errorf("expected due date '2026-11-15', got %q", obl.DueDate)
	}
}

func TestRegexParser_GermanMayAndMarch(t *testing.T) {
	parser := NewRegexParser()

	// Mai
	textMai := "Zahlbar bis 20. Mai 2026. Gesamtbetrag: 45,00 €"
	oblsMai := parser.ParseObligations(textMai, "Rechnung Nr. 50")
	if len(oblsMai) != 1 {
		t.Fatalf("expected 1 obligation for Mai, got %d", len(oblsMai))
	}
	if oblsMai[0].DueDate != "2026-05-20" {
		t.Errorf("expected due date '2026-05-20', got %q", oblsMai[0].DueDate)
	}
	if oblsMai[0].Amount == nil || *oblsMai[0].Amount != 45.00 {
		t.Errorf("expected amount 45.00, got %v", oblsMai[0].Amount)
	}

	// Mrz
	textMrz := "Fällig zum 10. Mrz 2026. Gesamtbetrag: 30,00 €"
	oblsMrz := parser.ParseObligations(textMrz, "Rechnung Nr. 51")
	if len(oblsMrz) != 1 {
		t.Fatalf("expected 1 obligation for Mrz, got %d", len(oblsMrz))
	}
	if oblsMrz[0].DueDate != "2026-03-10" {
		t.Errorf("expected due date '2026-03-10', got %q", oblsMrz[0].DueDate)
	}
}

func TestObligationScanner_NightlyPromotionToPending(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "promotion_test.db"))
	if err != nil {
		t.Fatalf("Open store failed: %v", err)
	}
	defer db.Close()

	dStore, err := store.NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	broadcasterEvents := 0
	broadcaster := func(event string, data any) {
		if event == EventDeadlineDetected {
			broadcasterEvents++
		}
	}

	scanner := NewObligationScanner(dStore, nil, broadcaster, nil)

	// Ingest deadline at day 0: 25 days in future -> status 'future'
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	scanner.SetNowFunc(func() time.Time { return t0 })

	doc := "Insurance policy renewal due by 2026-10-26. Total: $200.00"
	deadlines, err := scanner.ScanDocumentText(context.Background(), "doc-ins", "Insurance", doc, false)
	if err != nil {
		t.Fatalf("ScanDocumentText failed: %v", err)
	}
	if len(deadlines) != 1 || deadlines[0].Status != store.DeadlineStatusFuture {
		t.Fatalf("expected 1 future deadline, got %+v", deadlines)
	}
	if broadcasterEvents != 0 {
		t.Errorf("expected 0 SSE events for initial future deadline, got %d", broadcasterEvents)
	}

	// Advance time by 15 days: now 10 days remaining (<= 14)
	t1 := t0.AddDate(0, 0, 15)
	scanner.SetNowFunc(func() time.Time { return t1 })

	processed, err := scanner.ProcessNightlyBatch(context.Background())
	if err != nil {
		t.Fatalf("ProcessNightlyBatch failed: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 promoted deadline, got %d", processed)
	}
	if broadcasterEvents != 1 {
		t.Errorf("expected 1 SSE event upon promotion to pending, got %d", broadcasterEvents)
	}

	// Verify status in store is now pending
	saved, err := dStore.Get(context.Background(), deadlines[0].ID)
	if err != nil {
		t.Fatalf("Get deadline failed: %v", err)
	}
	if saved.Status != store.DeadlineStatusPending {
		t.Errorf("expected status 'pending' after promotion, got %q", saved.Status)
	}
}

func TestObligationScanner_LLMFallbackToRegexOn503(t *testing.T) {
	// Mock AI gateway returning 503
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "AI Service Unavailable", http.StatusServiceUnavailable)
	}))
	defer gateway.Close()

	llm := NewGatewayLLMExtractor(gateway.URL, nil)
	scanner := NewObligationScanner(nil, llm, nil, nil)

	doc := "Invoice #777. Payment due: 2026-11-20. Total Due: $500.00"
	// Force LLM extraction, which will fail with 503 and fall back to Tier 1 regex baseline
	results, err := scanner.ScanDocumentText(context.Background(), "doc-777", "Invoice #777", doc, true)
	if err != nil {
		t.Fatalf("ScanDocumentText should not fail on LLM 503 fallback, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 fallback regex obligation, got %d", len(results))
	}
	if results[0].DueDate != "2026-11-20" {
		t.Errorf("expected due date '2026-11-20', got %q", results[0].DueDate)
	}
	if results[0].Amount == nil || *results[0].Amount != 500.00 {
		t.Errorf("expected amount 500.00, got %v", results[0].Amount)
	}
}
