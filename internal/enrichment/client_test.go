package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMetadataClient_FetchMovieMetadata(t *testing.T) {
	// Test 1: No API key returns ErrNoAPIKey
	clientNoKey := NewHTTPMetadataClient(HTTPMetadataConfig{})
	_, err := clientNoKey.FetchMovieMetadata(context.Background(), "Inception", 2010)
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}

	// Test 2: Successful TMDb query with genre mapping
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.URL.Query().Get("api_key")
		if apiKey != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		query := r.URL.Query().Get("query")
		if query != "Inception" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		resp := map[string]any{
			"page": 1,
			"results": []map[string]any{
				{
					"title":         "Inception",
					"release_date":  "2010-07-16",
					"overview":      "A thief who steals corporate secrets...",
					"poster_path":   "/qmDpIHrmpJINaRKAfWQfftjCdyi.jpg",
					"genre_ids":     []int{28, 878}, // Action, Science Fiction
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tmdbServer.Close()

	client := NewHTTPMetadataClient(HTTPMetadataConfig{
		TMDbAPIKey:  "test-key",
		TMDbBaseURL: tmdbServer.URL,
	})

	meta, err := client.FetchMovieMetadata(context.Background(), "Inception", 2010)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Title != "Inception" {
		t.Errorf("expected title 'Inception', got %q", meta.Title)
	}
	if meta.Year != 2010 {
		t.Errorf("expected year 2010, got %d", meta.Year)
	}
	if len(meta.Genres) != 2 || meta.Genres[0] != "Action" || meta.Genres[1] != "Science Fiction" {
		t.Errorf("expected genres [Action, Science Fiction], got %v", meta.Genres)
	}
	expectedPoster := "https://image.tmdb.org/t/p/w500/qmDpIHrmpJINaRKAfWQfftjCdyi.jpg"
	if meta.PosterURL != expectedPoster {
		t.Errorf("expected poster URL %q, got %q", expectedPoster, meta.PosterURL)
	}

	// Test 3: Not found
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
		})
	}))
	defer emptyServer.Close()

	emptyClient := NewHTTPMetadataClient(HTTPMetadataConfig{
		TMDbAPIKey:  "test-key",
		TMDbBaseURL: emptyServer.URL,
	})

	_, err = emptyClient.FetchMovieMetadata(context.Background(), "Nonexistent Movie", 0)
	if !errors.Is(err, ErrMetadataNotFound) {
		t.Fatalf("expected ErrMetadataNotFound, got %v", err)
	}
}

func TestHTTPMetadataClient_FetchTVMetadata(t *testing.T) {
	// Test 1: No API key
	clientNoKey := NewHTTPMetadataClient(HTTPMetadataConfig{})
	_, err := clientNoKey.FetchTVMetadata(context.Background(), "Breaking Bad", 1)
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}

	// Test 2: Successful TV query with Sci-Fi / Drama genre
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{
					"name":           "Stranger Things",
					"first_air_date": "2016-07-15",
					"overview":       "When a young boy vanishes...",
					"poster_path":    "/49WJfeN0moxb9IPfGn8AIqMGskD.jpg",
					"genre_ids":      []int{18, 10765}, // Drama, Sci-Fi & Fantasy
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tmdbServer.Close()

	client := NewHTTPMetadataClient(HTTPMetadataConfig{
		TMDbAPIKey:  "test-key",
		TMDbBaseURL: tmdbServer.URL,
	})

	tvMeta, err := client.FetchTVMetadata(context.Background(), "Stranger Things", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tvMeta.Title != "Stranger Things" {
		t.Errorf("expected title 'Stranger Things', got %q", tvMeta.Title)
	}
	if tvMeta.Season != 1 {
		t.Errorf("expected season 1, got %d", tvMeta.Season)
	}
	if len(tvMeta.Genres) != 2 || tvMeta.Genres[0] != "Drama" || tvMeta.Genres[1] != "Sci-Fi & Fantasy" {
		t.Errorf("expected [Drama, Sci-Fi & Fantasy], got %v", tvMeta.Genres)
	}
}

func TestHTTPMetadataClient_FetchMusicMetadata(t *testing.T) {
	mbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/release" || r.URL.Path == "/release/" {
			// Search releases
			resp := map[string]any{
				"releases": []map[string]any{
					{
						"id":     "mb-rel-12345",
						"title":  "Dark Side of the Moon",
						"date":   "1973-03-01",
						"artist-credit": []map[string]any{
							{"name": "Pink Floyd"},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/release/mb-rel-12345" {
			// Release recordings detail
			resp := map[string]any{
				"id": "mb-rel-12345",
				"media": []map[string]any{
					{
						"tracks": []map[string]any{
							{"position": 1, "number": "1", "title": "Speak to Me"},
							{"position": 2, "number": "2", "title": "Breathe"},
							{"position": 3, "number": "3", "title": "On the Run"},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		http.NotFound(w, r)
	}))
	defer mbServer.Close()

	client := NewHTTPMetadataClient(HTTPMetadataConfig{
		MusicBrainzBaseURL: mbServer.URL,
		CoverArtBaseURL:    "http://coverart.test",
	})

	meta, err := client.FetchMusicMetadata(context.Background(), "Dark Side of the Moon", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Artist != "Pink Floyd" {
		t.Errorf("expected artist 'Pink Floyd', got %q", meta.Artist)
	}
	if meta.Album != "Dark Side of the Moon" {
		t.Errorf("expected album 'Dark Side of the Moon', got %q", meta.Album)
	}
	if meta.Year != 1973 {
		t.Errorf("expected year 1973, got %d", meta.Year)
	}
	if len(meta.Tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(meta.Tracks))
	}
	if meta.Tracks[0].Number != 1 || meta.Tracks[0].Title != "Speak to Me" {
		t.Errorf("unexpected track 0: %+v", meta.Tracks[0])
	}
	if meta.Tracks[1].Number != 2 || meta.Tracks[1].Title != "Breathe" {
		t.Errorf("unexpected track 1: %+v", meta.Tracks[1])
	}
	if meta.CoverURL != "http://coverart.test/mb-rel-12345/front" {
		t.Errorf("unexpected cover URL: %q", meta.CoverURL)
	}
}

func TestHTTPMetadataClient_MusicBrainzRateLimitContextCancel(t *testing.T) {
	mbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"releases": []any{},
		})
	}))
	defer mbServer.Close()

	client := NewHTTPMetadataClient(HTTPMetadataConfig{
		MusicBrainzBaseURL: mbServer.URL,
	})

	// First request succeeds and sets lastMBReq
	_, _ = client.FetchMusicMetadata(context.Background(), "First Call", "")

	// Second request immediately with an already canceled context should abort cleanly
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.FetchMusicMetadata(cancelCtx, "Second Call", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
