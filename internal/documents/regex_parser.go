package documents

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExtractedObligation represents an obligation or deadline extracted from document text.
type ExtractedObligation struct {
	DueDate    string   `json:"due_date"` // Normalized ISO date YYYY-MM-DD
	Amount     *float64 `json:"amount,omitempty"`
	Currency   string   `json:"currency,omitempty"`
	Title      string   `json:"title"`
	Snippet    string   `json:"snippet"`
	Confidence float64  `json:"confidence"`
}

// RegexParser executes Tier 1 deterministic parsing on document text.
type RegexParser struct{}

// NewRegexParser creates a new Tier 1 regex parser instance.
func NewRegexParser() *RegexParser {
	return &RegexParser{}
}

// Month maps for multilingual parsing (English and German).
var (
	monthNames = map[string]int{
		// English full and abbreviated
		"january": 1, "jan": 1,
		"february": 2, "feb": 2,
		"march": 3, "mar": 3,
		"april": 4, "apr": 4,
		"may": 5,
		"june": 6, "jun": 6,
		"july": 7, "jul": 7,
		"august": 8, "aug": 8,
		"september": 9, "sep": 9, "sept": 9,
		"october": 10, "oct": 10,
		"november": 11, "nov": 11,
		"december": 12, "dec": 12,

		// German full and abbreviated
		"januar": 1,
		"februar": 2,
		"märz": 3, "maerz": 3, "mär": 3, "mae": 3,
		"juni": 6,
		"juli": 7,
		"oktober": 10, "okt": 10,
		"dezember": 12, "dez": 12,
	}

	// Trigger keywords in English and German
	reTriggers = regexp.MustCompile(`(?i)\b(?:due\s*(?:by|on|date|before)?|total\s+due|payment\s+due|balance\s+due|pay\s+by|expires\s*(?:on)?|expiration\s+date|expiry\s*(?:date)?|renewal\s*(?:date)?|renew\s*by|deadline|valid\s+until|fällig\s*(?:am|bis|zum|keitsdatum)?|zahlbar\s+bis|zu\s+zahlen\s+bis|frist\s*(?:bis)?|kündigungsfrist\s*bis|kündigung\s*bis|vertragsende|ablaufdatum|spätestens\s+bis|verlängerung|gültig\s+bis|until|bis)\b`)

	// Non-due trigger keywords (e.g. invoice date, billing date) to avoid picking invoice date as due date
	reNonDueTriggers = regexp.MustCompile(`(?i)\b(?:invoice\s+date|rechnungsdatum|belegdatum|ausstellungsdatum|date\s+of\s+issue|issued\s+on)\b`)

	// ISO date: YYYY-MM-DD
	reISODate = regexp.MustCompile(`\b(\d{4})-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])\b`)

	// European numerical date: DD.MM.YYYY or DD/MM/YYYY
	reEuroDate = regexp.MustCompile(`\b(0?[1-9]|[12]\d|3[01])[./](0?[1-9]|1[0-2])[./](\d{4})\b`)

	// English textual date: October 25, 2026 or 25 October 2026 or 25th of October 2026
	reEnMonthName = `(?:January|February|March|April|May|June|July|August|September|October|November|December|Jan|Feb|Mar|Apr|Jun|Jul|Aug|Sep|Sept|Oct|Nov|Dec)`
	reEnDate1     = regexp.MustCompile(`(?i)\b(` + reEnMonthName + `)\.?\s+(0?[1-9]|[12]\d|3[01])(?:st|nd|rd|th)?,?\s+(\d{4})\b`)
	reEnDate2     = regexp.MustCompile(`(?i)\b(0?[1-9]|[12]\d|3[01])(?:st|nd|rd|th)?\s+(?:of\s+)?(` + reEnMonthName + `)\.?,?\s+(\d{4})\b`)

	// German textual date: 15. November 2026 or 15. Maerz 2026 or 15. März 2026
	reDeMonthName = `(?:Januar|Februar|März|Maerz|Mär|Mae|April|Mai|Juni|Juli|August|September|Oktober|Okt|November|Nov|Dezember|Dez|Jan|Feb|Apr|Jun|Jul|Aug|Sep)`
	reDeDate      = regexp.MustCompile(`(?i)\b(0?[1-9]|[12]\d|3[01])\.\s*(` + reDeMonthName + `)\.?,?\s+(\d{4})\b`)

	// Monetary amounts
	reAmountPrefixSymbol = regexp.MustCompile(`([$€£])\s*([0-9]{1,3}(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})|[0-9]+)`)
	reAmountSuffixSymbol = regexp.MustCompile(`([0-9]{1,3}(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})|[0-9]+)\s*([$€£])`)
	reAmountSuffixCode   = regexp.MustCompile(`([0-9]{1,3}(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})|[0-9]+)\s*\b(EUR|USD|GBP|CHF)\b`)
	reAmountPrefixCode   = regexp.MustCompile(`\b(USD|EUR|GBP|CHF)\s*([0-9]{1,3}(?:[.,][0-9]{3})*(?:[.,][0-9]{1,2})|[0-9]+)\b`)

	// Specific title patterns
	reTitlePattern = regexp.MustCompile(`(?i)\b((?:Invoice|Rechnung|Police|Policy|Contract|Vertrag)\s*#?\s*[A-Za-z0-9-_]+)\b`)
)

type rawDateMatch struct {
	normalizedDate string
	start          int
	end            int
	hasTrigger     bool
	isNonDue       bool
}

// ParseObligations extracts deadlines and monetary obligations from document text.
func (p *RegexParser) ParseObligations(text string, fallbackTitle string) []ExtractedObligation {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	dateMatches := p.extractValidDates(text)
	if len(dateMatches) == 0 {
		return nil
	}

	// Filter out explicit non-due dates if we have triggered due dates
	var triggerMatches []rawDateMatch
	for _, m := range dateMatches {
		if m.hasTrigger && !m.isNonDue {
			triggerMatches = append(triggerMatches, m)
		}
	}

	chosenMatches := triggerMatches
	if len(chosenMatches) == 0 {
		// If no matches had explicit triggers, use dates that are not marked as non-due dates
		for _, m := range dateMatches {
			if !m.isNonDue {
				chosenMatches = append(chosenMatches, m)
			}
		}
		if len(chosenMatches) == 0 {
			chosenMatches = dateMatches
		}
	}

	// Deduplicate by normalizedDate
	seenDates := make(map[string]bool)
	var uniqueMatches []rawDateMatch
	for _, m := range chosenMatches {
		if !seenDates[m.normalizedDate] {
			seenDates[m.normalizedDate] = true
			uniqueMatches = append(uniqueMatches, m)
		}
	}

	var results []ExtractedObligation
	for _, m := range uniqueMatches {
		snippet := p.extractSnippet(text, m.start, m.end)
		title := p.extractTitle(text, snippet, fallbackTitle)
		amt, curr := p.extractAmount(snippet, text)

		conf := 0.85
		if m.hasTrigger {
			conf = 0.95
		}
		if amt != nil {
			conf = math.Min(1.0, conf+0.04)
		}

		results = append(results, ExtractedObligation{
			DueDate:    m.normalizedDate,
			Amount:     amt,
			Currency:   curr,
			Title:      title,
			Snippet:    snippet,
			Confidence: conf,
		})
	}

	return results
}

// extractValidDates finds and calendar-validates all dates in the text.
func (p *RegexParser) extractValidDates(text string) []rawDateMatch {
	var matches []rawDateMatch

	checkMatch := func(year, month, day, start, end int) {
		norm, ok := ValidateCalendarDate(year, month, day)
		if !ok {
			return
		}

		// Look for nearby trigger keywords (within 100 chars before, 40 chars after)
		ctxStart := start - 100
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxEnd := end + 40
		if ctxEnd > len(text) {
			ctxEnd = len(text)
		}
		surrounding := text[ctxStart:ctxEnd]

		hasTrigger := reTriggers.MatchString(surrounding)
		isNonDue := reNonDueTriggers.MatchString(surrounding)

		matches = append(matches, rawDateMatch{
			normalizedDate: norm,
			start:          start,
			end:            end,
			hasTrigger:     hasTrigger,
			isNonDue:       isNonDue,
		})
	}

	// 1. ISO format: YYYY-MM-DD
	for _, loc := range reISODate.FindAllStringSubmatchIndex(text, -1) {
		y, _ := strconv.Atoi(text[loc[2]:loc[3]])
		m, _ := strconv.Atoi(text[loc[4]:loc[5]])
		d, _ := strconv.Atoi(text[loc[6]:loc[7]])
		checkMatch(y, m, d, loc[0], loc[1])
	}

	// 2. European dot/slash format: DD.MM.YYYY or DD/MM/YYYY
	for _, loc := range reEuroDate.FindAllStringSubmatchIndex(text, -1) {
		d, _ := strconv.Atoi(text[loc[2]:loc[3]])
		m, _ := strconv.Atoi(text[loc[4]:loc[5]])
		y, _ := strconv.Atoi(text[loc[6]:loc[7]])
		checkMatch(y, m, d, loc[0], loc[1])
	}

	// 3. English textual format 1: Month DD, YYYY
	for _, loc := range reEnDate1.FindAllStringSubmatchIndex(text, -1) {
		mStr := strings.ToLower(text[loc[2]:loc[3]])
		d, _ := strconv.Atoi(text[loc[4]:loc[5]])
		y, _ := strconv.Atoi(text[loc[6]:loc[7]])
		if m, ok := monthNames[mStr]; ok {
			checkMatch(y, m, d, loc[0], loc[1])
		}
	}

	// 4. English textual format 2: DD Month YYYY
	for _, loc := range reEnDate2.FindAllStringSubmatchIndex(text, -1) {
		d, _ := strconv.Atoi(text[loc[2]:loc[3]])
		mStr := strings.ToLower(text[loc[4]:loc[5]])
		y, _ := strconv.Atoi(text[loc[6]:loc[7]])
		if m, ok := monthNames[mStr]; ok {
			checkMatch(y, m, d, loc[0], loc[1])
		}
	}

	// 5. German textual format: DD. Month YYYY
	for _, loc := range reDeDate.FindAllStringSubmatchIndex(text, -1) {
		d, _ := strconv.Atoi(text[loc[2]:loc[3]])
		mStr := strings.ToLower(text[loc[4]:loc[5]])
		y, _ := strconv.Atoi(text[loc[6]:loc[7]])
		if m, ok := monthNames[mStr]; ok {
			checkMatch(y, m, d, loc[0], loc[1])
		}
	}

	return matches
}

// ValidateCalendarDate strictly validates that year, month, and day form a real calendar date.
// Prevents invalid dates such as 31.02, 29.02 in non-leap years, etc.
func ValidateCalendarDate(year, month, day int) (string, bool) {
	if year < 1970 || year > 2100 || month < 1 || month > 12 || day < 1 || day > 31 {
		return "", false
	}

	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return "", false
	}

	return fmt.Sprintf("%04d-%02d-%02d", year, month, day), true
}

func (p *RegexParser) extractSnippet(text string, start, end int) string {
	// Look for sentence boundaries (. or \n) before and after
	sStart := strings.LastIndexAny(text[:start], ".\n\r")
	if sStart == -1 || start-sStart > 120 {
		sStart = start - 80
		if sStart < 0 {
			sStart = 0
		}
	} else {
		sStart++ // skip boundary char
	}

	sEnd := strings.IndexAny(text[end:], ".\n\r")
	if sEnd == -1 || sEnd > 120 {
		sEnd = end + 80
		if sEnd > len(text) {
			sEnd = len(text)
		}
	} else {
		sEnd = end + sEnd
	}

	snippet := strings.TrimSpace(text[sStart:sEnd])
	// Collapse multiple whitespaces
	fields := strings.Fields(snippet)
	return strings.Join(fields, " ")
}

func (p *RegexParser) extractTitle(text, snippet, fallback string) string {
	if match := reTitlePattern.FindString(snippet); match != "" {
		return match
	}
	if match := reTitlePattern.FindString(text); match != "" {
		return match
	}
	if fallback != "" {
		// Clean up file extensions if fallback looks like a filename
		cleaned := strings.TrimSuffix(fallback, ".pdf")
		cleaned = strings.TrimSuffix(cleaned, ".png")
		cleaned = strings.TrimSuffix(cleaned, ".jpg")
		cleaned = strings.TrimSuffix(cleaned, ".jpeg")
		return cleaned
	}
	return "Upcoming Obligation"
}

func (p *RegexParser) extractAmount(snippet, fullText string) (*float64, string) {
	// Try snippet first, then fullText
	amt, curr := p.findAmountInString(snippet)
	if amt != nil {
		return amt, curr
	}
	return p.findAmountInString(fullText)
}

func (p *RegexParser) findAmountInString(s string) (*float64, string) {
	// 1. Suffix symbol/code: e.g. 120,00 €, 89,50 EUR, 120.00 CHF, 150.00 USD
	if matches := reAmountSuffixSymbol.FindAllStringSubmatch(s, -1); len(matches) > 0 {
		last := matches[0]
		// If multiple, check if any is near "Gesamt" or "Total"
		for _, m := range matches {
			if strings.Contains(s, "Gesamt") || strings.Contains(s, "Total") {
				last = m
				break
			}
		}
		val, err := parseNumericAmount(last[1])
		if err == nil {
			curr := normalizeCurrency(last[2])
			return &val, curr
		}
	}

	// 2. Suffix code: e.g. 120.00 CHF, 89.50 EUR, 150.00 USD
	if matches := reAmountSuffixCode.FindAllStringSubmatch(s, -1); len(matches) > 0 {
		last := matches[0]
		for _, m := range matches {
			if strings.Contains(s, "Gesamt") || strings.Contains(s, "Total") {
				last = m
				break
			}
		}
		val, err := parseNumericAmount(last[1])
		if err == nil {
			curr := normalizeCurrency(last[2])
			return &val, curr
		}
	}

	// 2. Prefix symbol: e.g. $150.00, €89.50, £120.00
	if matches := reAmountPrefixSymbol.FindAllStringSubmatch(s, -1); len(matches) > 0 {
		last := matches[0]
		val, err := parseNumericAmount(last[2])
		if err == nil {
			curr := normalizeCurrency(last[1])
			return &val, curr
		}
	}

	// 3. Prefix code: e.g. USD 150.00, EUR 89.50, CHF 120.00
	if matches := reAmountPrefixCode.FindAllStringSubmatch(s, -1); len(matches) > 0 {
		last := matches[0]
		val, err := parseNumericAmount(last[2])
		if err == nil {
			curr := normalizeCurrency(last[1])
			return &val, curr
		}
	}

	return nil, ""
}

func normalizeCurrency(curr string) string {
	curr = strings.ToUpper(strings.TrimSpace(curr))
	switch curr {
	case "$", "USD":
		return "USD"
	case "€", "EUR":
		return "EUR"
	case "£", "GBP":
		return "GBP"
	case "CHF":
		return "CHF"
	default:
		return curr
	}
}

func parseNumericAmount(numStr string) (float64, error) {
	s := strings.TrimSpace(numStr)
	// Handle European number formats: 1.250,50 -> 1250.50 or 89,50 -> 89.50
	hasComma := strings.Contains(s, ",")
	hasDot := strings.Contains(s, ".")

	if hasComma && hasDot {
		lastComma := strings.LastIndex(s, ",")
		lastDot := strings.LastIndex(s, ".")
		if lastComma > lastDot {
			// Dot is thousand separator, comma is decimal: 1.250,50
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			// Comma is thousand separator, dot is decimal: 1,250.50
			s = strings.ReplaceAll(s, ",", "")
		}
	} else if hasComma {
		// Only comma, e.g. 89,50 or 120,00
		s = strings.ReplaceAll(s, ",", ".")
	}

	return strconv.ParseFloat(s, 64)
}
