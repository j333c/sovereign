package enrichment

import (
	"context"
	"fmt"
	"sync"
)

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
