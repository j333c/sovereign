package storage

import (
	"bytes"
	"sovereign/internal/supervisor"
	"strings"
)

// IsValidCategory checks whether a string is one of the valid media categories.
func IsValidCategory(cat string) bool {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case CategoryMovies, CategoryTV, CategoryMusic, CategoryAudiobooks, CategoryDocuments:
		return true
	default:
		return false
	}
}

// IsCategoryCompatible checks whether the detected media MIME type is suitable for the target category.
func IsCategoryCompatible(category, contentType string) bool {
	category = strings.ToLower(strings.TrimSpace(category))
	switch category {
	case CategoryMovies, CategoryTV:
		return strings.HasPrefix(contentType, "video/")
	case CategoryMusic, CategoryAudiobooks:
		return strings.HasPrefix(contentType, "audio/")
	case CategoryDocuments:
		return contentType == "application/pdf" || strings.HasPrefix(contentType, "image/")
	default:
		return false
	}
}

// ServiceForCategory maps a media category to its responsible supervisor service ID.
func ServiceForCategory(cat string) string {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case CategoryMovies, CategoryTV:
		return supervisor.ServiceJellyfin
	case CategoryMusic:
		return supervisor.ServiceNavidrome
	case CategoryAudiobooks:
		return supervisor.ServiceAudiobookshelf
	case CategoryDocuments:
		return supervisor.ServicePaperless
	default:
		return ""
	}
}

// DetectMedia inspects initial buffer bytes for recognized video and audio signatures:
// MP4, MKV, WebM, AVI, MPEG-TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC.
// Returns mime type, format slug, and whether it is a verified media file.
func DetectMedia(data []byte) (contentType string, format string, ok bool) {
	if len(data) < 3 {
		return "", "", false
	}

	// 1. FLAC: starts with "fLaC"
	if bytes.HasPrefix(data, []byte("fLaC")) {
		return "audio/flac", "flac", true
	}

	// 2. OGG container (OGG Vorbis, Opus, Theora)
	if bytes.HasPrefix(data, []byte("OggS")) {
		limit := len(data)
		if limit > 64 {
			limit = 64
		}
		sub := data[:limit]
		if bytes.Contains(sub, []byte("OpusHead")) {
			return "audio/opus", "opus", true
		}
		if bytes.Contains(sub, []byte("theora")) {
			return "video/ogg", "ogg", true
		}
		return "audio/ogg", "ogg", true
	}

	// 3. MKV / WebM: EBML header ID 0x1A 0x45 0xDF 0xA3
	if len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		limit := len(data)
		if limit > 64 {
			limit = 64
		}
		if bytes.Contains(data[:limit], []byte("webm")) {
			return "video/webm", "webm", true
		}
		return "video/x-matroska", "mkv", true
	}

	// 4. RIFF formats: WAV and AVI
	if len(data) >= 12 && (bytes.HasPrefix(data, []byte("RIFF")) || bytes.HasPrefix(data, []byte("RIFX"))) {
		formType := string(data[8:12])
		if formType == "WAVE" {
			return "audio/wav", "wav", true
		}
		if formType == "AVI " || formType == "AVIX" {
			return "video/x-msvideo", "avi", true
		}
	}

	// 5. MP4, M4A, M4B (ISO Base Media File Format)
	// Box size at 0..3, box type "ftyp" at 4..7
	if len(data) >= 8 && string(data[4:8]) == "ftyp" {
		var brand string
		if len(data) >= 12 {
			brand = string(data[8:12])
		}
		switch brand {
		case "M4A ", "m4a ":
			return "audio/mp4", "m4a", true
		case "M4B ", "m4b ":
			return "audio/mp4", "m4b", true
		default:
			return "video/mp4", "mp4", true
		}
	}
	// QuickTime files without ftyp (moov, mdat, wide) at offset 4
	if len(data) >= 8 {
		boxType := string(data[4:8])
		if boxType == "moov" || boxType == "mdat" || boxType == "wide" {
			return "video/quicktime", "mp4", true
		}
	}

	// 6. MP3 with ID3v2 tag: starts with "ID3"
	if len(data) >= 3 && data[0] == 'I' && data[1] == 'D' && data[2] == '3' {
		return "audio/mpeg", "mp3", true
	}

	// 7. MP3 raw MPEG audio frame sync & AAC ADTS
	// Top 11 bits set: 0xFF followed by (data[1] & 0xE0) == 0xE0
	if len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
		layer := (data[1] >> 1) & 0x03
		if layer == 0 {
			// AAC ADTS frame: syncword 0xFFF, layer is 00
			if (data[1] & 0xF6) == 0xF0 {
				return "audio/aac", "aac", true
			}
		} else {
			// MPEG Audio Layer I, II, or III
			return "audio/mpeg", "mp3", true
		}
	}

	// 8. AAC ADTS: 0xFF followed by 0xF0..0xF9
	if len(data) >= 2 && data[0] == 0xFF && (data[1]&0xF6) == 0xF0 {
		return "audio/aac", "aac", true
	}

	// 9. MPEG-TS: Transport Stream sync byte 0x47
	// Bounds safe: ensure slice length strictly exceeds offset before indexing
	if data[0] == 0x47 {
		if len(data) > 188 && data[188] == 0x47 {
			return "video/mp2t", "ts", true
		}
		if len(data) > 204 && data[204] == 0x47 {
			return "video/mp2t", "ts", true
		}
		if len(data) >= 4 && !isAllPrintableASCII(data[:4]) {
			return "video/mp2t", "ts", true
		}
	}

	return "", "", false
}

// DetectDocument inspects buffer bytes for document signatures (PDF, PNG, JPEG, TIFF).
func DetectDocument(data []byte) (contentType string, ok bool) {
	if len(data) < 4 {
		return "", false
	}
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return "application/pdf", true
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png", true
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", true
	}
	if len(data) >= 4 {
		if (data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00) ||
			(data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A) {
			return "image/tiff", true
		}
	}
	return "", false
}

func isAllPrintableASCII(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}
