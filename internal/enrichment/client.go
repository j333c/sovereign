package enrichment

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrMetadataNotFound = errors.New("metadata not found")
	ErrAPIUnavailable   = errors.New("metadata API unavailable or network offline")
	ErrNoAPIKey         = errors.New("no API key configured")
	ErrRateLimited      = errors.New("metadata API rate limit exceeded (HTTP 429)")
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

// BackoffConfig configures jittered exponential backoff parameters for external APIs.
type BackoffConfig struct {
	BaseDelay  time.Duration
	Factor     int
	MaxRetries int
	Jitter     time.Duration
	MaxDelay   time.Duration
}

// DefaultBackoffConfig returns the standard parameters: 1s base delay, factor 2, 3 max retries, 250ms jitter, 30s max delay.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		BaseDelay:  1 * time.Second,
		Factor:     2,
		MaxRetries: 3,
		Jitter:     250 * time.Millisecond,
		MaxDelay:   30 * time.Second,
	}
}

// RetryMetrics records aggregate retry telemetry for external metadata API interactions.
// Durations (TotalBackoffDelay, LastBackoffDelay, etc.) serialize to nanoseconds in JSON.
type RetryMetrics struct {
	TotalRetries       uint64        `json:"total_retries"`
	RateLimitRetries   uint64        `json:"rate_limit_retries"`
	NetworkRetries     uint64        `json:"network_retries"`
	TotalBackoffDelay  time.Duration `json:"total_backoff_delay"`
	LastBackoffDelay   time.Duration `json:"last_backoff_delay"`
	TotalJitterOffset  time.Duration `json:"total_jitter_offset"`
	LastJitterOffset   time.Duration `json:"last_jitter_offset"`
	LastRetryTimestamp time.Time     `json:"last_retry_timestamp"`
}

// RetryMetricsProvider represents any metadata client capable of exposing retry telemetry.
type RetryMetricsProvider interface {
	GetRetryMetrics() RetryMetrics
}

// HTTPMetadataConfig holds configuration for the real third-party API client.
type HTTPMetadataConfig struct {
	TMDbAPIKey         string
	TMDbBaseURL        string
	MusicBrainzBaseURL string
	CoverArtBaseURL    string
	UserAgent          string
	Timeout            time.Duration
	Backoff            *BackoffConfig
	Logger             *slog.Logger
}

// HTTPMetadataClient connects to TMDb and MusicBrainz external databases.
type HTTPMetadataClient struct {
	cfg        HTTPMetadataConfig
	httpClient *http.Client
	backoffMu  sync.RWMutex
	backoff    BackoffConfig
	mbMu       sync.Mutex
	lastMBReq  time.Time
	loggerMu   sync.RWMutex
	logger     *slog.Logger
	metricsMu  sync.RWMutex
	metrics    RetryMetrics
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

	bo := DefaultBackoffConfig()
	if cfg.Backoff != nil {
		bo = *cfg.Backoff
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &HTTPMetadataClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		backoff: bo,
		logger:  logger,
	}
}

// SetLogger configures the logger used for structured debug telemetry.
func (c *HTTPMetadataClient) SetLogger(l *slog.Logger) {
	c.loggerMu.Lock()
	defer c.loggerMu.Unlock()
	if l == nil {
		l = slog.Default()
	}
	c.logger = l
}

func (c *HTTPMetadataClient) getLogger() *slog.Logger {
	c.loggerMu.RLock()
	defer c.loggerMu.RUnlock()
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

// GetRetryMetrics returns a copy of current retry telemetry metrics.
func (c *HTTPMetadataClient) GetRetryMetrics() RetryMetrics {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()
	return c.metrics
}

// ResetRetryMetrics clears recorded retry metrics.
func (c *HTTPMetadataClient) ResetRetryMetrics() {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()
	c.metrics = RetryMetrics{}
}

func (c *HTTPMetadataClient) recordRetryTelemetry(reason string, delay time.Duration, jitterOffset time.Duration) {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()

	c.metrics.TotalRetries++
	if reason == "rate_limit" {
		c.metrics.RateLimitRetries++
	} else if reason == "network_error" {
		c.metrics.NetworkRetries++
	}
	c.metrics.TotalBackoffDelay += delay
	c.metrics.LastBackoffDelay = delay
	c.metrics.TotalJitterOffset += jitterOffset
	c.metrics.LastJitterOffset = jitterOffset
	c.metrics.LastRetryTimestamp = time.Now()
}

func sanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	clone := *u
	q := clone.Query()
	if q.Has("api_key") {
		q.Set("api_key", "REDACTED")
		clone.RawQuery = q.Encode()
	}
	return clone.String()
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
			return fmt.Sprintf("%s %q: %v", urlErr.Op, sanitizeURL(parsed), urlErr.Err)
		}
	}
	return err.Error()
}

// SetBackoff overrides the backoff configuration (used for testing).
func (c *HTTPMetadataClient) SetBackoff(b BackoffConfig) {
	c.backoffMu.Lock()
	defer c.backoffMu.Unlock()
	c.backoff = b
}

func (c *HTTPMetadataClient) getBackoff() BackoffConfig {
	c.backoffMu.RLock()
	defer c.backoffMu.RUnlock()
	return c.backoff
}

func (c *HTTPMetadataClient) calculateBackoffDelay(attempt int) (time.Duration, time.Duration) {
	bo := c.getBackoff()
	base := bo.BaseDelay
	if base <= 0 {
		base = 1 * time.Second
	}
	factor := bo.Factor
	if factor <= 0 {
		factor = 2
	}

	mult := 1
	for i := 0; i < attempt; i++ {
		mult *= factor
	}
	delay := base * time.Duration(mult)

	maxDelay := bo.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	if delay > maxDelay {
		delay = maxDelay
	}

	jitter := bo.Jitter
	var offset int64
	if jitter > 0 {
		var b [8]byte
		_, _ = rand.Read(b[:])
		u := binary.LittleEndian.Uint64(b[:])
		jitterRange := uint64(jitter * 2)
		if jitterRange > 0 {
			offset = int64(u%jitterRange) - int64(jitter)
			delay += time.Duration(offset)
		}
	}
	if delay < 0 {
		delay = 0
	}
	return delay, time.Duration(offset)
}

func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var rootErr error = err
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		rootErr = urlErr.Err
	}
	lower := strings.ToLower(rootErr.Error())
	if strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "network is unreachable") ||
		strings.Contains(lower, "no route to host") ||
		strings.Contains(lower, "unexpected eof") {
		return true
	}
	return false
}

func (c *HTTPMetadataClient) doRequestWithBackoff(req *http.Request) (*http.Response, error) {
	bo := c.getBackoff()
	maxRetries := bo.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		reqClone := req.Clone(req.Context())
		resp, err := c.httpClient.Do(reqClone)

		if err != nil {
			if req.Context().Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if isRetryableNetworkError(err) && attempt < maxRetries {
				delay, jitterOffset := c.calculateBackoffDelay(attempt)
				c.getLogger().DebugContext(req.Context(), "metadata api retry backoff",
					slog.String("reason", "network_error"),
					slog.String("method", req.Method),
					slog.String("url", sanitizeURL(req.URL)),
					slog.Int("attempt", attempt+1),
					slog.Int("max_retries", maxRetries),
					slog.Duration("delay", delay),
					slog.Duration("jitter_offset", jitterOffset),
					slog.String("error", sanitizeError(err)),
				)
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(delay):
					c.recordRetryTelemetry("network_error", delay, jitterOffset)
					continue
				}
			}
			return nil, fmt.Errorf("%w: %v", ErrAPIUnavailable, err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			if req.Context().Err() != nil {
				return nil, req.Context().Err()
			}
			if attempt < maxRetries {
				delay, jitterOffset := c.calculateBackoffDelay(attempt)
				c.getLogger().DebugContext(req.Context(), "metadata api retry backoff",
					slog.String("reason", "rate_limit"),
					slog.String("method", req.Method),
					slog.String("url", sanitizeURL(req.URL)),
					slog.Int("attempt", attempt+1),
					slog.Int("max_retries", maxRetries),
					slog.Duration("delay", delay),
					slog.Duration("jitter_offset", jitterOffset),
					slog.Int("status_code", http.StatusTooManyRequests),
				)
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(delay):
					c.recordRetryTelemetry("rate_limit", delay, jitterOffset)
					continue
				}
			}
			return nil, ErrRateLimited
		}

		return resp, nil
	}

	return nil, ErrRateLimited
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

	resp, err := c.doRequestWithBackoff(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, ErrRateLimited) {
			return nil, ErrRateLimited
		}
		return nil, fmt.Errorf("%w: TMDb request error: %w", ErrAPIUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: TMDb responded with status %d", ErrAPIUnavailable, resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Title       string   `json:"title"`
			ReleaseDate string   `json:"release_date"`
			Overview    string   `json:"overview"`
			PosterPath  string   `json:"poster_path"`
			GenreIDs    []int    `json:"genre_ids"`
			GenreNames  []string `json:"genre_names"`
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

	resp, err := c.doRequestWithBackoff(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, ErrRateLimited) {
			return nil, ErrRateLimited
		}
		return nil, fmt.Errorf("%w: TMDb request error: %w", ErrAPIUnavailable, err)
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

	resp, err := c.doRequestWithBackoff(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, ErrRateLimited) {
			return nil, ErrRateLimited
		}
		return nil, fmt.Errorf("%w: MusicBrainz request error: %w", ErrAPIUnavailable, err)
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
			detailResp, err := c.doRequestWithBackoff(detailReq)
			if err != nil {
				if errors.Is(err, ErrRateLimited) {
					return nil, ErrRateLimited
				}
			} else {
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

	resp, err := c.doRequestWithBackoff(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, ErrRateLimited) {
			return nil, ErrRateLimited
		}
		return nil, fmt.Errorf("%w: artwork download failed: %w", ErrAPIUnavailable, err)
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
