package supervisor

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Default base URLs and networking defaults for Paperless-ngx
const (
	DefaultPaperlessContainerURL = "http://sovereign-paperless:8000"
	DefaultPaperlessLocalURL     = "http://127.0.0.1:8000"
	DefaultPaperlessPort         = 8000
	DefaultPaperlessSubpath      = "/docs"
)

// PaperlessProvisionResult contains output parameters for starting the Paperless-ngx container.
type PaperlessProvisionResult struct {
	ServiceID  string
	Env        map[string]string
	Devices    []string
	OCRWorkers int
}

// PaperlessProvisionerOptions provides dependency overrides for testing.
type PaperlessProvisionerOptions struct {
	NumCPU  func() int
	BaseURL string
}

// PaperlessProvisioner manages storage directory layouts, SQLite WAL pragma durability,
// OCR and task worker concurrency calculations, and environment variable synthesis for Paperless-ngx.
type PaperlessProvisioner struct {
	mu         sync.RWMutex
	dataDir    string
	logger     *slog.Logger
	httpClient *http.Client
	baseURL    string
	numCPU     func() int
}

func resolvePaperlessBaseURL(envKey, containerHostPort, localhostPort string) string {
	if env := os.Getenv(envKey); env != "" {
		return strings.TrimRight(env, "/")
	}
	host := strings.Split(strings.TrimPrefix(containerHostPort, "http://"), ":")[0]
	if _, err := net.LookupHost(host); err == nil {
		return containerHostPort
	}
	return localhostPort
}

// NewPaperlessProvisioner constructs and initializes a new PaperlessProvisioner.
func NewPaperlessProvisioner(dataDir string, logger *slog.Logger, opts ...PaperlessProvisionerOptions) *PaperlessProvisioner {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logger == nil {
		logger = slog.Default()
	}

	numCPUFn := runtime.NumCPU
	baseURL := resolvePaperlessBaseURL("SOVEREIGN_PAPERLESS_URL", DefaultPaperlessContainerURL, DefaultPaperlessLocalURL)

	if len(opts) > 0 {
		if opts[0].NumCPU != nil {
			numCPUFn = opts[0].NumCPU
		}
		if opts[0].BaseURL != "" {
			baseURL = strings.TrimRight(opts[0].BaseURL, "/")
		}
	}

	return &PaperlessProvisioner{
		dataDir:    dataDir,
		logger:     logger,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
		numCPU:     numCPUFn,
	}
}

// SetBaseURL overrides the Paperless HTTP base URL in a thread-safe manner.
func (p *PaperlessProvisioner) SetBaseURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.baseURL = strings.TrimRight(url, "/")
}

// SetHTTPClient overrides the HTTP client in a thread-safe manner.
func (p *PaperlessProvisioner) SetHTTPClient(client *http.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.httpClient = client
}

// SetNumCPUFunc overrides the CPU core detection function for testing.
func (p *PaperlessProvisioner) SetNumCPUFunc(fn func() int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if fn == nil {
		p.numCPU = runtime.NumCPU
	} else {
		p.numCPU = fn
	}
}

func (p *PaperlessProvisioner) resolveBaseURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.baseURL != "" {
		return p.baseURL
	}
	return resolvePaperlessBaseURL("SOVEREIGN_PAPERLESS_URL", DefaultPaperlessContainerURL, DefaultPaperlessLocalURL)
}

// CalculateOCRWorkers calculates the capped OCR and task worker concurrency using max(1, physical_cores / 2).
func CalculateOCRWorkers(numCores int) int {
	if numCores <= 0 {
		numCores = runtime.NumCPU()
	}
	workers := numCores / 2
	if workers < 1 {
		workers = 1
	}
	return workers
}

// EnsureSQLiteWAL ensures that the SQLite database file at dbPath exists and operates in WAL mode
// with NORMAL synchronous pragma, strictly preserving existing tables and document data.
func EnsureSQLiteWAL(dbPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for sqlite db: %w", err)
	}

	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath, separator)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL;").Scan(&mode); err != nil {
		return fmt.Errorf("failed to apply PRAGMA journal_mode = WAL: %w", err)
	}
	if strings.ToLower(mode) != "wal" {
		return fmt.Errorf("expected journal_mode 'wal', got: %s", mode)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = NORMAL;"); err != nil {
		return fmt.Errorf("failed to apply PRAGMA synchronous = NORMAL: %w", err)
	}

	return nil
}

// Provision prepares the storage directory layout with 0755 permissions, verifies SQLite WAL durability,
// calculates capped OCR worker concurrency, and returns container environment variables.
func (p *PaperlessProvisioner) Provision(ctx context.Context) (*PaperlessProvisionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.RLock()
	cpuFn := p.numCPU
	p.mu.RUnlock()

	// 1. Provision storage directories with 0755 permissions, preserving existing files
	requiredDirs := []string{
		filepath.Join(p.dataDir, "documents"),
		filepath.Join(p.dataDir, "documents", "media"),
		filepath.Join(p.dataDir, "documents", "data"),
		filepath.Join(p.dataDir, "documents", "consume"),
		filepath.Join(p.dataDir, "documents", "export"),
		filepath.Join(p.dataDir, "state", "services", "paperless"),
	}

	for _, dir := range requiredDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			p.logger.Error("failed to create paperless storage directory", "dir", dir, "error", err)
			return nil, fmt.Errorf("failed to create paperless directory %s: %w", dir, err)
		}
	}

	// 2. Ensure SQLite database WAL durability at /data/documents/data/db.sqlite3
	dbPath := filepath.Join(p.dataDir, "documents", "data", "db.sqlite3")
	if err := EnsureSQLiteWAL(dbPath); err != nil {
		p.logger.Error("failed to configure SQLite WAL mode for Paperless", "path", dbPath, "error", err)
		return nil, fmt.Errorf("failed to enforce SQLite WAL mode on %s: %w", dbPath, err)
	}

	// 3. Calculate bounded OCR and task worker concurrency: max(1, physical_cores / 2)
	cores := cpuFn()
	workers := CalculateOCRWorkers(cores)

	// 4. Synthesize environment variables
	redisURL := os.Getenv("SOVEREIGN_PAPERLESS_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://sovereign-redis:6379"
	}

	env := map[string]string{
		"PAPERLESS_OCR_WORKERS":        strconv.Itoa(workers),
		"PAPERLESS_TASK_WORKERS":       strconv.Itoa(workers),
		"PAPERLESS_OCR_PROCESSES":      "1",
		"PAPERLESS_THREADS_PER_WORKER": "1",
		"PAPERLESS_FORCE_SCRIPT_NAME":  DefaultPaperlessSubpath,
		"PAPERLESS_URL":                fmt.Sprintf("%s%s", strings.TrimRight(p.resolveBaseURL(), "/"), DefaultPaperlessSubpath),
		"PAPERLESS_DATA_DIR":           "/usr/src/paperless/data",
		"PAPERLESS_MEDIA_ROOT":         "/usr/src/paperless/media",
		"PAPERLESS_CONSUMPTION_DIR":    "/usr/src/paperless/consume",
		"PAPERLESS_EXPORT_DIR":         "/usr/src/paperless/export",
		"PAPERLESS_PORT":               "8000",
		"PAPERLESS_REDIS":              redisURL,
		"USERMAP_UID":                  "1000",
		"USERMAP_GID":                  "1000",
		"PAPERLESS_OCR_LANGUAGES":      "deu eng",
		"PAPERLESS_OCR_LANGUAGE":       "eng",
	}

	p.logger.Info("provisioned paperless-ngx environment successfully",
		"ocr_workers", workers,
		"physical_cores", cores,
		"documents_dir", filepath.Join(p.dataDir, "documents"),
	)

	return &PaperlessProvisionResult{
		ServiceID:  ServicePaperless,
		Env:        env,
		Devices:    nil,
		OCRWorkers: workers,
	}, nil
}
