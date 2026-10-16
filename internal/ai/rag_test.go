package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type mockInferenceClient struct {
	lastReq  ChatCompletionRequest
	response string
	err      error
}

func (m *mockInferenceClient) Complete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	respText := m.response
	if respText == "" {
		respText = "Your deductible for storm damage is $500, according to page 2 of Car Insurance Policy [Car Insurance Policy (Page 2)](/docs/documents/1)."
	}
	return &ChatCompletionResponse{
		ID: "chatcmpl-test-123",
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatCompletionMessage{
					Role:    "assistant",
					Content: respText,
				},
				FinishReason: "stop",
			},
		},
	}, nil
}

// setupTestPaperlessDB creates an SQLite database mimicking Paperless-ngx schema.
func setupTestPaperlessDB(t *testing.T) (string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "rag-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	docsDir := filepath.Join(tempDir, "documents", "data")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("failed to create docs dir: %v", err)
	}

	dbPath := filepath.Join(docsDir, "db.sqlite3")
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	// Create documents_document table
	createSQL := `
	CREATE TABLE documents_document (
		id INTEGER PRIMARY KEY,
		title VARCHAR(128) NOT NULL,
		content TEXT NOT NULL,
		created DATETIME,
		modified DATETIME
	);`
	if _, err := db.Exec(createSQL); err != nil {
		t.Fatalf("failed to create documents_document: %v", err)
	}

	// Insert test documents
	doc1 := `Sovereign Insurance Services. Policy number: POL-98231.
Insured: Alex Doe.
Coverage Details:
Comprehensive and collision coverage active.
` + "\x0c" + `Page 2: Deductibles and Limits.
The deductible for storm damage and hail is $500.
Collision deductible is $1000.
Please report claims within 14 days of an incident.`

	doc2 := `City Utility Billing - Quarterly Electricity Invoice.
Account: ACC-44219. Billing Period: Q3 2026.
Total amount due: 184.50 EUR by November 15, 2026.`

	doc3 := `Household Appliance Manual - Dishwasher Series 7.
--- Page 1 ---
General safety guidelines and installation instructions.
--- Page 2 ---
Water intake hose and leak prevention settings.
--- Page 3 ---
Error codes: E04 indicates water drainage blockage. Clean the drain pump filter.`

	insertSQL := `INSERT INTO documents_document (id, title, content, created, modified) VALUES
		(1, 'Car Insurance Policy', ?, datetime('now'), datetime('now')),
		(2, 'Electricity Bill Q3', ?, datetime('now'), datetime('now')),
		(3, 'Dishwasher Manual', ?, datetime('now'), datetime('now'));`

	if _, err := db.Exec(insertSQL, doc1, doc2, doc3); err != nil {
		t.Fatalf("failed to insert test documents: %v", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}
	return tempDir, cleanup
}

func TestRAGEngine_SearchAndFTS5(t *testing.T) {
	tempDir, cleanup := setupTestPaperlessDB(t)
	defer cleanup()

	engine := NewRAGEngine(RAGConfig{
		DataDir: tempDir,
	})
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Search for car insurance deductible
	resp, err := engine.Search(ctx, RAGSearchRequest{
		Query:    "deductible on car insurance",
		Limit:    5,
		Language: "en",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if resp.TotalMatches == 0 {
		t.Fatalf("expected matches for 'deductible on car insurance', got 0")
	}

	match := resp.Results[0]
	if match.DocumentID != 1 {
		t.Errorf("expected document ID 1, got %d", match.DocumentID)
	}
	if match.Title != "Car Insurance Policy" {
		t.Errorf("expected title 'Car Insurance Policy', got %s", match.Title)
	}
	if match.PageNumber != 2 {
		t.Errorf("expected match on Page 2 (after \\x0c), got %d", match.PageNumber)
	}
	if !strings.Contains(match.Snippet, "deductible") {
		t.Errorf("expected snippet to contain 'deductible', got: %s", match.Snippet)
	}
	if match.URL != "/docs/documents/1#page=2" {
		t.Errorf("expected URL '/docs/documents/1#page=2', got: %s", match.URL)
	}
}

func TestRAGEngine_ExactPageExtraction(t *testing.T) {
	// Scenario 1: Form feed character (\x0c)
	contentWithFF := "Page one header\nSome introductory content\n" +
		"\x0cPage two starts here\nDetails and deductible info\n" +
		"\x0cPage three starts here\nFinal signature"

	offsetP1 := strings.Index(contentWithFF, "introductory")
	offsetP2 := strings.Index(contentWithFF, "deductible")
	offsetP3 := strings.Index(contentWithFF, "signature")

	if p := CalculatePageNumber(contentWithFF, offsetP1); p != 1 {
		t.Errorf("expected page 1, got %d", p)
	}
	if p := CalculatePageNumber(contentWithFF, offsetP2); p != 2 {
		t.Errorf("expected page 2, got %d", p)
	}
	if p := CalculatePageNumber(contentWithFF, offsetP3); p != 3 {
		t.Errorf("expected page 3, got %d", p)
	}

	// Scenario 2: OCR page headers (--- Page N ---)
	contentWithHeaders := "--- Page 1 ---\nDocument Title\n--- Page 2 ---\nSection 2 terms\n--- Page 3 ---\nError code E04 drainage"
	offsetCode := strings.Index(contentWithHeaders, "drainage")

	if p := CalculatePageNumber(contentWithHeaders, offsetCode); p != 3 {
		t.Errorf("expected page 3 from OCR header, got %d", p)
	}

	// Scenario 3: Page headers with [Page N]
	contentBracket := "[Page 1]\nIntro\n[Page 4]\nContract termination clause"
	offsetTerm := strings.Index(contentBracket, "termination")
	if p := CalculatePageNumber(contentBracket, offsetTerm); p != 4 {
		t.Errorf("expected page 4 from [Page 4], got %d", p)
	}

	// Scenario 4: Absence of markers falls back to Page 1
	plainText := "Single page letter with no markers or form feeds."
	offsetLetter := strings.Index(plainText, "letter")
	if p := CalculatePageNumber(plainText, offsetLetter); p != 1 {
		t.Errorf("expected fallback to page 1, got %d", p)
	}
}

func TestHeadroomTruncator_BufferAndPriority(t *testing.T) {
	truncator := NewHeadroomTruncator(4096, 512, EstimateTokens)

	systemPrompt := GetRAGSystemPrompt("en")
	userQuery := "What is our deductible for storm damage?"

	// Case 1: Normal excerpts fitting well within headroom
	normalExcerpts := []RAGSearchResult{
		{
			DocumentID: 1,
			Title:      "Car Insurance Policy",
			PageNumber: 2,
			Snippet:    "The deductible for storm damage is $500.",
			Score:      -1.5,
			URL:        "/docs/documents/1",
		},
	}

	resNormal := truncator.Truncate(systemPrompt, userQuery, normalExcerpts, "en")
	if resNormal.Truncated {
		t.Errorf("expected normal excerpts not to be truncated, got truncated=true")
	}
	if len(resNormal.RetainedChunks) != 1 {
		t.Fatalf("expected 1 retained chunk, got %d", len(resNormal.RetainedChunks))
	}
	if resNormal.Notice != "" {
		t.Errorf("expected empty notice when not truncated, got: %s", resNormal.Notice)
	}

	// Case 2: Exceeding headroom (>3500 tokens / 20+ large chunks)
	var largeExcerpts []RAGSearchResult
	largeChunkSnippet := strings.Repeat("Detailed contract clause with terms and provisions. ", 50) // ~500 runes = ~125 tokens
	for i := 1; i <= 30; i++ {
		largeExcerpts = append(largeExcerpts, RAGSearchResult{
			DocumentID: int64(i),
			Title:      fmt.Sprintf("Contract Document Volume %d", i),
			PageNumber: i,
			Snippet:    largeChunkSnippet,
			Score:      float64(-30 + i), // Doc 1 has highest rank score (-29), Doc 30 has lowest (0)
			URL:        fmt.Sprintf("/docs/documents/%d", i),
		})
	}

	resTruncated := truncator.Truncate(systemPrompt, userQuery, largeExcerpts, "en")
	if !resTruncated.Truncated {
		t.Errorf("expected large excerpts to be truncated, got truncated=false")
	}
	if len(resTruncated.RetainedChunks) >= 30 {
		t.Errorf("expected fewer than 30 retained chunks, got %d", len(resTruncated.RetainedChunks))
	}
	if len(resTruncated.RetainedChunks) == 0 {
		t.Errorf("expected some chunks to be retained, got 0")
	}

	// Invariants: Total tokens used must never exceed contextCapacity - 512
	availableHeadroom := 4096 - 512 - EstimateTokens(systemPrompt) - EstimateTokens(userQuery)
	var usedExcerptTokens int
	for _, chunk := range resTruncated.RetainedChunks {
		usedExcerptTokens += EstimateTokens(FormatExcerptChunk(chunk))
	}

	if usedExcerptTokens > availableHeadroom {
		t.Errorf("retained excerpts (%d tokens) exceeded available headroom (%d tokens)", usedExcerptTokens, availableHeadroom)
	}

	// Notice must match localized EN notice
	expectedNoticeEN := "Note: Document excerpts were prioritized to fit your active context window."
	if resTruncated.Notice != expectedNoticeEN {
		t.Errorf("expected notice %q, got %q", expectedNoticeEN, resTruncated.Notice)
	}
}

func TestHeadroomTruncator_LocalizationNotices(t *testing.T) {
	truncator := NewHeadroomTruncator(1000, 512, EstimateTokens)
	systemPrompt := "System prompt for test"
	userQuery := "User question"

	var heavyExcerpts []RAGSearchResult
	for i := 1; i <= 10; i++ {
		heavyExcerpts = append(heavyExcerpts, RAGSearchResult{
			DocumentID: int64(i),
			Title:      "Document",
			PageNumber: 1,
			Snippet:    strings.Repeat("Heavy paragraph content. ", 30),
			URL:        "/docs/documents/1",
		})
	}

	tests := []struct {
		lang           string
		expectedNotice string
	}{
		{
			lang:           "en",
			expectedNotice: "Note: Document excerpts were prioritized to fit your active context window.",
		},
		{
			lang:           "de",
			expectedNotice: "Hinweis: Dokumentenauszüge wurden priorisiert, um in das aktive Kontextfenster zu passen.",
		},
		{
			lang:           "es",
			expectedNotice: "Nota: Los extractos de documentos se priorizaron para ajustarse a su ventana de contexto activa.",
		},
		{
			lang:           "fr",
			expectedNotice: "Remarque : Les extraits de documents ont été hiérarchisés pour s'adapter à votre fenêtre de contexte active.",
		},
		{
			lang:           "unsupported_lang",
			expectedNotice: "Note: Document excerpts were prioritized to fit your active context window.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.lang, func(t *testing.T) {
			res := truncator.Truncate(systemPrompt, userQuery, heavyExcerpts, tc.lang)
			if !res.Truncated {
				t.Fatalf("expected truncated=true")
			}
			if res.Notice != tc.expectedNotice {
				t.Errorf("expected notice %q, got %q", tc.expectedNotice, res.Notice)
			}
		})
	}
}

func TestRAGEngine_QueryWithInferenceAndCitations(t *testing.T) {
	tempDir, cleanup := setupTestPaperlessDB(t)
	defer cleanup()

	mockClient := &mockInferenceClient{
		response: "Your deductible is $500 for storm damage, according to page 2 of Car Insurance Policy.",
	}

	engine := NewRAGEngine(RAGConfig{
		DataDir:         tempDir,
		InferenceClient: mockClient,
	})
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Query with match
	resp, err := engine.Query(ctx, RAGQueryRequest{
		Query:    "What is our deductible for storm damage on car insurance?",
		Language: "en",
	})
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}

	if !strings.Contains(resp.Answer, "$500") {
		t.Errorf("expected answer to contain '$500', got: %s", resp.Answer)
	}

	if len(resp.Citations) == 0 {
		t.Fatalf("expected citations in query response, got 0")
	}

	citation := resp.Citations[0]
	if citation.DocumentID != 1 {
		t.Errorf("expected citation doc ID 1, got %d", citation.DocumentID)
	}
	if citation.PageNumber != 2 {
		t.Errorf("expected citation page number 2, got %d", citation.PageNumber)
	}
	if citation.URL != "/docs/documents/1#page=2" {
		t.Errorf("expected citation URL '/docs/documents/1#page=2', got %s", citation.URL)
	}

	// Verify prompt passed to mockClient contained grounded context
	lastMessages := mockClient.lastReq.Messages
	if len(lastMessages) < 2 {
		t.Fatalf("expected at least 2 messages in LLM request, got %d", len(lastMessages))
	}
	systemMsg := lastMessages[0].Content
	if !strings.Contains(systemMsg, "Car Insurance Policy (Page 2)") {
		t.Errorf("expected system message to contain citation header, got: %s", systemMsg)
	}
}

func TestRAGEngine_UnprovisionedDBHandling(t *testing.T) {
	emptyDir, err := os.MkdirTemp("", "rag-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	engine := NewRAGEngine(RAGConfig{
		DataDir: emptyDir,
	})
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Search against unprovisioned DB
	searchRes, err := engine.Search(ctx, RAGSearchRequest{
		Query:    "deductible",
		Language: "de",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if searchRes.TotalMatches != 0 {
		t.Errorf("expected 0 matches for unprovisioned DB, got %d", searchRes.TotalMatches)
	}
	expectedNoticeDE := "Das Dokumentenarchiv ist noch nicht eingerichtet oder leer."
	if searchRes.Notice != expectedNoticeDE {
		t.Errorf("expected notice %q, got %q", expectedNoticeDE, searchRes.Notice)
	}

	// 2. Query against unprovisioned DB
	queryRes, err := engine.Query(ctx, RAGQueryRequest{
		Query:    "deductible",
		Language: "de",
	})
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}

	if len(queryRes.Citations) != 0 {
		t.Errorf("expected 0 citations for unprovisioned DB, got %d", len(queryRes.Citations))
	}
	if queryRes.Notice != expectedNoticeDE {
		t.Errorf("expected notice %q, got %q", expectedNoticeDE, queryRes.Notice)
	}
}

func TestRAGEngine_NoMatchingDocuments(t *testing.T) {
	tempDir, cleanup := setupTestPaperlessDB(t)
	defer cleanup()

	engine := NewRAGEngine(RAGConfig{
		DataDir: tempDir,
	})
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Query for terms that do not exist
	queryRes, err := engine.Query(ctx, RAGQueryRequest{
		Query:    "Spaceship navigation hyperdrive warp coil",
		Language: "fr",
	})
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}

	expectedMsgFR := "Aucun document correspondant n'a été trouvé dans vos archives pour répondre à cette question."
	if queryRes.Answer != expectedMsgFR {
		t.Errorf("expected answer %q, got %q", expectedMsgFR, queryRes.Answer)
	}
	if len(queryRes.Citations) != 0 {
		t.Errorf("expected 0 citations, got %d", len(queryRes.Citations))
	}
}

func TestGatewayInferenceClient_Lane1Priority(t *testing.T) {
	// Verify GatewayInferenceClient sets PriorityInteractive / Lane 1
	var capturedPriority string
	var capturedLane string
	var capturedIntent string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPriority = r.Header.Get("X-Inference-Priority")
		capturedLane = r.Header.Get("X-Lane")
		capturedIntent = r.Header.Get("X-Generation-Intent")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID: "chatcmpl-lane1-test",
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatCompletionMessage{
						Role:    "assistant",
						Content: "Tested response from lane 1.",
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer mockServer.Close()

	gw := NewGateway(GatewayConfig{
		LlamaURL: mockServer.URL,
		Governor: NewRequestGovernor(DefaultGovernorConfig()),
	})
	defer gw.Close()

	// Create dummy model file so CheckModelAvailability succeeds
	modelsDir, err := os.MkdirTemp("", "models-test-*")
	if err != nil {
		t.Fatalf("failed to create models dir: %v", err)
	}
	defer os.RemoveAll(modelsDir)

	dummyGGUF := filepath.Join(modelsDir, "test-model.gguf")
	if err := os.WriteFile(dummyGGUF, createMockGGUFBytes(3, 128, 42), 0644); err != nil {
		t.Fatalf("failed to create dummy gguf: %v", err)
	}
	gw.SetModelsDir(modelsDir)

	client := NewGatewayInferenceClient(gw)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, ChatCompletionRequest{
		Model: "test-model",
		Messages: []ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if resp.Choices[0].Message.Content != "Tested response from lane 1." {
		t.Errorf("unexpected content: %s", resp.Choices[0].Message.Content)
	}

	if capturedPriority != "interactive" {
		t.Errorf("expected X-Inference-Priority 'interactive', got: %s", capturedPriority)
	}
	if capturedLane != "1" {
		t.Errorf("expected X-Lane '1', got: %s", capturedLane)
	}
	if capturedIntent != "precise" {
		t.Errorf("expected X-Generation-Intent 'precise', got: %s", capturedIntent)
	}
}

func TestHeadroomTruncator_SingleChunkExceedsHeadroom_Sliced(t *testing.T) {
	systemPrompt := "Short system prompt" // ~5 tokens
	userQuery := "Short query"            // ~3 tokens

	// Restrict available headroom by using custom capacity
	constrainedTruncator := NewHeadroomTruncator(600, 512, EstimateTokens)
	// Headroom = 600 - 512 - 5 - 3 = 80 tokens

	largeSnippet := strings.Repeat("Extremely long document content that contains many words and exceeds the 80 token headroom limit. ", 10)
	excerpts := []RAGSearchResult{
		{
			DocumentID: 42,
			Title:      "Large Document",
			PageNumber: 1,
			Snippet:    largeSnippet,
			URL:        "/docs/documents/42",
		},
	}

	result := constrainedTruncator.Truncate(systemPrompt, userQuery, excerpts, LangEN)
	if !result.Truncated {
		t.Errorf("expected truncated to be true")
	}
	if len(result.RetainedChunks) != 1 {
		t.Fatalf("expected 1 retained chunk, got %d", len(result.RetainedChunks))
	}
	if !strings.HasSuffix(result.RetainedChunks[0].Snippet, "...") {
		t.Errorf("expected sliced snippet to end with '...', got: %s", result.RetainedChunks[0].Snippet)
	}
	if len(result.RetainedChunks[0].Snippet) >= len(largeSnippet) {
		t.Errorf("expected sliced snippet to be shorter than original")
	}
}

func TestHeadroomTruncator_PackingSubsequentChunks(t *testing.T) {
	// Headroom: 1000 - 512 - 5 - 3 = 480 tokens
	truncator := NewHeadroomTruncator(1000, 512, EstimateTokens)
	systemPrompt := "System"
	userQuery := "Query"

	// Chunk 1: small (~50 tokens) -> fits, remaining ~430
	// Chunk 2: huge (~600 tokens) -> doesn't fit, skipped
	// Chunk 3: small (~50 tokens) -> fits in remaining ~430!
	excerpts := []RAGSearchResult{
		{
			DocumentID: 1,
			Title:      "Small Doc 1",
			PageNumber: 1,
			Snippet:    "Small snippet that easily fits.",
			URL:        "/docs/documents/1",
		},
		{
			DocumentID: 2,
			Title:      "Huge Doc 2",
			PageNumber: 1,
			Snippet:    strings.Repeat("Huge content that will definitely not fit into the remaining headroom limit. ", 40),
			URL:        "/docs/documents/2",
		},
		{
			DocumentID: 3,
			Title:      "Small Doc 3",
			PageNumber: 2,
			Snippet:    "Another small snippet that should be packed into remaining headroom.",
			URL:        "/docs/documents/3#page=2",
		},
	}

	result := truncator.Truncate(systemPrompt, userQuery, excerpts, LangEN)
	if !result.Truncated {
		t.Errorf("expected truncated to be true")
	}
	if len(result.RetainedChunks) != 2 {
		t.Fatalf("expected 2 retained chunks (doc 1 and doc 3 packed), got %d", len(result.RetainedChunks))
	}
	if result.RetainedChunks[0].DocumentID != 1 {
		t.Errorf("expected first retained chunk to be doc 1, got %d", result.RetainedChunks[0].DocumentID)
	}
	if result.RetainedChunks[1].DocumentID != 3 {
		t.Errorf("expected second retained chunk to be doc 3, got %d", result.RetainedChunks[1].DocumentID)
	}
}

func TestRAGEngine_FallbackLIKESearch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rag-fallback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "db.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	// Create table WITHOUT FTS5
	_, err = db.Exec(`CREATE TABLE documents_document (
		id INTEGER PRIMARY KEY,
		title VARCHAR(128) NOT NULL,
		content TEXT NOT NULL,
		created DATETIME,
		modified DATETIME
	);
	INSERT INTO documents_document (id, title, content) VALUES
		(1, 'Insurance Policy', 'Comprehensive car policy. The deductible for hail and storm damage is $500.'),
		(2, 'Electricity Invoice', 'Monthly electricity bill 120 EUR.');`)
	if err != nil {
		t.Fatalf("failed to setup documents_document: %v", err)
	}

	engine := NewRAGEngine(RAGConfig{
		DBPath: dbPath,
	})
	defer engine.Close()

	// Query with natural language starting with stop words
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := engine.Search(ctx, RAGSearchRequest{
		Query:    "What is the deductible for storm damage?",
		Language: "en",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if resp.TotalMatches == 0 {
		t.Fatalf("expected at least 1 match via LIKE fallback, got 0")
	}
	if resp.Results[0].DocumentID != 1 {
		t.Errorf("expected doc 1, got doc %d", resp.Results[0].DocumentID)
	}
	if !strings.Contains(resp.Results[0].Snippet, "deductible") {
		t.Errorf("expected snippet to contain deductible, got: %s", resp.Results[0].Snippet)
	}
}
