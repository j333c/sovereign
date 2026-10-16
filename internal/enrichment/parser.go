package enrichment

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// MediaType specifies the kind of media identified.
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeTV      MediaType = "tv"
	MediaTypeMusic   MediaType = "music"
	MediaTypeUnknown MediaType = "unknown"
)

// ParsedMediaInfo contains parsed metadata extracted from volume labels, TOCs, and directory structures.
type ParsedMediaInfo struct {
	MediaType MediaType `json:"media_type"`
	Title     string    `json:"title"`
	Year      int       `json:"year,omitempty"`
	Season    int       `json:"season,omitempty"`
	Disc      int       `json:"disc,omitempty"`
	Episode   int       `json:"episode,omitempty"`
	Artist    string    `json:"artist,omitempty"`
	RawLabel  string    `json:"raw_label"`
}

var (
	// Matches trailing 8-hex character suffix from optical disc ID generation (e.g. "-a1b2c3d4")
	hexSuffixRegex = regexp.MustCompile(`-[0-9a-fA-F]{8}$`)

	// Matches TV patterns: SHOW_S01D01, SHOW_S1_DISC1, SHOW.S02E03, etc.
	tvPattern1 = regexp.MustCompile(`(?i)^(.*?)[._\s-]+S(\d{1,2})[._\s-]*(?:D|DISC|E|EP)(\d{1,2})(?:[._\s-].*)?$`)
	// Matches TV pattern: S1_DISC1 or S01D02 (without show name prefix)
	tvPattern2 = regexp.MustCompile(`(?i)^S(\d{1,2})[._\s-]*(?:D|DISC|E|EP)(\d{1,2})(?:[._\s-].*)?$`)
	// Matches TV pattern: SHOW_Season_01_Disc_02
	tvPattern3 = regexp.MustCompile(`(?i)^(.*?)[._\s-]+Season[._\s-]*(\d{1,2})(?:[._\s-]*(?:Disc|D)[._\s-]*(\d{1,2}))?(?:[._\s-].*)?$`)

	// Matches Movie with 4-digit release year: THE_MATRIX_1999, Inception.2010, etc.
	movieYearRegex = regexp.MustCompile(`(?i)^(.*?)[._\s-]+((?:19|20)\d{2})(?:[._\s-].*)?$`)

	// Matches Audio CD labels: AUDIO_CD_8F3A, AUDIO-CD, CD_AUDIO
	audioCDRegex = regexp.MustCompile(`(?i)^(?:AUDIO[._\s-]*CD|CD[._\s-]*AUDIO)(?:[._\s-].*)?$`)
)

// CleanTitle formats an unformatted uppercase/spaced string into proper title case.
func CleanTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Replace underscores and dots with spaces
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, ".", " ")

	// Collapse multiple spaces
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}

	// Minor words in English titles that should remain lowercase unless at the start/end
	minorWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "but": true,
		"or": true, "for": true, "nor": true, "on": true, "at": true,
		"to": true, "from": true, "by": true, "in": true, "of": true,
	}

	// Common acronyms and Roman numerals to keep uppercase
	keepUppercase := map[string]bool{
		"CD": true, "DVD": true, "TV": true, "HD": true, "4K": true, "BD": true,
		"I": true, "II": true, "III": true, "IV": true, "V": true,
		"VI": true, "VII": true, "VIII": true, "IX": true, "X": true,
	}

	for i, w := range words {
		upper := strings.ToUpper(w)
		lower := strings.ToLower(w)

		if keepUppercase[upper] {
			words[i] = upper
			continue
		}

		// If token contains both digits and uppercase letters (e.g. 8F3A), keep it as-is or uppercase
		hasDigit := strings.ContainsAny(w, "0123456789")
		if hasDigit && strings.ToUpper(w) == w {
			words[i] = upper
			continue
		}

		if i > 0 && i < len(words)-1 && minorWords[lower] {
			words[i] = lower
			continue
		}

		// Capitalize first rune, lowercase the rest
		runes := []rune(lower)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}

	return strings.Join(words, " ")
}

// ParseVolumeLabel parses a disc volume label string into ParsedMediaInfo.
func ParseVolumeLabel(label string) ParsedMediaInfo {
	raw := label
	clean := hexSuffixRegex.ReplaceAllString(label, "")

	// 1. Check TV patterns
	if matches := tvPattern1.FindStringSubmatch(clean); len(matches) > 3 {
		title := CleanTitle(matches[1])
		if title == "" {
			title = "TV Show"
		}
		season, _ := strconv.Atoi(matches[2])
		discOrEp, _ := strconv.Atoi(matches[3])
		return ParsedMediaInfo{
			MediaType: MediaTypeTV,
			Title:     title,
			Season:    season,
			Disc:      discOrEp,
			RawLabel:  raw,
		}
	}

	if matches := tvPattern2.FindStringSubmatch(clean); len(matches) > 2 {
		season, _ := strconv.Atoi(matches[1])
		discOrEp, _ := strconv.Atoi(matches[2])
		return ParsedMediaInfo{
			MediaType: MediaTypeTV,
			Title:     fmt.Sprintf("Season %d", season),
			Season:    season,
			Disc:      discOrEp,
			RawLabel:  raw,
		}
	}

	if matches := tvPattern3.FindStringSubmatch(clean); len(matches) > 2 {
		title := CleanTitle(matches[1])
		if title == "" {
			title = "TV Show"
		}
		season, _ := strconv.Atoi(matches[2])
		disc := 1
		if len(matches) > 3 && matches[3] != "" {
			disc, _ = strconv.Atoi(matches[3])
		}
		return ParsedMediaInfo{
			MediaType: MediaTypeTV,
			Title:     title,
			Season:    season,
			Disc:      disc,
			RawLabel:  raw,
		}
	}

	// 2. Check Audio CD patterns
	if audioCDRegex.MatchString(clean) {
		return ParsedMediaInfo{
			MediaType: MediaTypeMusic,
			Title:     CleanTitle(clean),
			Artist:    "Unknown Artist",
			RawLabel:  raw,
		}
	}

	// Check Artist - Album pattern
	if strings.Contains(clean, " - ") || strings.Contains(clean, "_-_") {
		delim := " - "
		if strings.Contains(clean, "_-_") {
			delim = "_-_"
		}
		parts := strings.SplitN(clean, delim, 2)
		artist := CleanTitle(parts[0])
		albumPart := parts[1]
		year := 0
		if yearMatches := movieYearRegex.FindStringSubmatch(albumPart); len(yearMatches) > 2 {
			albumPart = yearMatches[1]
			year, _ = strconv.Atoi(yearMatches[2])
		}
		return ParsedMediaInfo{
			MediaType: MediaTypeMusic,
			Artist:    artist,
			Title:     CleanTitle(albumPart),
			Year:      year,
			RawLabel:  raw,
		}
	}

	// 3. Check Movie with Year pattern
	if matches := movieYearRegex.FindStringSubmatch(clean); len(matches) > 2 {
		title := CleanTitle(matches[1])
		year, _ := strconv.Atoi(matches[2])
		return ParsedMediaInfo{
			MediaType: MediaTypeMovie,
			Title:     title,
			Year:      year,
			RawLabel:  raw,
		}
	}

	// 4. Fallback Movie / Unknown
	return ParsedMediaInfo{
		MediaType: MediaTypeMovie,
		Title:     CleanTitle(clean),
		RawLabel:  raw,
	}
}

// ParseStagedDisc analyzes directory contents, filenames, and label hint to identify media metadata.
func ParseStagedDisc(stagedDirPath string, labelHint string) (ParsedMediaInfo, error) {
	dirName := filepath.Base(stagedDirPath)
	label := labelHint
	if label == "" {
		label = dirName
	}

	parsed := ParseVolumeLabel(label)

	// Inspect staged folder entries
	entries, err := os.ReadDir(stagedDirPath)
	if err != nil {
		return parsed, fmt.Errorf("failed to read staging directory %q: %w", stagedDirPath, err)
	}

	hasCDDA := false
	hasAudio := false
	hasVideo := false

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".cdda":
			hasCDDA = true
			hasAudio = true
		case ".flac", ".wav", ".mp3", ".ogg", ".m4a", ".aac":
			hasAudio = true
		case ".iso", ".mkv", ".mp4", ".avi", ".ts", ".m2ts":
			hasVideo = true
		}
	}

	if hasCDDA || (hasAudio && !hasVideo) {
		parsed.MediaType = MediaTypeMusic
		if parsed.Artist == "" {
			parsed.Artist = "Unknown Artist"
		}
	} else if hasVideo && parsed.MediaType == MediaTypeMusic {
		// Overwrite false music detection if video files are present
		parsed.MediaType = MediaTypeMovie
	}

	return parsed, nil
}
