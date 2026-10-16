package enrichment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVolumeLabel_Movies(t *testing.T) {
	tests := []struct {
		input     string
		wantType  MediaType
		wantTitle string
		wantYear  int
	}{
		{
			input:     "THE_MATRIX_1999",
			wantType:  MediaTypeMovie,
			wantTitle: "The Matrix",
			wantYear:  1999,
		},
		{
			input:     "The.Matrix.1999",
			wantType:  MediaTypeMovie,
			wantTitle: "The Matrix",
			wantYear:  1999,
		},
		{
			input:     "INCEPTION_2010-8f3a21b4",
			wantType:  MediaTypeMovie,
			wantTitle: "Inception",
			wantYear:  2010,
		},
		{
			input:     "INTERSTELLAR_2014",
			wantType:  MediaTypeMovie,
			wantTitle: "Interstellar",
			wantYear:  2014,
		},
		{
			input:     "PULP_FICTION",
			wantType:  MediaTypeMovie,
			wantTitle: "Pulp Fiction",
			wantYear:  0,
		},
		{
			input:     "ROCKY_IV_1985",
			wantType:  MediaTypeMovie,
			wantTitle: "Rocky IV",
			wantYear:  1985,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseVolumeLabel(tt.input)
			if got.MediaType != tt.wantType {
				t.Errorf("MediaType = %v, want %v", got.MediaType, tt.wantType)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Year != tt.wantYear {
				t.Errorf("Year = %d, want %d", got.Year, tt.wantYear)
			}
		})
	}
}

func TestParseVolumeLabel_TV(t *testing.T) {
	tests := []struct {
		input      string
		wantTitle  string
		wantSeason int
		wantDisc   int
	}{
		{
			input:      "BREAKING_BAD_S01D01",
			wantTitle:  "Breaking Bad",
			wantSeason: 1,
			wantDisc:   1,
		},
		{
			input:      "BREAKING_BAD_S02_DISC1",
			wantTitle:  "Breaking Bad",
			wantSeason: 2,
			wantDisc:   1,
		},
		{
			input:      "GAME_OF_THRONES_S04D02-1234abcd",
			wantTitle:  "Game of Thrones",
			wantSeason: 4,
			wantDisc:   2,
		},
		{
			input:      "S1_DISC1",
			wantTitle:  "Season 1",
			wantSeason: 1,
			wantDisc:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseVolumeLabel(tt.input)
			if got.MediaType != MediaTypeTV {
				t.Errorf("MediaType = %v, want %v", got.MediaType, MediaTypeTV)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Season != tt.wantSeason {
				t.Errorf("Season = %d, want %d", got.Season, tt.wantSeason)
			}
			if got.Disc != tt.wantDisc {
				t.Errorf("Disc = %d, want %d", got.Disc, tt.wantDisc)
			}
		})
	}
}

func TestParseVolumeLabel_Music(t *testing.T) {
	tests := []struct {
		input      string
		wantArtist string
		wantTitle  string
		wantYear   int
	}{
		{
			input:      "AUDIO_CD_8F3A",
			wantArtist: "Unknown Artist",
			wantTitle:  "Audio CD 8F3A",
			wantYear:   0,
		},
		{
			input:      "Pink_Floyd_-_The_Dark_Side_of_the_Moon_1973",
			wantArtist: "Pink Floyd",
			wantTitle:  "The Dark Side of the Moon",
			wantYear:   1973,
		},
		{
			input:      "Miles_Davis_-_Kind_of_Blue",
			wantArtist: "Miles Davis",
			wantTitle:  "Kind of Blue",
			wantYear:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseVolumeLabel(tt.input)
			if got.MediaType != MediaTypeMusic {
				t.Errorf("MediaType = %v, want %v", got.MediaType, MediaTypeMusic)
			}
			if got.Artist != tt.wantArtist {
				t.Errorf("Artist = %q, want %q", got.Artist, tt.wantArtist)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Year != tt.wantYear {
				t.Errorf("Year = %d, want %d", got.Year, tt.wantYear)
			}
		})
	}
}

func TestParseStagedDisc_FileInspection(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Audio CD with CDDA tracks
	cdDir := filepath.Join(tmpDir, "AUDIO_CD_8F3A")
	if err := os.MkdirAll(cdDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(cdDir, "track01.cdda"), []byte("cdda-data"), 0644)
	_ = os.WriteFile(filepath.Join(cdDir, "track02.cdda"), []byte("cdda-data"), 0644)

	info, err := ParseStagedDisc(cdDir, "")
	if err != nil {
		t.Fatalf("ParseStagedDisc failed: %v", err)
	}
	if info.MediaType != MediaTypeMusic {
		t.Errorf("expected MediaTypeMusic, got %v", info.MediaType)
	}
	if info.Artist != "Unknown Artist" {
		t.Errorf("expected Unknown Artist, got %q", info.Artist)
	}

	// 2. Movie disc with disc.iso
	movieDir := filepath.Join(tmpDir, "THE_MATRIX_1999-a1b2c3d4")
	if err := os.MkdirAll(movieDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(movieDir, "disc.iso"), []byte("iso-data"), 0644)

	info, err = ParseStagedDisc(movieDir, "")
	if err != nil {
		t.Fatalf("ParseStagedDisc failed: %v", err)
	}
	if info.MediaType != MediaTypeMovie {
		t.Errorf("expected MediaTypeMovie, got %v", info.MediaType)
	}
	if info.Title != "The Matrix" {
		t.Errorf("expected 'The Matrix', got %q", info.Title)
	}
	if info.Year != 1999 {
		t.Errorf("expected 1999, got %d", info.Year)
	}
}
