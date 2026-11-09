package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AIModelDownloadRequest specifies parameters to initiate a model download.
type AIModelDownloadRequest struct {
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// AIModelDownloadProgress represents live progress and state of a model download job.
type AIModelDownloadProgress struct {
	ID               string    `json:"id"`
	URL              string    `json:"url"`
	Filename         string    `json:"filename"`
	BytesDownloaded  int64     `json:"bytes_downloaded"`
	TotalBytes       int64     `json:"total_bytes"`
	Percent          float64   `json:"percent"`
	SpeedBytesPerSec int64     `json:"speed_bytes_per_sec"`
	ETASeconds       int64     `json:"eta_seconds"`
	Status           string    `json:"status"` // pending, downloading, verifying, completed, cancelled, failed
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	SHA256           string    `json:"sha256,omitempty"`
}

type downloadJob struct {
	progress AIModelDownloadProgress
	cancel   context.CancelFunc
	mu       sync.RWMutex
}

// StartDownload initiates an atomic, resumable model download in the background.
// Enforces pre-flight storage reservation (model size + 5.0 GB safety buffer),
// streams to `<filename>.downloading` using HTTP Range resumption, verifies SHA-256 checksum,
// verifies GGUF magic bytes, and atomically renames the file to `<filename>.gguf`.
func (m *ModelManager) StartDownload(ctx context.Context, req AIModelDownloadRequest) (*AIModelDownloadProgress, error) {
	if req.URL == "" {
		return nil, errors.New("download URL is required")
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, errors.New("invalid download URL scheme (http/https required)")
	}

	filename := req.Filename
	if filename == "" {
		filename = filepath.Base(parsedURL.Path)
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == "/" {
		filename = "custom-model.gguf"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".gguf") {
		filename += ".gguf"
	}

	m.mu.Lock()
	// Check if already downloading this filename
	for _, job := range m.downloads {
		job.mu.RLock()
		st := job.progress.Status
		fn := job.progress.Filename
		job.mu.RUnlock()
		if fn == filename && (st == "downloading" || st == "pending" || st == "verifying") {
			m.mu.Unlock()
			return nil, fmt.Errorf("%w: %s", ErrDownloadInProgress, filename)
		}
	}

	modelsDir := m.modelsDir
	diskFreeFn := m.diskFree
	m.mu.Unlock()

	// 1. Pre-flight storage space probe via HTTP HEAD / Range GET
	contentLength, err := m.probeContentLength(ctx, req.URL)
	if err == nil && contentLength > 0 {
		if err := CheckModelStorageSpaceWithFunc(modelsDir, contentLength, diskFreeFn); err != nil {
			return nil, err
		}
	}

	jobID := fmt.Sprintf("dl-%d", time.Now().UnixNano())
	downloadCtx, cancel := context.WithCancel(context.Background())

	initialProgress := AIModelDownloadProgress{
		ID:              jobID,
		URL:             req.URL,
		Filename:        filename,
		BytesDownloaded: 0,
		TotalBytes:      contentLength,
		Percent:         0.0,
		Status:          "downloading",
		CreatedAt:       time.Now().UTC(),
		SHA256:          req.SHA256,
	}

	job := &downloadJob{
		progress: initialProgress,
		cancel:   cancel,
	}

	m.mu.Lock()
	m.downloads[jobID] = job
	m.mu.Unlock()

	go m.runDownloadWorker(downloadCtx, job)

	return &initialProgress, nil
}

func (m *ModelManager) probeContentLength(ctx context.Context, rawURL string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		return resp.ContentLength, nil
	}

	// Fallback to range probe
	rangeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	rangeReq.Header.Set("Range", "bytes=0-0")
	rangeResp, err := m.httpClient.Do(rangeReq)
	if err != nil {
		return 0, err
	}
	defer rangeResp.Body.Close()

	if rangeResp.StatusCode == http.StatusPartialContent {
		// Extract Content-Range: bytes 0-0/123456
		cr := rangeResp.Header.Get("Content-Range")
		if parts := strings.Split(cr, "/"); len(parts) == 2 {
			if total, parseErr := strconv.ParseInt(parts[1], 10, 64); parseErr == nil {
				return total, nil
			}
		}
	}

	if rangeResp.ContentLength > 0 {
		return rangeResp.ContentLength, nil
	}

	return 0, nil
}

func (m *ModelManager) runDownloadWorker(ctx context.Context, job *downloadJob) {
	job.mu.RLock()
	filename := job.progress.Filename
	rawURL := job.progress.URL
	expectedSHA := strings.ToLower(strings.TrimSpace(job.progress.SHA256))
	job.mu.RUnlock()

	tempFile := filepath.Join(m.modelsDir, filename+".downloading")
	finalFile := filepath.Join(m.modelsDir, filename)

	var startOffset int64 = 0
	if fi, err := os.Stat(tempFile); err == nil {
		startOffset = fi.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		job.mu.Lock()
		job.progress.Status = "failed"
		job.progress.Error = err.Error()
		job.mu.Unlock()
		return
	}

	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		job.mu.Lock()
		if errors.Is(ctx.Err(), context.Canceled) {
			job.progress.Status = "cancelled"
		} else {
			job.progress.Status = "failed"
			job.progress.Error = err.Error()
		}
		job.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		job.mu.Lock()
		job.progress.Status = "failed"
		job.progress.Error = fmt.Sprintf("unexpected HTTP response status: %s", resp.Status)
		job.mu.Unlock()
		return
	}

	var totalBytes int64
	var file *os.File

	if resp.StatusCode == http.StatusPartialContent {
		// Append to existing temp file
		file, err = os.OpenFile(tempFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			job.mu.Lock()
			job.progress.Status = "failed"
			job.progress.Error = err.Error()
			job.mu.Unlock()
			return
		}
		// Content-Range: bytes 500-999/1000
		cr := resp.Header.Get("Content-Range")
		if parts := strings.Split(cr, "/"); len(parts) == 2 {
			if total, pErr := strconv.ParseInt(parts[1], 10, 64); pErr == nil {
				totalBytes = total
			}
		}
		if totalBytes <= 0 && resp.ContentLength > 0 {
			totalBytes = startOffset + resp.ContentLength
		}
	} else {
		// 200 OK: restart from beginning
		startOffset = 0
		file, err = os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			job.mu.Lock()
			job.progress.Status = "failed"
			job.progress.Error = err.Error()
			job.mu.Unlock()
			return
		}
		totalBytes = resp.ContentLength
	}
	defer file.Close()

	job.mu.Lock()
	job.progress.BytesDownloaded = startOffset
	if totalBytes > 0 {
		job.progress.TotalBytes = totalBytes
		job.progress.Percent = math.Round((float64(startOffset)/float64(totalBytes))*1000) / 10
	}
	job.mu.Unlock()

	buf := make([]byte, 64*1024) // 64 KB buffer
	lastTime := time.Now()
	var bytesSinceLastSample int64 = 0

	for {
		select {
		case <-ctx.Done():
			job.mu.Lock()
			job.progress.Status = "cancelled"
			job.mu.Unlock()
			return
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := file.Write(buf[:n]); wErr != nil {
				_ = file.Close()
				_ = os.Remove(tempFile)
				job.mu.Lock()
				job.progress.Status = "failed"
				job.progress.Error = wErr.Error()
				job.mu.Unlock()
				return
			}
			startOffset += int64(n)
			bytesSinceLastSample += int64(n)

			now := time.Now()
			elapsed := now.Sub(lastTime)
			if elapsed >= 500*time.Millisecond {
				speed := int64(float64(bytesSinceLastSample) / elapsed.Seconds())
				var eta int64 = 0
				if speed > 0 && totalBytes > startOffset {
					eta = (totalBytes - startOffset) / speed
				}

				job.mu.Lock()
				job.progress.BytesDownloaded = startOffset
				if totalBytes > 0 {
					job.progress.Percent = math.Round((float64(startOffset)/float64(totalBytes))*1000) / 10
				}
				job.progress.SpeedBytesPerSec = speed
				job.progress.ETASeconds = eta
				job.mu.Unlock()

				bytesSinceLastSample = 0
				lastTime = now
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			job.mu.Lock()
			if errors.Is(ctx.Err(), context.Canceled) {
				job.progress.Status = "cancelled"
			} else {
				job.progress.Status = "failed"
				job.progress.Error = readErr.Error()
			}
			job.mu.Unlock()
			return
		}
	}

	_ = file.Sync()
	_ = file.Close()

	// 2. Verification phase
	job.mu.Lock()
	job.progress.Status = "verifying"
	job.progress.BytesDownloaded = startOffset
	if totalBytes > 0 {
		job.progress.Percent = 100.0
	}
	job.progress.SpeedBytesPerSec = 0
	job.progress.ETASeconds = 0
	job.mu.Unlock()

	// Check SHA-256 if provided
	if expectedSHA != "" {
		f, err := os.Open(tempFile)
		if err != nil {
			job.mu.Lock()
			job.progress.Status = "failed"
			job.progress.Error = err.Error()
			job.mu.Unlock()
			return
		}
		hasher := sha256.New()
		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			job.mu.Lock()
			job.progress.Status = "failed"
			job.progress.Error = err.Error()
			job.mu.Unlock()
			return
		}
		f.Close()

		actualSHA := hex.EncodeToString(hasher.Sum(nil))
		if strings.ToLower(actualSHA) != expectedSHA {
			_ = os.Remove(tempFile)
			job.mu.Lock()
			job.progress.Status = "failed"
			job.progress.Error = fmt.Sprintf("%v: expected %s, got %s", ErrChecksumMismatch, expectedSHA, actualSHA)
			job.mu.Unlock()
			return
		}
	}

	// Verify GGUF header magic bytes
	header, err := ValidateGGUFHeader(tempFile)
	if err != nil {
		_ = os.Remove(tempFile)
		job.mu.Lock()
		job.progress.Status = "failed"
		job.progress.Error = fmt.Sprintf("invalid GGUF header: %v", err)
		job.mu.Unlock()
		return
	}

	// 3. Atomic rename to final destination
	if err := os.Rename(tempFile, finalFile); err != nil {
		_ = os.Remove(tempFile)
		job.mu.Lock()
		job.progress.Status = "failed"
		job.progress.Error = fmt.Sprintf("failed to atomically rename model: %v", err)
		job.mu.Unlock()
		return
	}

	job.mu.Lock()
	job.progress.Status = "completed"
	job.progress.Percent = 100.0
	job.mu.Unlock()

	m.logRecorder.Add(
		time.Now().UTC(),
		"INFO",
		"ai-engine",
		fmt.Sprintf("Downloaded and verified model %s (GGUF v%d, %d tensors)", filename, header.Version, header.TensorCount),
	)
}

// ListDownloads returns all active and historical download jobs.
func (m *ModelManager) ListDownloads() []AIModelDownloadProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	downloads := make([]AIModelDownloadProgress, 0, len(m.downloads))
	for _, job := range m.downloads {
		job.mu.RLock()
		downloads = append(downloads, job.progress)
		job.mu.RUnlock()
	}

	sort.Slice(downloads, func(i, j int) bool {
		return downloads[i].CreatedAt.After(downloads[j].CreatedAt)
	})

	return downloads
}

// CancelDownload terminates an active download by ID.
func (m *ModelManager) CancelDownload(id string) error {
	m.mu.RLock()
	job, ok := m.downloads[id]
	dir := m.modelsDir
	m.mu.RUnlock()

	if !ok {
		return ErrDownloadNotFound
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	if job.progress.Status != "downloading" && job.progress.Status != "pending" && job.progress.Status != "verifying" {
		return errors.New("download job is not active")
	}

	job.progress.Status = "cancelled"
	if job.cancel != nil {
		job.cancel()
	}

	tempFile := filepath.Join(dir, job.progress.Filename+".downloading")
	_ = os.Remove(tempFile)

	return nil
}
