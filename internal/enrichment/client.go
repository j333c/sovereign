package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var (
	ErrMetadataNotFound = errors.New("metadata not found")
	ErrAPIUnavailable   = errors.New("metadata API unavailable or network offline")
	ErrNoAPIKey         = errors.New("no API key configured")
)

// Standard TMDb Genre ID to name mappings
var tmdbGenreNames = map[int]string{
	28:    "Action",
	12:    "Adventure",
	16:    "Animation",
	35:    "Comedy",
	80:    "Crime",
	99:    "Documentary",
	18:    "Drama",
	10751: "Family",
	14:    "Fantasy",
	36:    "History",
	27:    "Horror",
	10402: "Music",
	9648:  "Mystery",
	10749: "Romance",
	878:   "Science Fiction",
	10770: "TV Movie",
	53:    "Thriller",
	10752: "War",
	37:    "Western",
	// TV specific
	10759: "Action & Adventure",
	10762: "Kids",
	10763: "News",
	10764: "Reality",
	10765: "Sci-Fi & Fantasy",
	10766: "Soap",
	10767: "Talk",
	10768: "War & Politics",
}

func mapTMDbGenres(genreIDs []int, explicitNames []string) []string {
	if len(explicitNames) > 0 {
		return explicitNames
	}
	var genres []string
	for _, id := range genreIDs {
		if name, ok := tmdbGenreNames[id]; ok {
			genres = append(genres, name)
		}
	}
	return genres
}

// MovieMetadata represents enriched metadata for a movie.
type MovieMetadata struct {
	Title       string   `json:"title"`
	Year        int      `json:"year"`
	Overview    string   `json:"overview"`
	Genres      []string `json:"genres"`
	PosterURL   string   `json:"poster_url,omitempty"`
	PosterBytes []byte   `json:"-"`
}

// TVMetadata represents enriched metadata for a TV show or season.
type TVMetadata struct {
	Title       string   `json:"title"`
	Year        int      `json:"year,omitempty"`
	Season      int      `json:"season"`
	Overview    string   `json:"overview"`
	Genres      []string `json:"genres"`
	PosterURL   string   `json:"poster_url,omitempty"`
	PosterBytes []byte   `json:"-"`
}

// TrackMetadata represents a single track on an album.
type TrackMetadata struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// MusicMetadata represents enriched metadata for a music album.
type MusicMetadata struct {
	Artist     string          `json:"artist"`
	Album      string          `json:"album"`
	Year       int             `json:"year,omitempty"`
	Genres     []string        `json:"genres"`
	Tracks     []TrackMetadata `json:"tracks,omitempty"`
	CoverURL   string          `json:"cover_url,omitempty"`
	CoverBytes []byte          `json:"-"`
}

// MetadataClient defines the contract for fetching media metadata and cover art.
type MetadataClient interface {
	FetchMovieMetadata(ctx context.Context, query string, year int) (*MovieMetadata, error)
	FetchTVMetadata(ctx context.Context, query string, season int) (*TVMetadata, error)
	FetchMusicMetadata(ctx context.Context, query string, tocFingerprint string) (*MusicMetadata, error)
	DownloadArtwork(ctx context.Context, artworkURL string) ([]byte, error)
}

// MockMetadataClient provides a deterministic mock implementation for unit and integration testing.
type MockMetadataClient struct {
	mu          sync.Mutex
	MockMovie   *MovieMetadata
	MockTV      *TVMetadata
	MockMusic   *MusicMetadata
	MockArtwork []byte
	MovieErr    error
	TVErr       error
	MusicErr    error
	ArtworkErr  error

	MovieFunc   func(ctx context.Context, query string, year int) (*MovieMetadata, error)
	TVFunc      func(ctx context.Context, query string, season int) (*TVMetadata, error)
	MusicFunc   func(ctx context.Context, query string, tocFingerprint string) (*MusicMetadata, error)
	ArtworkFunc func(ctx context.Context, artworkURL string) ([]byte, error)

	Calls []string
}

// NewMockMetadataClient creates an initialized MockMetadataClient.
func NewMockMetadataClient() *MockMetadataClient {
	return &MockMetadataClient{
		Calls: make([]string, 0),
	}
}

func (m *MockMetadataClient) recordCall(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, name)
}

func (m *MockMetadataClient) FetchMovieMetadata(ctx context.Context, query string, year int) (*MovieMetadata, error) {
	m.recordCall(fmt.Sprintf("FetchMovieMetadata:%s:%d", query, year))
	if m.MovieFunc != nil {
		return m.MovieFunc(ctx, query, year)
	}
	if m.MovieErr != nil {
		return nil, m.MovieErr
	}
	if m.MockMovie != nil {
		copyMeta := *m.MockMovie
		return &copyMeta, nil
	}
	return nil, ErrMetadataNotFound
}

func (m *MockMetadataClient) FetchTVMetadata(ctx context.Context, query string, season int) (*TVMetadata, error) {
	m.recordCall(fmt.Sprintf("FetchTVMetadata:%s:%d", query, season))
	if m.TVFunc != nil {
		return m.TVFunc(ctx, query, season)
	}
	if m.TVErr != nil {
		return nil, m.TVErr
	}
	if m.MockTV != nil {
		copyMeta := *m.MockTV
		return &copyMeta, nil
	}
	return nil, ErrMetadataNotFound
}

func (m *MockMetadataClient) FetchMusicMetadata(ctx context.Context, query string, tocFingerprint string) (*MusicMetadata, error) {
	m.recordCall(fmt.Sprintf("FetchMusicMetadata:%s:%s", query, tocFingerprint))
	if m.MusicFunc != nil {
		return m.MusicFunc(ctx, query, tocFingerprint)
	}
	if m.MusicErr != nil {
		return nil, m.MusicErr
	}
	if m.MockMusic != nil {
		copyMeta := *m.MockMusic
		return &copyMeta, nil
	}
	return nil, ErrMetadataNotFound
}

func (m *MockMetadataClient) DownloadArtwork(ctx context.Context, artworkURL string) ([]byte, error) {
	m.recordCall(fmt.Sprintf("DownloadArtwork:%s", artworkURL))
	if m.ArtworkFunc != nil {
		return m.ArtworkFunc(ctx, artworkURL)
	}
	if m.ArtworkErr != nil {
		return nil, m.ArtworkErr
	}
	if m.MockArtwork != nil {
		b := make([]byte, len(m.MockArtwork))
		copy(b, m.MockArtwork)
		return b, nil
	}
	// Return a minimal valid 1x1 JPEG if no artwork specified
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0xFF, 0xD9}, nil
}

// HTTPMetadataConfig holds configuration for the real third-party API client.
type HTTPMetadataConfig struct {
	TMDbAPIKey         string
	TMDbBaseURL        string
	MusicBrainzBaseURL string
	CoverArtBaseURL    string
	UserAgent          string
	Timeout            time.Duration
}

// HTTPMetadataClient connects to TMDb and MusicBrainz external databases.
type HTTPMetadataClient struct {
	cfg        HTTPMetadataConfig
	httpClient *http.Client
	mbMu       sync.Mutex
	lastMBReq  time.Time
}

// NewHTTPMetadataClient creates an HTTPMetadataClient with bounded timeouts (<= 5s).
func NewHTTPMetadataClient(cfg HTTPMetadataConfig) *HTTPMetadataClient {
	timeout := cfg.Timeout
	if timeout <= 0 || timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	if cfg.TMDbBaseURL == "" {
		cfg.TMDbBaseURL = "https://api.themoviedb.org/3"
	}
	if cfg.MusicBrainzBaseURL == "" {
		cfg.MusicBrainzBaseURL = "https://musicbrainz.org/ws/2"
	}
	if cfg.CoverArtBaseURL == "" {
		cfg.CoverArtBaseURL = "https://coverartarchive.org/release"
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Sovereign/0.1.0 ( support@sovereign.local )"
	}

	return &HTTPMetadataClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// FetchMovieMetadata queries TMDb for movie metadata and poster artwork.
func (c *HTTPMetadataClient) FetchMovieMetadata(ctx context.Context, query string, year int) (*MovieMetadata, error) {
	if c.cfg.TMDbAPIKey == "" {
		return nil, ErrNoAPIKey
	}

	endpoint := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s", c.cfg.TMDbBaseURL, url.QueryEscape(c.cfg.TMDbAPIKey), url.QueryEscape(query))
	if year > 0 {
		endpoint += fmt.Sprintf("&year=%d", year)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create TMDb request: %w", err)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: TMDb request error: %v", ErrAPIUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: TMDb responded with status %d", ErrAPIUnavailable, resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Title        string   `json:"title"`
			ReleaseDate  string   `json:"release_date"`
			Overview     string   `json:"overview"`
			PosterPath   string   `json:"poster_path"`
			GenreIDs     []int    `json:"genre_ids"`
			GenreNames   []string `json:"genre_names"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode TMDb response: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, ErrMetadataNotFound
	}

	top := result.Results[0]
	relYear := year
	if relYear == 0 && len(top.ReleaseDate) >= 4 {
		relYear, _ = strconv.Atoi(top.ReleaseDate[:4])
	}

	posterURL := ""
	if top.PosterPath != "" {
		posterURL = "https://image.tmdb.org/t/p/w500" + top.PosterPath
	}

	genres := mapTMDbGenres(top.GenreIDs, top.GenreNames)

	return &MovieMetadata{
		Title:     top.Title,
		Year:      relYear,
		Overview:  top.Overview,
		Genres:    genres,
		PosterURL: posterURL,
	}, nil
}

// FetchTVMetadata queries TMDb for TV show metadata and poster artwork.
func (c *HTTPMetadataClient) FetchTVMetadata(ctx context.Context, query string, season int) (*TVMetadata, error) {
	if c.cfg.TMDbAPIKey == "" {
		return nil, ErrNoAPIKey
	}

	endpoint := fmt.Sprintf("%s/search/tv?api_key=%s&query=%s", c.cfg.TMDbBaseURL, url.QueryEscape(c.cfg.TMDbAPIKey), url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create TMDb TV request: %w", err)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: TMDb request error: %v", ErrAPIUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: TMDb responded with status %d", ErrAPIUnavailable, resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name         string   `json:"name"`
			FirstAirDate string   `json:"first_air_date"`
			Overview     string   `json:"overview"`
			PosterPath   string   `json:"poster_path"`
			GenreIDs     []int    `json:"genre_ids"`
			GenreNames   []string `json:"genre_names"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode TMDb TV response: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, ErrMetadataNotFound
	}

	top := result.Results[0]
	year := 0
	if len(top.FirstAirDate) >= 4 {
		year, _ = strconv.Atoi(top.FirstAirDate[:4])
	}

	posterURL := ""
	if top.PosterPath != "" {
		posterURL = "https://image.tmdb.org/t/p/w500" + top.PosterPath
	}

	genres := mapTMDbGenres(top.GenreIDs, top.GenreNames)

	return &TVMetadata{
		Title:     top.Name,
		Year:      year,
		Season:    season,
		Overview:  top.Overview,
		Genres:    genres,
		PosterURL: posterURL,
	}, nil
}

// FetchMusicMetadata queries MusicBrainz with non-blocking 1 req/sec rate limiting for album metadata.
func (c *HTTPMetadataClient) FetchMusicMetadata(ctx context.Context, query string, tocFingerprint string) (*MusicMetadata, error) {
	// Enforce 1 req/sec rate limiting for MusicBrainz without blocking other threads under lock
	c.mbMu.Lock()
	now := time.Now()
	var delay time.Duration
	if !c.lastMBReq.IsZero() {
		elapsed := now.Sub(c.lastMBReq)
		if elapsed < 1*time.Second {
			delay = 1*time.Second - elapsed
		}
	}
	c.lastMBReq = now.Add(delay)
	c.mbMu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	searchQuery := fmt.Sprintf("release:%s", query)
	if tocFingerprint != "" {
		searchQuery = fmt.Sprintf("discid:%s", tocFingerprint)
	}

	endpoint := fmt.Sprintf("%s/release/?query=%s&fmt=json", c.cfg.MusicBrainzBaseURL, url.QueryEscape(searchQuery))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MusicBrainz request: %w", err)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: MusicBrainz request error: %v", ErrAPIUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: MusicBrainz responded with status %d", ErrAPIUnavailable, resp.StatusCode)
	}

	var result struct {
		Releases []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			Date         string `json:"date"`
			ArtistCredit []struct {
				Name string `json:"name"`
			} `json:"artist-credit"`
		} `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode MusicBrainz response: %w", err)
	}

	if len(result.Releases) == 0 {
		return nil, ErrMetadataNotFound
	}

	rel := result.Releases[0]
	artist := "Unknown Artist"
	if len(rel.ArtistCredit) > 0 {
		artist = rel.ArtistCredit[0].Name
	}
	year := 0
	if len(rel.Date) >= 4 {
		year, _ = strconv.Atoi(rel.Date[:4])
	}

	coverURL := fmt.Sprintf("%s/%s/front", c.cfg.CoverArtBaseURL, rel.ID)

	var trackList []TrackMetadata
	if rel.ID != "" {
		relDetailURL := fmt.Sprintf("%s/release/%s?inc=recordings&fmt=json", c.cfg.MusicBrainzBaseURL, rel.ID)
		if detailReq, err := http.NewRequestWithContext(ctx, http.MethodGet, relDetailURL, nil); err == nil {
			detailReq.Header.Set("User-Agent", c.cfg.UserAgent)
			detailReq.Header.Set("Accept", "application/json")
			if detailResp, err := c.httpClient.Do(detailReq); err == nil {
				defer detailResp.Body.Close()
				if detailResp.StatusCode == http.StatusOK {
					var detailResult struct {
						Media []struct {
							Tracks []struct {
								Position int    `json:"position"`
								Number   string `json:"number"`
								Title    string `json:"title"`
							} `json:"tracks"`
						} `json:"media"`
					}
					if err := json.NewDecoder(detailResp.Body).Decode(&detailResult); err == nil {
						for _, med := range detailResult.Media {
							for _, trk := range med.Tracks {
								num := trk.Position
								if num == 0 {
									num, _ = strconv.Atoi(trk.Number)
								}
								trackList = append(trackList, TrackMetadata{
									Number: num,
									Title:  trk.Title,
								})
							}
						}
					}
				}
			}
		}
	}

	return &MusicMetadata{
		Artist:   artist,
		Album:    rel.Title,
		Year:     year,
		Tracks:   trackList,
		CoverURL: coverURL,
	}, nil
}

// DownloadArtwork fetches cover art or poster binary content via HTTP with 5s timeout.
func (c *HTTPMetadataClient) DownloadArtwork(ctx context.Context, artworkURL string) ([]byte, error) {
	if artworkURL == "" {
		return nil, fmt.Errorf("empty artwork URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artworkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create artwork request: %w", err)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: artwork download failed: %v", ErrAPIUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: artwork responded with status %d", ErrAPIUnavailable, resp.StatusCode)
	}

	// Read with bounded size limit (max 10MB)
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read artwork bytes: %w", err)
	}

	return data, nil
}
