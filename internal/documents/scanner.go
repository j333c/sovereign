package documents

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sovereign/internal/store"
)

const (
	EventDeadlineDetected = "dms.deadline_detected"
	DefaultAlertThreshold = 14 // days
)

// QueuedNightlyDoc holds documents waiting for the 01:00-07:00 nighttime LLM extraction pass.
type QueuedNightlyDoc struct {
	DocID    string
	Title    string
	Text     string
	QueuedAt time.Time
}

// ObligationScanner coordinates Tier 1 regex and Tier 2 off-peak LLM extraction.
type ObligationScanner struct {
	store          *store.DeadlineStore
	regexParser    *RegexParser
	llmExtractor   LLMExtractor
	broadcaster    func(event string, data any)
	logger         *slog.Logger
	alertThreshold int // days, default 14

	nightlyMu    sync.Mutex
	nightlyQueue []QueuedNightlyDoc

	nowFunc func() time.Time

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewObligationScanner creates a new scanner coordinator.
func NewObligationScanner(
	dStore *store.DeadlineStore,
	llm LLMExtractor,
	broadcaster func(event string, data any),
	logger *slog.Logger,
) *ObligationScanner {
	if logger == nil {
		logger = slog.Default()
	}

	return &ObligationScanner{
		store:          dStore,
		regexParser:    NewRegexParser(),
		llmExtractor:   llm,
		broadcaster:    broadcaster,
		logger:         logger,
		alertThreshold: DefaultAlertThreshold,
		nightlyQueue:   make([]QueuedNightlyDoc, 0),
		nowFunc:        time.Now,
		stopCh:         make(chan struct{}),
	}
}

// SetNowFunc allows overriding the current time function for deterministic testing.
func (s *ObligationScanner) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
}

// SetAlertThreshold configures the alert threshold in days (default: 14).
func (s *ObligationScanner) SetAlertThreshold(days int) {
	if days > 0 {
		s.alertThreshold = days
	}
}

// AlertThreshold returns the active alert threshold in days.
func (s *ObligationScanner) AlertThreshold() int {
	return s.alertThreshold
}

// IsNighttime returns true exclusively between 01:00 and 07:00 local time.
func (s *ObligationScanner) IsNighttime(t time.Time) bool {
	hr := t.Hour()
	return hr >= 1 && hr < 7
}

// ScanDocumentText executes deadline extraction on document text.
// Tier 1 deterministic regex parsing runs immediately.
// Tier 2 LLM extraction runs only if forceLLM is true or currently in nighttime window (01:00-07:00).
func (s *ObligationScanner) ScanDocumentText(ctx context.Context, docID, title, text string, forceLLM bool) ([]*store.DocumentDeadline, error) {
	if strings.TrimSpace(docID) == "" {
		docID = generateDocUUID()
	}
	if strings.TrimSpace(title) == "" {
		title = "Document"
	}

	now := s.nowFunc()
	var obligations []ExtractedObligation

	// 1. Tier 1: Real-time deterministic regex extraction (authoritative baseline)
	regexResults := s.regexParser.ParseObligations(text, title)
	obligations = append(obligations, regexResults...)

	// 2. Check if Tier 2 LLM extraction should execute
	shouldRunLLM := (forceLLM || s.IsNighttime(now)) && s.llmExtractor != nil

	if shouldRunLLM {
		llmResults, err := s.llmExtractor.ExtractObligations(ctx, text, title)
		if err != nil {
			s.logger.Debug("Tier 2 LLM extraction failed, retaining Tier 1 regex baseline", "error", err)
		} else if len(llmResults) > 0 {
			// Merge LLM obligations with regex obligations, giving precedence to LLM refinements
			obligations = s.mergeObligations(regexResults, llmResults)
		}
	} else if !forceLLM && !s.IsNighttime(now) && s.llmExtractor != nil {
		// Daytime ingestion (07:00 - 01:00): queue for off-peak nighttime window
		s.queueForNightly(docID, title, text)
	}

	// 3. Process, deduplicate, and persist detected obligations in SQLite
	var persisted []*store.DocumentDeadline
	for _, obl := range obligations {
		diffDays, err := calendarDaysDiff(obl.DueDate, now)
		if err != nil {
			continue
		}

		// Ignore past dates older than 30 days
		if diffDays < -30 {
			s.logger.Debug("ignoring past deadline older than 30 days", "due_date", obl.DueDate, "diff_days", diffDays)
			continue
		}

		status := store.DeadlineStatusPending
		if diffDays > s.alertThreshold {
			status = store.DeadlineStatusFuture
		}

		var prior *store.DocumentDeadline
		if s.store != nil {
			prior, _ = s.store.GetByDocAndDueDate(ctx, docID, obl.DueDate)
		}

		rec := &store.DocumentDeadline{
			DocumentID: docID,
			Title:      obl.Title,
			DueDate:    obl.DueDate,
			Amount:     obl.Amount,
			Currency:   obl.Currency,
			Snippet:    obl.Snippet,
			Status:     status,
		}

		if s.store != nil {
			if err := s.store.Save(ctx, rec); err != nil {
				s.logger.Error("failed to persist document deadline", "error", err)
				continue
			}
		}

		// Broadcast SSE event for newly impending deadlines (new record or promoted from future)
		isNewOrPromoted := (prior == nil) || (prior.Status == store.DeadlineStatusFuture && rec.Status == store.DeadlineStatusPending)
		if rec.Status == store.DeadlineStatusPending && isNewOrPromoted && s.broadcaster != nil {
			s.broadcaster(EventDeadlineDetected, map[string]any{
				"id":             rec.ID,
				"document_id":    rec.DocumentID,
				"title":          rec.Title,
				"due_date":       rec.DueDate,
				"amount":         rec.Amount,
				"currency":       rec.Currency,
				"snippet":        rec.Snippet,
				"status":         rec.Status,
				"days_remaining": diffDays,
			})
		}

		persisted = append(persisted, rec)
	}

	return persisted, nil
}

// ScanDocumentFile reads file contents and performs obligation scanning.
func (s *ObligationScanner) ScanDocumentFile(ctx context.Context, docID, title, filePath string, forceLLM bool) ([]*store.DocumentDeadline, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read document file %q: %w", filePath, err)
	}

	text := extractTextFromBytes(content, filepath.Ext(filePath))
	if text == "" {
		text = string(content)
	}

	return s.ScanDocumentText(ctx, docID, title, text, forceLLM)
}

// OnDocumentIngested is called when a document is saved to the consume directory.
// Executes Tier 1 parsing asynchronously to never block document ingestion.
func (s *ObligationScanner) OnDocumentIngested(doc *IngestedDocument) {
	if doc == nil || doc.ConsumePath == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := s.ScanDocumentFile(ctx, doc.ID, doc.Filename, doc.ConsumePath, false)
		if err != nil {
			s.logger.Debug("async document obligation scan completed with error", "id", doc.ID, "error", err)
		}
	}()
}

// ProcessNightlyBatch processes queued documents and updates future deadlines within the 14-day threshold.
func (s *ObligationScanner) ProcessNightlyBatch(ctx context.Context) (int, error) {
	now := s.nowFunc()
	processedCount := 0

	// 1. Process queued daytime documents using Tier 2 LLM pass
	s.nightlyMu.Lock()
	queue := s.nightlyQueue
	s.nightlyQueue = make([]QueuedNightlyDoc, 0)
	s.nightlyMu.Unlock()

	for _, doc := range queue {
		_, err := s.ScanDocumentText(ctx, doc.DocID, doc.Title, doc.Text, true)
		if err != nil {
			s.logger.Error("nightly LLM extraction failed for doc", "doc_id", doc.DocID, "error", err)
		} else {
			processedCount++
		}
	}

	// 2. Review 'future' deadlines in the store to promote to 'pending' if now within threshold
	if s.store != nil {
		futureDeadlines, err := s.store.List(ctx, store.DeadlineStatusFuture, 0, now)
		if err == nil {
			for _, d := range futureDeadlines {
				diffDays, parseErr := calendarDaysDiff(d.DueDate, now)
				if parseErr != nil {
					continue
				}
				if diffDays <= s.alertThreshold && diffDays >= -30 {
					d.Status = store.DeadlineStatusPending
					_ = s.store.Save(ctx, d)

					if s.broadcaster != nil {
						s.broadcaster(EventDeadlineDetected, map[string]any{
							"id":             d.ID,
							"document_id":    d.DocumentID,
							"title":          d.Title,
							"due_date":       d.DueDate,
							"amount":         d.Amount,
							"currency":       d.Currency,
							"snippet":        d.Snippet,
							"status":         d.Status,
							"days_remaining": diffDays,
						})
					}
					processedCount++
				}
			}
		}
	}

	return processedCount, nil
}

// Start begins periodic background checks for the nightly window and threshold transitions.
func (s *ObligationScanner) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		var lastRunNight int = -1

		for {
			select {
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := s.nowFunc()
				if s.IsNighttime(now) {
					day := now.Day()
					if lastRunNight != day {
						lastRunNight = day
						_, _ = s.ProcessNightlyBatch(ctx)
					}
				}
			}
		}
	}()
}

// Stop shuts down the background runner.
func (s *ObligationScanner) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
}

func (s *ObligationScanner) queueForNightly(docID, title, text string) {
	s.nightlyMu.Lock()
	defer s.nightlyMu.Unlock()
	s.nightlyQueue = append(s.nightlyQueue, QueuedNightlyDoc{
		DocID:    docID,
		Title:    title,
		Text:     text,
		QueuedAt: s.nowFunc(),
	})
}

// QueuedCount returns the number of documents waiting for nightly processing.
func (s *ObligationScanner) QueuedCount() int {
	s.nightlyMu.Lock()
	defer s.nightlyMu.Unlock()
	return len(s.nightlyQueue)
}

func (s *ObligationScanner) mergeObligations(regexResults, llmResults []ExtractedObligation) []ExtractedObligation {
	merged := make(map[string]ExtractedObligation)

	for _, r := range regexResults {
		merged[r.DueDate] = r
	}

	for _, l := range llmResults {
		if existing, ok := merged[l.DueDate]; ok {
			// Refine existing regex match with LLM metadata if higher confidence
			if l.Title != "" && l.Title != "Upcoming Obligation" {
				existing.Title = l.Title
			}
			if l.Amount != nil {
				existing.Amount = l.Amount
			}
			if l.Currency != "" {
				existing.Currency = l.Currency
			}
			if l.Snippet != "" {
				existing.Snippet = l.Snippet
			}
			existing.Confidence = 0.99
			merged[l.DueDate] = existing
		} else {
			merged[l.DueDate] = l
		}
	}

	var results []ExtractedObligation
	for _, v := range merged {
		results = append(results, v)
	}
	return results
}

// calendarDaysDiff calculates the calendar day difference between a YYYY-MM-DD due date and now,
// normalizing both to midnight UTC to prevent off-by-one errors from local timezone offsets or DST transitions.
func calendarDaysDiff(dueDateStr string, now time.Time) (int, error) {
	dueDate, err := time.Parse("2006-01-02", dueDateStr)
	if err != nil {
		return 0, err
	}
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(math.Round(dueDate.Sub(nowDate).Hours() / 24.0)), nil
}

// extractTextFromBytes extracts text from plain text or basic PDF text streams.
func extractTextFromBytes(data []byte, ext string) string {
	ext = strings.ToLower(ext)
	if ext == ".txt" || ext == ".text" {
		return string(data)
	}

	if ext == ".pdf" {
		var extracted strings.Builder
		// Simple stream/string extractor looking for parentheses in text blocks: (text) Tj
		inParen := false
		var cur strings.Builder
		for i := 0; i < len(data); i++ {
			b := data[i]
			if b == '(' && (i == 0 || data[i-1] != '\\') {
				inParen = true
				cur.Reset()
			} else if b == ')' && (i == 0 || data[i-1] != '\\') && inParen {
				inParen = false
				s := cur.String()
				if len(s) > 0 {
					extracted.WriteString(s)
					extracted.WriteString(" ")
				}
			} else if inParen {
				cur.WriteByte(b)
			}
		}
		if extracted.Len() > 0 {
			return extracted.String()
		}
	}

	// Fallback to printable ASCII/UTF-8 strings
	var buf bytes.Buffer
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' || b >= 160 {
			buf.WriteByte(b)
		} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != ' ' {
			buf.WriteByte(' ')
		}
	}
	return buf.String()
}
