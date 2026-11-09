package ai

import (
	"fmt"
)

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
