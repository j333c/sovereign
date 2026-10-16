package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Default context capacity and headroom buffer constants.
const (
	DefaultContextCapacity        = 4096
	DefaultReservedResponseTokens = 512
	DefaultSearchLimit            = 5
	MaxSearchLimit                = 50
)

// Localized notices for RAG context window truncation.
var TruncationNotices = map[string]string{
	LangEN: "Note: Document excerpts were prioritized to fit your active context window.",
	LangDE: "Hinweis: Dokumentenauszüge wurden priorisiert, um in das aktive Kontextfenster zu passen.",
	LangES: "Nota: Los extractos de documentos se priorizaron para ajustarse a su ventana de contexto activa.",
	LangFR: "Remarque : Les extraits de documents ont été hiérarchisés pour s'adapter à votre fenêtre de contexte active.",
}

// Localized notices when no documents match the search/query.
var NoMatchingDocumentsMessages = map[string]string{
	LangEN: "No matching documents were found in your household archive to answer this question.",
	LangDE: "In Ihrem Dokumentenarchiv wurden keine passenden Dokumente gefunden, um diese Frage zu beantworten.",
	LangES: "No se encontraron documentos coincidentes en su archivo para responder a esta pregunta.",
	LangFR: "Aucun document correspondant n'a été trouvé dans vos archives pour répondre à cette question.",
}

// Localized notices when the Paperless database is unprovisioned.
var UnprovisionedDBNotices = map[string]string{
	LangEN: "Document archive database is unprovisioned or empty.",
	LangDE: "Das Dokumentenarchiv ist noch nicht eingerichtet oder leer.",
	LangES: "La base de datos del archivo de documentos no está aprovisionada o está vacía.",
	LangFR: "La base de données d'archives de documents n'est pas configurée ou est vide.",
}

// Localized system prompts for RAG document question answering.
var RAGSystemPrompts = map[string]string{
	LangEN: `You are the Sovereign Household AI assistant. Answer the user's question using ONLY the provided document excerpts from their household document archive. Cite the document title and exact page number for any facts you reference. Format citations clearly, for example: [Document Title (Page X)](/docs/documents/ID). If the excerpts do not contain enough information to answer, state that clearly without guessing.`,
	LangDE: `Du bist die Sovereign Haushalts-KI. Beantworte die Frage des Benutzers AUSSCHLIESSLICH anhand der bereitgestellten Dokumentenauszüge aus dem Dokumentenarchiv. Zitiere den Dokumententitel und die genaue Seitenzahl für alle Tatsachen, auf die du dich beziehst, zum Beispiel: [Dokumententitel (Seite X)](/docs/documents/ID). Wenn die Auszüge nicht genügend Informationen enthalten, gib dies deutlich an, ohne Vermutungen anzustellen.`,
	LangES: `Eres el asistente de IA del Hogar de Sovereign. Responde a la pregunta del usuario utilizando ÚNICAMENTE los extractos de documentos proporcionados de su archivo del hogar. Cita el título del documento y el número de página exacto para cualquier dato que menciones, por ejemplo: [Título del Documento (Página X)](/docs/documents/ID). Si los extractos no contienen suficiente información para responder, indícalo claramente sin inventar datos.`,
	LangFR: `Vous êtes l'assistant IA Domestique Sovereign. Répondez à la question de l'utilisateur en utilisant UNIQUEMENT les extraits de documents fournis dans ses archives. Citez le titre du document et le numéro de page exact pour toute information mentionnée, par exemple : [Titre du document (Page X)](/docs/documents/ID). Si les extraits ne contiennent pas suffisamment d'informations pour répondre, indiquez-le clairement sans faire de suppositions.`,
}

// GetTruncationNotice returns the localized truncation notice for the given language.
func GetTruncationNotice(lang string) string {
	norm := NormalizeLanguage(lang)
	if msg, ok := TruncationNotices[norm]; ok {
		return msg
	}
	return TruncationNotices[LangEN]
}

// GetNoMatchingDocumentsMessage returns the localized no-matches message.
func GetNoMatchingDocumentsMessage(lang string) string {
	norm := NormalizeLanguage(lang)
	if msg, ok := NoMatchingDocumentsMessages[norm]; ok {
		return msg
	}
	return NoMatchingDocumentsMessages[LangEN]
}

// GetUnprovisionedDBNotice returns the localized unprovisioned database notice.
func GetUnprovisionedDBNotice(lang string) string {
	norm := NormalizeLanguage(lang)
	if msg, ok := UnprovisionedDBNotices[norm]; ok {
		return msg
	}
	return UnprovisionedDBNotices[LangEN]
}

// GetRAGSystemPrompt returns the localized RAG system prompt.
func GetRAGSystemPrompt(lang string) string {
	norm := NormalizeLanguage(lang)
	if p, ok := RAGSystemPrompts[norm]; ok {
		return p
	}
	return RAGSystemPrompts[LangEN]
}

// RAGSearchResult represents a ranked document match with citation details.
type RAGSearchResult struct {
	DocumentID int64   `json:"document_id"`
	Title      string  `json:"title"`
	PageNumber int     `json:"page_number"`
	Snippet    string  `json:"snippet"`
	Score      float64 `json:"score"`
	URL        string  `json:"url"`
}

// RAGSearchRequest represents a document search request.
type RAGSearchRequest struct {
	Query    string `json:"query"`
	Limit    int    `json:"limit,omitempty"`
	Language string `json:"language,omitempty"`
}

// RAGSearchResponse represents the result of a document search.
type RAGSearchResponse struct {
	Query        string            `json:"query"`
	TotalMatches int               `json:"total_matches"`
	Results      []RAGSearchResult `json:"results"`
	Notice       string            `json:"notice,omitempty"`
}

// RAGQueryRequest represents a question answering request against the household archive.
type RAGQueryRequest struct {
	Query         string `json:"query"`
	Language      string `json:"language,omitempty"`
	Model         string `json:"model,omitempty"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
	ContextTokens int    `json:"context_tokens,omitempty"`
}

// RAGQueryResponse represents the synthesized answer and document citations.
type RAGQueryResponse struct {
	Answer    string            `json:"answer"`
	Citations []RAGSearchResult `json:"citations"`
	Truncated bool              `json:"truncated"`
	Notice    string            `json:"notice,omitempty"`
}

// ChatCompletionMessage represents an OpenAI-compatible message.
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionChoice represents a single completion choice.
type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

// ChatCompletionRequest represents an OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
	Temperature float64                 `json:"temperature,omitempty"`
	TopP        float64                 `json:"top_p,omitempty"`
	Stream      bool                    `json:"stream,omitempty"`
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Choices []ChatCompletionChoice `json:"choices"`
}

// InferenceClient executes an inference request.
type InferenceClient interface {
	Complete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// GatewayInferenceClient implements InferenceClient by routing completion requests
// through the supervisor AI Gateway with PriorityInteractive (Lane 1).
type GatewayInferenceClient struct {
	gateway Gateway
}

// NewGatewayInferenceClient creates a new GatewayInferenceClient.
func NewGatewayInferenceClient(gw Gateway) *GatewayInferenceClient {
	return &GatewayInferenceClient{gateway: gw}
}

// Complete dispatches the completion request to the AI Gateway via an internal HTTP request.
func (c *GatewayInferenceClient) Complete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c.gateway == nil {
		return nil, errors.New("ai gateway is not configured")
	}

	avail, err := c.gateway.CheckModelAvailability(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check model availability: %w", err)
	}
	if !avail {
		return nil, ErrSafeStub
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Inference-Priority", "interactive")
	httpReq.Header.Set("X-Lane", "1")
	httpReq.Header.Set("X-Generation-Intent", "precise")

	rec := httptest.NewRecorder()
	c.gateway.ServeHTTP(rec, httpReq)

	if rec.Code == http.StatusServiceUnavailable {
		return nil, ErrSafeStub
	}
	if rec.Code < 200 || rec.Code >= 300 {
		return nil, fmt.Errorf("gateway inference error (status %d): %s", rec.Code, rec.Body.String())
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal completion response: %w", err)
	}
	return &resp, nil
}

// EstimateTokens calculates a fast, conservative token estimation for prompt texts.
// Heuristic: ~4 characters per token for Latin/English text, rounded up.
func EstimateTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	tokens := (len(runes) + 3) / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

// TruncationResult holds the outcome of headroom truncation.
type TruncationResult struct {
	RetainedChunks    []RAGSearchResult
	Truncated         bool
	Notice            string
	AvailableHeadroom int
	TotalUsedTokens   int
}

// HeadroomTruncator enforces context capacity limits and reserves a dedicated
// 512-token response buffer for the LLM output.
type HeadroomTruncator struct {
	ContextCapacity        int
	ReservedResponseBuffer int
	TokenEstimator         func(string) int
}

// NewHeadroomTruncator creates a new HeadroomTruncator with default 4096 capacity and 512 reserved buffer.
func NewHeadroomTruncator(capacity, reserved int, estimator func(string) int) *HeadroomTruncator {
	if capacity <= 0 {
		capacity = DefaultContextCapacity
	}
	if reserved <= 0 {
		reserved = DefaultReservedResponseTokens
	}
	if estimator == nil {
		estimator = EstimateTokens
	}
	return &HeadroomTruncator{
		ContextCapacity:        capacity,
		ReservedResponseBuffer: reserved,
		TokenEstimator:         estimator,
	}
}

// FormatExcerptChunk formats a single document excerpt for inclusion in the prompt context.
func FormatExcerptChunk(ex RAGSearchResult) string {
	return fmt.Sprintf("--- Document Excerpt (ID: %d) ---\nSource: %s (Page %d)\nURL: %s\nContent:\n%s\n\n",
		ex.DocumentID, ex.Title, ex.PageNumber, ex.URL, ex.Snippet)
}

// Truncate checks available context headroom and retains the highest-ranking chunks that fit.
func (t *HeadroomTruncator) Truncate(systemPrompt, userQuery string, excerpts []RAGSearchResult, lang string) TruncationResult {
	capacity := t.ContextCapacity
	if capacity <= 0 {
		capacity = DefaultContextCapacity
	}
	reserved := t.ReservedResponseBuffer
	if reserved <= 0 {
		reserved = DefaultReservedResponseTokens
	}
	estimator := t.TokenEstimator
	if estimator == nil {
		estimator = EstimateTokens
	}

	systemTokens := estimator(systemPrompt)
	queryTokens := estimator(userQuery)

	availableHeadroom := capacity - reserved - systemTokens - queryTokens
	if availableHeadroom < 0 {
		availableHeadroom = 0
	}

	var retained []RAGSearchResult
	usedTokens := 0
	truncated := false

	for _, ex := range excerpts {
		chunkText := FormatExcerptChunk(ex)
		chunkTokens := estimator(chunkText)

		if usedTokens+chunkTokens <= availableHeadroom {
			retained = append(retained, ex)
			usedTokens += chunkTokens
		} else {
			truncated = true
			// If we have remaining headroom and no chunk could be retained yet,
			// slice the snippet text to fit the available headroom
			if len(retained) == 0 && availableHeadroom > 50 {
				headerTokens := estimator(fmt.Sprintf("--- Document Excerpt (ID: %d) ---\nSource: %s (Page %d)\nURL: %s\nContent:\n\n\n",
					ex.DocumentID, ex.Title, ex.PageNumber, ex.URL))
				remainingForSnippet := availableHeadroom - headerTokens
				if remainingForSnippet > 20 {
					allowedChars := remainingForSnippet * 4
					runes := []rune(ex.Snippet)
					if len(runes) > allowedChars {
						ex.Snippet = string(runes[:allowedChars]) + "..."
					}
					retained = append(retained, ex)
					usedTokens += estimator(FormatExcerptChunk(ex))
					break
				}
			}
			// Continue loop to see if smaller subsequent excerpts can fit in the remaining budget
			continue
		}
	}

	if len(retained) < len(excerpts) {
		truncated = true
	}

	notice := ""
	if truncated {
		notice = GetTruncationNotice(lang)
	}

	return TruncationResult{
		RetainedChunks:    retained,
		Truncated:         truncated,
		Notice:            notice,
		AvailableHeadroom: availableHeadroom,
		TotalUsedTokens:   systemTokens + queryTokens + usedTokens,
	}
}

// RAGConfig provides configuration options for RAGEngine.
type RAGConfig struct {
	DataDir         string
	DBPath          string
	Logger          *slog.Logger
	InferenceClient InferenceClient
	Truncator       *HeadroomTruncator
}

// RAGEngine orchestrates Paperless SQLite FTS5 search, exact page citations,
// context window headroom truncation, and LLM answer generation.
type RAGEngine struct {
	mu              sync.RWMutex
	dataDir         string
	dbPath          string
	logger          *slog.Logger
	inferenceClient InferenceClient
	truncator       *HeadroomTruncator
	dbPool          *sql.DB
}

// NewRAGEngine constructs a new RAGEngine.
func NewRAGEngine(cfg RAGConfig) *RAGEngine {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.Truncator == nil {
		cfg.Truncator = NewHeadroomTruncator(DefaultContextCapacity, DefaultReservedResponseTokens, EstimateTokens)
	}

	return &RAGEngine{
		dataDir:         cfg.DataDir,
		dbPath:          cfg.DBPath,
		logger:          cfg.Logger,
		inferenceClient: cfg.InferenceClient,
		truncator:       cfg.Truncator,
	}
}

// SetInferenceClient sets or overrides the inference client.
func (e *RAGEngine) SetInferenceClient(client InferenceClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inferenceClient = client
}

// InferenceClient returns the active inference client.
func (e *RAGEngine) InferenceClient() InferenceClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inferenceClient
}

// SetDBPath sets or overrides the target SQLite database path.
func (e *RAGEngine) SetDBPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.dbPool != nil {
		_ = e.dbPool.Close()
		e.dbPool = nil
	}
	e.dbPath = path
}

// Close closes any open SQLite database connections.
func (e *RAGEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.dbPool != nil {
		err := e.dbPool.Close()
		e.dbPool = nil
		return err
	}
	return nil
}

// ResolveDBPath determines the active Paperless SQLite database file location.
// Checks candidate paths in /data/documents/data/db.sqlite3 and /data/state/services/paperless/db.sqlite3.
func (e *RAGEngine) ResolveDBPath() string {
	e.mu.RLock()
	explicit := e.dbPath
	dataDir := e.dataDir
	e.mu.RUnlock()

	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return explicit
		}
		return ""
	}

	candidates := []string{
		filepath.Join(dataDir, "documents", "data", "db.sqlite3"),
		filepath.Join(dataDir, "state", "services", "paperless", "db.sqlite3"),
	}

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}

	return ""
}

// getDB returns an open, configured sql.DB connection or opens one with WAL mode and busy timeout.
func (e *RAGEngine) getDB(ctx context.Context) (*sql.DB, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	dbPath := e.dbPath
	if dbPath == "" {
		candidates := []string{
			filepath.Join(e.dataDir, "documents", "data", "db.sqlite3"),
			filepath.Join(e.dataDir, "state", "services", "paperless", "db.sqlite3"),
		}
		for _, p := range candidates {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				dbPath = p
				break
			}
		}
	}

	if dbPath == "" {
		return nil, "", nil
	}

	if _, err := os.Stat(dbPath); err != nil {
		return nil, "", nil
	}

	if e.dbPool != nil {
		if err := e.dbPool.PingContext(ctx); err == nil {
			return e.dbPool, dbPath, nil
		} else if ctx.Err() != nil {
			// Do not destroy the shared connection pool if ping failed due to caller context cancellation/deadline
			return nil, dbPath, ctx.Err()
		}
		_ = e.dbPool.Close()
		e.dbPool = nil
	}

	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath, separator)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, dbPath, fmt.Errorf("failed to open paperless sqlite database: %w", err)
	}

	e.dbPool = db
	return db, dbPath, nil
}

// EnsureFTS5 checks if documents_document exists and initializes the documents_fts virtual table with triggers.
func (e *RAGEngine) EnsureFTS5(ctx context.Context, db *sql.DB) error {
	// 1. Check if documents_document exists
	var tableCount int
	err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='documents_document';").Scan(&tableCount)
	if err != nil || tableCount == 0 {
		return nil
	}

	// 2. Check if documents_fts virtual table exists
	var ftsCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='documents_fts';").Scan(&ftsCount)
	if err == nil && ftsCount > 0 {
		return nil
	}

	// 3. Create documents_fts external content table pointing to documents_document
	createFTS := `CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
		title,
		content,
		content='documents_document',
		content_rowid='id',
		tokenize='porter unicode61'
	);`
	if _, err := db.ExecContext(ctx, createFTS); err != nil {
		e.logger.Warn("unable to create documents_fts virtual table, falling back to direct search", "error", err)
		return err
	}

	// 4. Create sync triggers and rebuild index
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents_document BEGIN
			INSERT INTO documents_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents_document BEGIN
			INSERT INTO documents_fts(documents_fts, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents_document BEGIN
			INSERT INTO documents_fts(documents_fts, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
			INSERT INTO documents_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
		END;`,
		`INSERT INTO documents_fts(documents_fts) VALUES('rebuild');`,
	}

	for _, trig := range triggers {
		if _, err := db.ExecContext(ctx, trig); err != nil {
			e.logger.Warn("failed to apply fts trigger/rebuild", "error", err)
		}
	}

	return nil
}

var wordTokenRegex = regexp.MustCompile(`[\p{L}\p{N}_-]+`)

// SanitizeFTSQuery converts arbitrary natural language input into a safe FTS5 query.
func SanitizeFTSQuery(raw string) string {
	matches := wordTokenRegex.FindAllString(raw, -1)
	if len(matches) == 0 {
		return ""
	}

	var terms []string
	seen := make(map[string]bool)
	for _, m := range matches {
		cleaned := strings.TrimSpace(m)
		if len(cleaned) < 2 {
			continue
		}
		lower := strings.ToLower(cleaned)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		terms = append(terms, fmt.Sprintf(`"%s"`, strings.ReplaceAll(cleaned, `"`, `""`)))
	}

	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " OR ")
}

var pageHeaderRegex = regexp.MustCompile(`(?im)(?:^|\n)(?:---+\s*page\s*(\d+)\s*---+|\[page\s*(\d+)\]|page\s*(\d+)(?:\s+of\s+\d+)?)(?:\r?\n|$)`)

// CalculatePageNumber determines the 1-indexed page number where matchOffset falls.
// Inspects form feed markers (\x0c) first, then OCR page headers, defaulting to 1.
func CalculatePageNumber(content string, matchOffset int) int {
	if matchOffset <= 0 || len(content) == 0 {
		return 1
	}

	if matchOffset > len(content) {
		matchOffset = len(content)
	}

	prefix := content[:matchOffset]

	// 1. Check for standard form feed page break characters (\x0c / \f)
	if strings.Contains(content, "\x0c") {
		return 1 + strings.Count(prefix, "\x0c")
	}

	// 2. Check for OCR page header patterns
	matches := pageHeaderRegex.FindAllStringSubmatchIndex(content, -1)
	if len(matches) > 0 {
		currentPage := 1
		for _, m := range matches {
			if m[0] <= matchOffset {
				for i := 1; i < len(m)/2; i++ {
					start := m[2*i]
					end := m[2*i+1]
					if start >= 0 && end > start {
						if p, err := strconv.Atoi(content[start:end]); err == nil && p > 0 {
							currentPage = p
							break
						}
					}
				}
			} else {
				break
			}
		}
		return currentPage
	}

	return 1
}

// ExtractSnippet generates a readable ~250 character context excerpt around the match offset.
func ExtractSnippet(content string, matchOffset int, termLen int) string {
	if len(content) == 0 {
		return ""
	}
	if matchOffset < 0 || matchOffset >= len(content) {
		matchOffset = 0
	}

	runes := []rune(content)
	// Find rune index for byte offset
	bytePos := 0
	runePos := 0
	for i, r := range runes {
		if bytePos >= matchOffset {
			runePos = i
			break
		}
		bytePos += len(string(r))
	}

	windowBefore := 70
	windowAfter := 180

	start := runePos - windowBefore
	if start < 0 {
		start = 0
	}
	end := runePos + termLen + windowAfter
	if end > len(runes) {
		end = len(runes)
	}

	snippetRunes := runes[start:end]
	snippet := string(snippetRunes)

	// Clean up newlines and excessive whitespace
	snippet = strings.ReplaceAll(snippet, "\x0c", " ")
	snippet = strings.Join(strings.Fields(snippet), " ")

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet = snippet + "..."
	}

	return snippet
}

// Search searches the Paperless SQLite database using FTS5 with BM25 ranking.
func (e *RAGEngine) Search(ctx context.Context, req RAGSearchRequest) (*RAGSearchResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	limit := req.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	normLang := NormalizeLanguage(req.Language)

	db, dbPath, err := e.getDB(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		e.logger.Error("failed to connect to paperless database", "error", err)
		return nil, fmt.Errorf("paperless database error: %w", err)
	}

	// If database is unprovisioned or file doesn't exist
	if db == nil || dbPath == "" {
		return &RAGSearchResponse{
			Query:        req.Query,
			TotalMatches: 0,
			Results:      []RAGSearchResult{},
			Notice:       GetUnprovisionedDBNotice(normLang),
		}, nil
	}

	// Verify documents_document table exists
	var tableCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='documents_document';").Scan(&tableCount); err != nil || tableCount == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return &RAGSearchResponse{
			Query:        req.Query,
			TotalMatches: 0,
			Results:      []RAGSearchResult{},
			Notice:       GetUnprovisionedDBNotice(normLang),
		}, nil
	}

	// Try ensuring FTS5 table
	_ = e.EnsureFTS5(ctx, db)

	ftsQuery := SanitizeFTSQuery(req.Query)
	if ftsQuery == "" {
		return &RAGSearchResponse{
			Query:        req.Query,
			TotalMatches: 0,
			Results:      []RAGSearchResult{},
			Notice:       "",
		}, nil
	}

	var results []RAGSearchResult

	// 1. Try querying via documents_fts with BM25 rank
	ftsSearchSQL := `SELECT d.id, d.title, d.content, rank
		FROM documents_fts f
		JOIN documents_document d ON d.id = f.rowid
		WHERE documents_fts MATCH ?
		ORDER BY rank
		LIMIT ?;`

	rows, queryErr := db.QueryContext(ctx, ftsSearchSQL, ftsQuery, limit)
	if queryErr == nil {
		defer rows.Close()
		for rows.Next() {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var docID int64
			var title, content string
			var score float64
			if err := rows.Scan(&docID, &title, &content, &score); err != nil {
				continue
			}

			offset := findBestMatchOffset(content, req.Query)
			page := CalculatePageNumber(content, offset)
			snippet := ExtractSnippet(content, offset, 15)

			url := fmt.Sprintf("/docs/documents/%d", docID)
			if page > 1 {
				url = fmt.Sprintf("/docs/documents/%d#page=%d", docID, page)
			}

			results = append(results, RAGSearchResult{
				DocumentID: docID,
				Title:      title,
				PageNumber: page,
				Snippet:    snippet,
				Score:      score,
				URL:        url,
			})
		}
		if err := rows.Err(); err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	} else {
		// 2. Fallback to LIKE query on documents_document if FTS table is not present
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		e.logger.Warn("FTS5 query failed, executing fallback query", "error", queryErr)
		words := wordTokenRegex.FindAllString(req.Query, -1)
		var significantWords []string
		stopWords := map[string]bool{
			"what": true, "when": true, "where": true, "which": true, "who": true, "whom": true,
			"this": true, "that": true, "these": true, "those": true, "with": true, "from": true,
			"have": true, "has": true, "had": true, "does": true, "doing": true, "done": true,
			"the": true, "a": true, "an": true, "and": true, "or": true, "but": true, "our": true,
			"der": true, "die": true, "das": true, "und": true, "oder": true, "ist": true, "sind": true,
			"para": true, "pour": true, "dans": true,
		}
		for _, w := range words {
			lower := strings.ToLower(w)
			if len(w) >= 3 && !stopWords[lower] {
				significantWords = append(significantWords, w)
			}
		}
		if len(significantWords) == 0 && len(words) > 0 {
			longest := words[0]
			for _, w := range words {
				if len(w) > len(longest) {
					longest = w
				}
			}
			significantWords = append(significantWords, longest)
		}

		if len(significantWords) > 0 {
			searchTerm := "%" + significantWords[0] + "%"
			likeSQL := `SELECT id, title, content FROM documents_document WHERE content LIKE ? OR title LIKE ? LIMIT ?;`
			fallbackRows, fbErr := db.QueryContext(ctx, likeSQL, searchTerm, searchTerm, limit)
			if fbErr == nil {
				defer fallbackRows.Close()
				for fallbackRows.Next() {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					var docID int64
					var title, content string
					if err := fallbackRows.Scan(&docID, &title, &content); err != nil {
						continue
					}
					offset := findBestMatchOffset(content, req.Query)
					page := CalculatePageNumber(content, offset)
					snippet := ExtractSnippet(content, offset, 15)

					url := fmt.Sprintf("/docs/documents/%d", docID)
					if page > 1 {
						url = fmt.Sprintf("/docs/documents/%d#page=%d", docID, page)
					}

					results = append(results, RAGSearchResult{
						DocumentID: docID,
						Title:      title,
						PageNumber: page,
						Snippet:    snippet,
						Score:      -1.0,
						URL:        url,
					})
				}
				if err := fallbackRows.Err(); err != nil && ctx.Err() != nil {
					return nil, ctx.Err()
				}
			}
		}
	}

	notice := ""
	if len(results) == 0 {
		notice = GetNoMatchingDocumentsMessage(normLang)
	}

	return &RAGSearchResponse{
		Query:        req.Query,
		TotalMatches: len(results),
		Results:      results,
		Notice:       notice,
	}, nil
}

// findBestMatchOffset returns the character offset of the first matching query word in content.
func findBestMatchOffset(content, query string) int {
	contentLower := strings.ToLower(content)
	words := wordTokenRegex.FindAllString(strings.ToLower(query), -1)

	// Try multi-word exact match first
	if len(words) >= 2 {
		phrase := strings.Join(words, " ")
		if idx := strings.Index(contentLower, phrase); idx >= 0 {
			return idx
		}
	}

	// Try individual words, preferring longer words
	for _, w := range words {
		if len(w) > 3 {
			if idx := strings.Index(contentLower, w); idx >= 0 {
				return idx
			}
		}
	}

	for _, w := range words {
		if idx := strings.Index(contentLower, w); idx >= 0 {
			return idx
		}
	}

	return 0
}

// Query executes a RAG workflow: searches documents, applies HeadroomTruncator,
// constructs a grounded prompt with exact page citations, and invokes inference.
func (e *RAGEngine) Query(ctx context.Context, req RAGQueryRequest) (*RAGQueryResponse, error) {
	normLang := NormalizeLanguage(req.Language)

	// 1. Search document archive
	searchResp, err := e.Search(ctx, RAGSearchRequest{
		Query:    req.Query,
		Limit:    10,
		Language: normLang,
	})
	if err != nil {
		return nil, fmt.Errorf("document search failed: %w", err)
	}

	// Check if unprovisioned database
	if searchResp.Notice == GetUnprovisionedDBNotice(normLang) {
		return &RAGQueryResponse{
			Answer:    GetNoMatchingDocumentsMessage(normLang),
			Citations: []RAGSearchResult{},
			Truncated: false,
			Notice:    searchResp.Notice,
		}, nil
	}

	// If no matching documents found
	if len(searchResp.Results) == 0 {
		return &RAGQueryResponse{
			Answer:    GetNoMatchingDocumentsMessage(normLang),
			Citations: []RAGSearchResult{},
			Truncated: false,
			Notice:    "",
		}, nil
	}

	// 2. Prepare context capacity and headroom truncator
	capacity := req.ContextTokens
	if capacity <= 0 {
		capacity = DefaultContextCapacity
	}

	reserved := DefaultReservedResponseTokens
	if req.MaxTokens > reserved {
		reserved = req.MaxTokens
	}

	systemPrompt := GetRAGSystemPrompt(normLang)
	truncator := e.truncator
	if truncator == nil {
		truncator = NewHeadroomTruncator(capacity, reserved, EstimateTokens)
	} else {
		truncator = NewHeadroomTruncator(capacity, reserved, truncator.TokenEstimator)
	}

	// 3. Truncate context to fit headroom
	truncResult := truncator.Truncate(systemPrompt, req.Query, searchResp.Results, normLang)

	// Format synthesized context for LLM prompt
	var contextBuilder strings.Builder
	for _, chunk := range truncResult.RetainedChunks {
		contextBuilder.WriteString(FormatExcerptChunk(chunk))
	}

	fullSystemPrompt := fmt.Sprintf("%s\n\nHousehold Document Excerpts:\n%s", systemPrompt, contextBuilder.String())

	// 4. Execute inference
	e.mu.RLock()
	client := e.inferenceClient
	e.mu.RUnlock()

	if client == nil {
		return nil, errors.New("no inference client configured for RAG engine")
	}

	model := req.Model
	if model == "" {
		model = "default"
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultReservedResponseTokens
	}

	chatReq := ChatCompletionRequest{
		Model: model,
		Messages: []ChatCompletionMessage{
			{Role: "system", Content: fullSystemPrompt},
			{Role: "user", Content: req.Query},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.1, // Precise & factual for RAG document analysis
		TopP:        0.9,
	}

	chatResp, err := client.Complete(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("rag inference failed: %w", err)
	}

	if chatResp == nil || len(chatResp.Choices) == 0 {
		return nil, errors.New("ai model returned empty completion choices")
	}

	answer := strings.TrimSpace(chatResp.Choices[0].Message.Content)

	// 5. Append localized truncation notice if context was truncated
	if truncResult.Truncated && truncResult.Notice != "" {
		if !strings.Contains(answer, truncResult.Notice) {
			answer = answer + "\n\n" + truncResult.Notice
		}
	}

	return &RAGQueryResponse{
		Answer:    answer,
		Citations: truncResult.RetainedChunks,
		Truncated: truncResult.Truncated,
		Notice:    truncResult.Notice,
	}, nil
}
