package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"sovereign/internal/ai"
	"sovereign/internal/auth"
	"sovereign/internal/documents"
	"sovereign/internal/enrichment"
	"sovereign/internal/hardware"
	"sovereign/internal/ingress"
	"sovereign/internal/notify"
	"sovereign/internal/optical"
	"sovereign/internal/storage"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"sovereign/internal/transcode"
)

const (
	AppVersion = "0.1.0"
)

// TranscodeJobResponse matches the OpenAPI 3.1 contract.
type TranscodeJobResponse struct {
	ID           string  `json:"id"`
	SourcePath   string  `json:"source_path"`
	TargetPath   string  `json:"target_path"`
	Profile      string  `json:"profile"`
	Status       string  `json:"status"`
	Progress     float64 `json:"progress"`
	EncoderUsed  string  `json:"encoder_used,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
	CreatedAt    string  `json:"created_at"`
	StartedAt    string  `json:"started_at,omitempty"`
	CompletedAt  string  `json:"completed_at,omitempty"`
}

// TranscodeQueueResponse matches the OpenAPI 3.1 contract.
type TranscodeQueueResponse struct {
	Jobs        []TranscodeJobResponse `json:"jobs"`
	Total       int                    `json:"total"`
	ActiveJobID string                 `json:"active_job_id,omitempty"`
}

// EnqueueTranscodeRequest matches the OpenAPI 3.1 contract.
type EnqueueTranscodeRequest struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path,omitempty"`
	Profile    string `json:"profile,omitempty"`
}

// EnqueueMobileRequest matches the OpenAPI 3.1 contract.
type EnqueueMobileRequest struct {
	SourcePath    string `json:"source_path"`
	TargetProfile string `json:"target_profile"`
	Profile       string `json:"profile,omitempty"`
}

// TranscodeScheduleResponse matches the OpenAPI 3.1 contract.
type TranscodeScheduleResponse struct {
	StartHour      int    `json:"start_hour"`
	StartMinute    int    `json:"start_minute"`
	EndHour        int    `json:"end_hour"`
	EndMinute      int    `json:"end_minute"`
	IsOffPeak      bool   `json:"is_off_peak"`
	ManualOverride bool   `json:"manual_override"`
	Status         string `json:"status"`
}

// UpdateTranscodeScheduleRequest matches the OpenAPI 3.1 contract.
type UpdateTranscodeScheduleRequest struct {
	StartHour   int `json:"start_hour"`
	StartMinute int `json:"start_minute,omitempty"`
	EndHour     int `json:"end_hour"`
	EndMinute   int `json:"end_minute,omitempty"`
}

// TranscodeOverrideRequest matches the OpenAPI 3.1 contract.
type TranscodeOverrideRequest struct {
	Force bool `json:"force"`
}

// TranscodeOverrideResponse matches the OpenAPI 3.1 contract.
type TranscodeOverrideResponse struct {
	Status         string `json:"status"`
	ManualOverride bool   `json:"manual_override"`
	Message        string `json:"message"`
}

// EnrichmentScanResponse matches the OpenAPI 3.1 contract.
type EnrichmentScanResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	ItemsProcessed int    `json:"items_processed"`
}

// OpticalEjectResponse matches the OpenAPI 3.1 contract.
type OpticalEjectResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// OpticalKeysUploadResponse matches the OpenAPI 3.1 contract.
type OpticalKeysUploadResponse struct {
	Status    string `json:"status"`
	KeysCount int    `json:"keys_count"`
	Message   string `json:"message"`
}

// TelemetryUpdate matches the OpenAPI 3.1 real-time telemetry schema.
type TelemetryUpdate struct {
	CPUPercent        float64 `json:"cpu_percent"`
	MemoryUsedBytes   uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes  uint64  `json:"memory_total_bytes"`
	StorageUsedBytes  uint64  `json:"storage_used_bytes"`
	StorageTotalBytes uint64  `json:"storage_total_bytes"`
	StoragePercent    float64 `json:"storage_percent"`
	NASStatus         string  `json:"nas_status"`
	NASMessage        string  `json:"nas_message"`
	OverallStatus     string  `json:"overall_status"`
	OpticalStatus     string  `json:"optical_status"`
	GPUStatus         string  `json:"gpu_status"`
	Timestamp         string  `json:"timestamp"`
}

// ProblemDetails implements RFC 7807 error responses.
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// HealthResponse matches the OpenAPI 3.1 contract.
type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Database string `json:"database"`
}

// IngressRoute matches the OpenAPI 3.1 contract.
type IngressRoute struct {
	Service  string `json:"service"`
	Subpath  string `json:"subpath"`
	Upstream string `json:"upstream"`
}

// IngressStatusResponse matches the OpenAPI 3.1 contract.
type IngressStatusResponse struct {
	Status         string         `json:"status"`
	CaddyConnected bool           `json:"caddy_connected"`
	MDNSActive     bool           `json:"mdns_active"`
	Hostname       string         `json:"hostname"`
	Routes         []IngressRoute `json:"routes"`
}

// Config specifies runtime options for the daemon server.
type Config struct {
	ListenAddr        string
	DataDir           string
	DBPath            string
	StatePath         string
	CaddyURL          string
	Logger            *slog.Logger
	IngressMgr        ingress.IngressManager
	MDNSService       *ingress.MDNSService
	BootMgr           *store.BootManager
	StorageEngine     *storage.Engine
	AuthMgr           *auth.Manager
	HardwareCollector hardware.Collector
	TelemetryInterval time.Duration
	ServiceMgr        *supervisor.ServiceManager
	DropzoneEngine    *storage.DropzoneEngine
	OpticalDriver     optical.OpticalDriver
	OpticalRipper     *optical.OpticalRipper
	KEYDBManager      *optical.KEYDBManager
	EnrichmentEngine  *enrichment.EnrichmentEngine
	MetadataClient    enrichment.MetadataClient
	TranscodeEngine   *transcode.TranscodeEngine
	Transcoder        transcode.Transcoder
	TranscodeStore    transcode.Store
	TranscodeSchedule *transcode.Scheduler
	IMAPStore         *store.IMAPStore
	IMAPPoller        *documents.Poller
	IngestEngine      *documents.IngestEngine
	DeadlineStore     *store.DeadlineStore
	ObligationScanner *documents.ObligationScanner
	AlertDispatcher   *notify.AlertDispatcher
	LlamaProvisioner  *supervisor.LlamaProvisioner
	AIGateway         ai.Gateway
	LlamaURL          string
}

type sseEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// Server is the appliance core supervisor HTTP daemon.
type Server struct {
	cfg               Config
	db                *store.DB
	bootMgr           *store.BootManager
	storageEngine     *storage.Engine
	authMgr           *auth.Manager
	hardwareCollector hardware.Collector
	telemetryInterval time.Duration
	logger            *slog.Logger
	router            *http.ServeMux
	ingressMgr        ingress.IngressManager
	mdnsService       *ingress.MDNSService
	serviceMgr        *supervisor.ServiceManager
	dropzoneEngine    *storage.DropzoneEngine
	opticalRipper     *optical.OpticalRipper
	enrichmentEngine  *enrichment.EnrichmentEngine
	transcodeEngine   *transcode.TranscodeEngine
	imapStore         *store.IMAPStore
	imapPoller        *documents.Poller
	ingestEngine      *documents.IngestEngine
	deadlineStore     *store.DeadlineStore
	obligationScanner *documents.ObligationScanner
	alertDispatcher   *notify.AlertDispatcher
	llamaProvisioner  *supervisor.LlamaProvisioner
	aiGateway         ai.Gateway
	llamaURL          string
	stopChan          chan struct{}
	closeOnce         sync.Once
	mu                sync.RWMutex

	eventsMu         sync.RWMutex
	eventSubscribers map[chan sseEvent]struct{}
}


// NewServer initializes the supervisor server, ensures storage directories,
// connects to SQLite WAL, and initializes ingress/mDNS managers.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}

	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.DataDir, "state", "sovereign.db")
	}

	if cfg.CaddyURL == "" {
		cfg.CaddyURL = os.Getenv("SOVEREIGN_CADDY_URL")
		if cfg.CaddyURL == "" {
			cfg.CaddyURL = "http://caddy:2019"
		}
	}

	// Ensure required storage layout: /data/{media,documents,models,state,drop}
	cfg.Logger.Info("verifying appliance storage layout", "data_dir", cfg.DataDir)
	if err := storage.EnsureDataLayout(cfg.DataDir); err != nil {
		cfg.Logger.Error("failed to initialize storage directories", "error", err, "data_dir", cfg.DataDir)
		return nil, fmt.Errorf("storage layout initialization failed: %w", err)
	}

	// Initialize SQLite WAL database
	cfg.Logger.Info("initializing SQLite WAL persistence", "db_path", cfg.DBPath)
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		cfg.Logger.Error("database initialization failed", "error", err, "db_path", cfg.DBPath)
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}

	if cfg.StatePath == "" {
		cfg.StatePath = filepath.Join(cfg.DataDir, "state", "boot_state.json")
	}

	if cfg.BootMgr == nil {
		cfg.BootMgr = store.NewBootManager(cfg.StatePath, db, cfg.Logger)
		if _, err := cfg.BootMgr.RecordBoot(context.Background()); err != nil {
			cfg.Logger.Warn("recording boot state resulted in non-fatal error", "error", err)
		}
	}

	if cfg.IngressMgr == nil {
		cfg.IngressMgr = ingress.NewCaddyClient(cfg.CaddyURL, cfg.Logger)
	}

	if cfg.MDNSService == nil {
		cfg.MDNSService = ingress.NewMDNSService(ingress.DefaultMDNSConfig(cfg.Logger))
	}

	if cfg.StorageEngine == nil {
		var err error
		cfg.StorageEngine, err = storage.NewEngine(storage.EngineConfig{
			DataDir: cfg.DataDir,
			DB:      db,
			Logger:  cfg.Logger,
		})
		if err != nil {
			cfg.Logger.Error("storage engine initialization failed", "error", err)
			return nil, fmt.Errorf("storage engine initialization failed: %w", err)
		}
	}

	if cfg.AuthMgr == nil {
		var err error
		cfg.AuthMgr, err = auth.NewManager(db.DB, cfg.Logger)
		if err != nil {
			cfg.Logger.Error("auth manager initialization failed", "error", err)
			return nil, fmt.Errorf("auth manager initialization failed: %w", err)
		}
	}

	if cfg.HardwareCollector == nil {
		cfg.HardwareCollector = hardware.NewSystemCollector()
	}

	if cfg.TelemetryInterval <= 0 {
		cfg.TelemetryInterval = 5 * time.Second
	}

	s := &Server{
		cfg:               cfg,
		db:                db,
		bootMgr:           cfg.BootMgr,
		storageEngine:     cfg.StorageEngine,
		authMgr:           cfg.AuthMgr,
		hardwareCollector: cfg.HardwareCollector,
		telemetryInterval: cfg.TelemetryInterval,
		logger:            cfg.Logger,
		router:            http.NewServeMux(),
		ingressMgr:        cfg.IngressMgr,
		mdnsService:       cfg.MDNSService,
		stopChan:          make(chan struct{}),
		eventSubscribers:  make(map[chan sseEvent]struct{}),
	}

	if cfg.LlamaProvisioner == nil {
		s.llamaProvisioner = supervisor.NewLlamaProvisioner(cfg.DataDir, cfg.Logger)
	} else {
		s.llamaProvisioner = cfg.LlamaProvisioner
	}

	if cfg.ServiceMgr == nil {
		var engine supervisor.RuntimeEngine
		sockPath := "/var/run/docker.sock"
		if _, err := os.Stat(sockPath); err == nil {
			engine = supervisor.NewDockerEngine(sockPath, cfg.Logger)
		} else {
			engine = supervisor.NewMockRuntimeEngine()
		}

		s.serviceMgr = supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
			DataDir:          cfg.DataDir,
			Engine:           engine,
			Logger:           cfg.Logger,
			EventBroadcaster: s.BroadcastEvent,
			LlamaProvisioner: s.llamaProvisioner,
		})
	} else {
		s.serviceMgr = cfg.ServiceMgr
		s.serviceMgr.SetEventBroadcaster(s.BroadcastEvent)
	}

	s.llamaURL = cfg.LlamaURL
	if s.llamaURL == "" {
		if env := os.Getenv("SOVEREIGN_LLAMA_URL"); env != "" {
			s.llamaURL = strings.TrimRight(env, "/")
		} else {
			s.llamaURL = "http://sovereign-llama:8080"
		}
	}

	if cfg.AIGateway == nil {
		hw, isGPU := supervisor.DetectLlamaHardware(supervisor.HardwareProbeOptions{DevRoot: "/dev", SysfsRoot: "/sys", ProcRoot: "/proc"})
		isHybrid := supervisor.IsHybridGPU(supervisor.HardwareProbeOptions{DevRoot: "/dev", SysfsRoot: "/sys", ProcRoot: "/proc"})
		cpuOptimized := !isGPU
		hwMode := hw.Type
		if hwMode == "" || hwMode == "none" {
			hwMode = "cpu"
		}
		var hybridAssignments map[string]string
		if isHybrid {
			hybridAssignments = map[string]string{
				"jellyfin": "igpu",
				"llama":    "dgpu",
			}
		}

		initialState := ai.ArbitratorStateReady
		modelsDir := filepath.Join(cfg.DataDir, "models")
		if scanned, scanErr := ai.ScanModels(modelsDir); scanErr != nil || len(scanned) == 0 {
			initialState = ai.ArbitratorStateSafeStub
		}

		arbitrator := ai.NewGPUArbitrator(ai.ArbitratorConfig{
			InitialState:      initialState,
			HardwareMode:      hwMode,
			IsGPU:             isGPU,
			IsHybrid:          isHybrid,
			CPUOptimized:      cpuOptimized,
			HybridAssignments: hybridAssignments,
			Logger:            cfg.Logger,
			UnloadFunc: func(ctx context.Context) error {
				if s.serviceMgr != nil {
					_, err := s.serviceMgr.DeactivateService(ctx, supervisor.ServiceLlama)
					return err
				}
				return nil
			},
			WakeFunc: func(ctx context.Context) error {
				if s.serviceMgr != nil {
					_, err := s.serviceMgr.ActivateService(ctx, supervisor.ServiceLlama)
					return err
				}
				return nil
			},
			OnStateChange: func(oldState, newState string) {
				if newState == ai.ArbitratorStateIdleSurrendered {
					s.BroadcastEvent("gpu.idle_surrender", map[string]any{
						"state":   newState,
						"message": "AI model surrendered VRAM due to idle inactivity",
					})
				}
			},
		})

		s.aiGateway = ai.NewGateway(ai.GatewayConfig{
			ModelsDir:  modelsDir,
			LlamaURL:   s.llamaURL,
			Logger:     cfg.Logger,
			Arbitrator: arbitrator,
		})
	} else {
		s.aiGateway = cfg.AIGateway
	}

	if cfg.DropzoneEngine == nil {
		var err error
		cfg.DropzoneEngine, err = storage.NewDropzoneEngine(storage.DropzoneConfig{
			DataDir:   cfg.DataDir,
			Logger:    cfg.Logger,
			Refresher: s.serviceMgr,
		})
		if err != nil {
			cfg.Logger.Error("dropzone engine initialization failed", "error", err)
			return nil, fmt.Errorf("dropzone engine initialization failed: %w", err)
		}
	} else if s.serviceMgr != nil {
		cfg.DropzoneEngine.SetRefresher(s.serviceMgr)
	}
	s.dropzoneEngine = cfg.DropzoneEngine

	if cfg.KEYDBManager == nil {
		keysPath := filepath.Join(cfg.DataDir, "state", "keys", "KEYDB.cfg")
		cfg.KEYDBManager = optical.NewKEYDBManager(keysPath, cfg.Logger)
	}

	if cfg.OpticalRipper == nil {
		if cfg.OpticalDriver == nil {
			cfg.OpticalDriver = optical.NewLinuxOpticalDriver("/dev/sr0", cfg.Logger)
		}
		cfg.OpticalRipper = optical.NewOpticalRipper(optical.RipperConfig{
			DataDir:          cfg.DataDir,
			Driver:           cfg.OpticalDriver,
			KeyMgr:           cfg.KEYDBManager,
			EventBroadcaster: s.BroadcastEvent,
			Logger:           cfg.Logger,
		})
	} else {
		cfg.OpticalRipper.SetEventBroadcaster(s.BroadcastEvent)
	}
	s.opticalRipper = cfg.OpticalRipper

	ripGuard := func(discDirPath string) bool {
		if s.opticalRipper == nil {
			return false
		}
		st := s.opticalRipper.GetStatus()
		if st.State == optical.StateRipping {
			if st.StagingPath == "" || filepath.Clean(st.StagingPath) == filepath.Clean(discDirPath) {
				return true
			}
		}
		return false
	}

	if cfg.EnrichmentEngine == nil {
		client := cfg.MetadataClient
		if client == nil {
			client = enrichment.NewHTTPMetadataClient(enrichment.HTTPMetadataConfig{
				TMDbAPIKey: os.Getenv("TMDB_API_KEY"),
			})
		}
		var err error
		cfg.EnrichmentEngine, err = enrichment.NewEnrichmentEngine(enrichment.EngineConfig{
			DataDir:        cfg.DataDir,
			Client:         client,
			Refresher:      s.serviceMgr,
			Broadcaster:    s.BroadcastEvent,
			ActiveRipGuard: ripGuard,
			Logger:         cfg.Logger,
		})
		if err != nil {
			cfg.Logger.Error("enrichment engine initialization failed", "error", err)
			return nil, fmt.Errorf("enrichment engine initialization failed: %w", err)
		}
	} else {
		if s.serviceMgr != nil {
			cfg.EnrichmentEngine.SetRefresher(s.serviceMgr)
		}
		cfg.EnrichmentEngine.SetBroadcaster(s.BroadcastEvent)
		cfg.EnrichmentEngine.SetActiveRipGuard(ripGuard)
	}
	s.enrichmentEngine = cfg.EnrichmentEngine

	if cfg.TranscodeEngine == nil {
		store := cfg.TranscodeStore
		if store == nil && db != nil {
			store = transcode.NewSQLiteStore(db.DB)
			if err := store.InitSchema(context.Background()); err != nil {
				cfg.Logger.Warn("transcode queue schema initialization warning", "error", err)
			}
		}
		sched := cfg.TranscodeSchedule
		if sched == nil {
			var err error
			sched, err = transcode.NewScheduler(transcode.DefaultScheduleConfig())
			if err != nil {
				cfg.Logger.Error("failed to create transcode scheduler", "error", err)
				return nil, fmt.Errorf("transcode scheduler initialization failed: %w", err)
			}
		}
		var err error
		cfg.TranscodeEngine, err = transcode.NewTranscodeEngine(transcode.EngineConfig{
			DataDir:     cfg.DataDir,
			Store:       store,
			Scheduler:   sched,
			Transcoder:  cfg.Transcoder,
			Refresher:   s.serviceMgr,
			Broadcaster: s.BroadcastEvent,
			Logger:      cfg.Logger,
		})
		if err != nil {
			cfg.Logger.Error("transcode engine initialization failed", "error", err)
			return nil, fmt.Errorf("transcode engine initialization failed: %w", err)
		}
	} else {
		if s.serviceMgr != nil {
			cfg.TranscodeEngine.SetRefresher(s.serviceMgr)
		}
		cfg.TranscodeEngine.SetBroadcaster(s.BroadcastEvent)
	}
	s.transcodeEngine = cfg.TranscodeEngine

	// 11. Initialize IMAP store, poller, and document ingestion
	if cfg.IMAPStore == nil && db != nil {
		var err error
		cfg.IMAPStore, err = store.NewIMAPStore(db, nil)
		if err != nil {
			cfg.Logger.Error("failed to initialize imap store", "error", err)
			return nil, fmt.Errorf("imap store initialization failed: %w", err)
		}
	}
	s.imapStore = cfg.IMAPStore

	consumeDir := filepath.Join(cfg.DataDir, "documents", "consume")
	if cfg.IngestEngine == nil {
		var err error
		cfg.IngestEngine, err = documents.NewIngestEngine(consumeDir, s.BroadcastEvent)
		if err != nil {
			cfg.Logger.Error("failed to initialize ingest engine", "error", err)
			return nil, fmt.Errorf("ingest engine initialization failed: %w", err)
		}
	}
	s.ingestEngine = cfg.IngestEngine

	if cfg.IMAPPoller == nil && s.imapStore != nil {
		healthSetter := func(serviceID, status, health, detail string) {
			if s.serviceMgr != nil {
				s.serviceMgr.SetServiceHealth(serviceID, status, health, detail)
			}
		}
		var err error
		cfg.IMAPPoller, err = documents.NewPoller(documents.PollerConfig{
			Store:               s.imapStore,
			ConsumeDir:          consumeDir,
			PollInterval:        documents.DefaultPollInterval,
			Broadcaster:         s.BroadcastEvent,
			ServiceHealthSetter: healthSetter,
			Logger:              cfg.Logger,
		})
		if err != nil {
			cfg.Logger.Error("failed to initialize imap poller", "error", err)
			return nil, fmt.Errorf("imap poller initialization failed: %w", err)
		}
	}
	s.imapPoller = cfg.IMAPPoller

	// 12. Initialize DeadlineStore and ObligationScanner
	if cfg.DeadlineStore == nil && db != nil {
		var err error
		cfg.DeadlineStore, err = store.NewDeadlineStore(db)
		if err != nil {
			cfg.Logger.Error("failed to initialize deadline store", "error", err)
			return nil, fmt.Errorf("deadline store initialization failed: %w", err)
		}
	}
	s.deadlineStore = cfg.DeadlineStore

	if cfg.ObligationScanner == nil && s.deadlineStore != nil {
		llmBaseURL := os.Getenv("SOVEREIGN_AI_GATEWAY_URL")
		if llmBaseURL == "" {
			llmBaseURL = os.Getenv("SOVEREIGN_LLM_URL")
		}
		if llmBaseURL == "" {
			llmBaseURL = "http://127.0.0.1:8080"
		}
		llmExtractor := documents.NewGatewayLLMExtractor(llmBaseURL, cfg.Logger)
		cfg.ObligationScanner = documents.NewObligationScanner(s.deadlineStore, llmExtractor, s.BroadcastEvent, cfg.Logger)
	}
	s.obligationScanner = cfg.ObligationScanner

	// 13. Initialize AlertDispatcher and wire into ObligationScanner
	if cfg.AlertDispatcher == nil && s.deadlineStore != nil && db != nil {
		cfg.AlertDispatcher = notify.NewAlertDispatcher(db, s.deadlineStore, cfg.Logger)
	}
	s.alertDispatcher = cfg.AlertDispatcher
	if s.obligationScanner != nil && s.alertDispatcher != nil {
		s.obligationScanner.SetAlertDispatcher(s.alertDispatcher)
	}
	if s.obligationScanner != nil && db != nil {
		if nCfg, err := notify.LoadConfig(context.Background(), db); err == nil && nCfg != nil && nCfg.DeadlineAlertThreshold > 0 {
			s.obligationScanner.SetAlertThreshold(nCfg.DeadlineAlertThreshold)
		}
	}

	s.routes()
	return s, nil
}


// StartBackgroundServices initializes background supervisor routines:
// starts mDNS responder broadcasting sovereign.local, and launches
// background route sync loop with Caddy.
func (s *Server) StartBackgroundServices(ctx context.Context) {
	// 1. Start mDNS responder
	if s.mdnsService != nil {
		go func() {
			if err := s.mdnsService.Start(); err != nil {
				s.logger.Warn("mDNS discovery responder failed to start", "error", err)
			}
		}()
	}

	// 2. Initial Caddy route synchronization with retry in background
	if s.ingressMgr != nil {
		go func() {
			s.logger.Info("starting background caddy route synchronization")
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			// Try initial sync immediately
			syncCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := s.syncIngressRoutes(syncCtx)
			cancel()
			if err == nil {
				s.logger.Info("initial caddy route synchronization succeeded")
				return
			}

			// Retry until successful or context cancelled
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					retryCtx, retryCancel := context.WithTimeout(ctx, 3*time.Second)
					err := s.syncIngressRoutes(retryCtx)
					retryCancel()
					if err == nil {
						s.logger.Info("caddy route synchronization succeeded after retry")
						return
					}
					s.logger.Debug("retrying caddy route sync...", "error", err)
				}
			}
		}()
	}

	// 3. Automatic integrity check trigger during diagnostics mode
	if s.bootMgr != nil && s.bootMgr.IsDiagnosticsMode() {
		go func() {
			s.logger.Info("unclean shutdown detected: initializing in diagnostics mode and running integrity checks")
			diagCtx, diagCancel := context.WithTimeout(ctx, 15*time.Second)
			defer diagCancel()
			if err := s.bootMgr.RunIntegrityCheck(diagCtx); err != nil {
				s.logger.Error("diagnostics integrity check failed", "error", err)
			} else {
				s.logger.Info("boot diagnostics integrity check succeeded, system operational")
			}
		}()
	}

	// 4. Background storage health probe
	if s.storageEngine != nil {
		s.storageEngine.StartProbing(ctx)
	}

	// 5. Prune expired auth sessions on startup
	if s.authMgr != nil {
		if err := s.authMgr.PruneExpiredSessions(ctx); err != nil {
			s.logger.Warn("failed to prune expired sessions on startup", "error", err)
		}
	}

	// 6. Start service watchdog probing
	if s.serviceMgr != nil {
		s.serviceMgr.StartWatchdog(ctx)
	}

	// 7. Start dropzone background folder scanner
	if s.dropzoneEngine != nil {
		s.dropzoneEngine.Start(ctx)
	}

	// 8. Start optical ripper hardware supervisor loop
	if s.opticalRipper != nil {
		s.opticalRipper.Start(ctx)
	}

	// 9. Start metadata enrichment worker loop
	if s.enrichmentEngine != nil {
		s.enrichmentEngine.Start(ctx)
	}

	// 10. Start off-peak transcoding queue runner
	if s.transcodeEngine != nil {
		s.transcodeEngine.Start(ctx)
	}

	// 11. Start background IMAP email poller
	if s.imapPoller != nil {
		s.imapPoller.Start(ctx)
	}

	// 12. Start obligation scanner background runner
	if s.obligationScanner != nil {
		s.obligationScanner.Start(ctx)
	}
}

// BootManager returns the active BootManager instance.
func (s *Server) BootManager() *store.BootManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bootMgr
}

// StorageEngine returns the active storage engine.
func (s *Server) StorageEngine() *storage.Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.storageEngine
}

// AuthManager returns the active auth manager.
func (s *Server) AuthManager() *auth.Manager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authMgr
}

// HardwareCollector returns the active hardware collector.
func (s *Server) HardwareCollector() hardware.Collector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hardwareCollector
}

// ServiceManager returns the active ServiceManager instance.
func (s *Server) ServiceManager() *supervisor.ServiceManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serviceMgr
}

// DropzoneEngine returns the active DropzoneEngine instance.
func (s *Server) DropzoneEngine() *storage.DropzoneEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dropzoneEngine
}

// OpticalRipper returns the active OpticalRipper instance.
func (s *Server) OpticalRipper() *optical.OpticalRipper {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.opticalRipper
}

// EnrichmentEngine returns the active EnrichmentEngine instance.
func (s *Server) EnrichmentEngine() *enrichment.EnrichmentEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enrichmentEngine
}

// TranscodeEngine returns the active TranscodeEngine instance.
func (s *Server) TranscodeEngine() *transcode.TranscodeEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.transcodeEngine
}

// BroadcastEvent emits an SSE event to all actively connected SSE clients.
func (s *Server) BroadcastEvent(event string, data any) {
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()

	evt := sseEvent{Event: event, Data: data}
	for ch := range s.eventSubscribers {
		select {
		case ch <- evt:
		default:
			// Non-blocking drop if client channel buffer is full
		}
	}

	// When an optical rip completes, automatically trigger enrichment scan
	if event == "optical.disc_ejected" {
		if m, ok := data.(map[string]any); ok && m["reason"] == "complete" {
			if s.enrichmentEngine != nil {
				go func() {
					_, _ = s.enrichmentEngine.Scan(context.Background())
				}()
			}
		}
	}

	// When a document is ingested, automatically trigger obligation scanning asynchronously
	if event == "dms.document_ingested" {
		if m, ok := data.(map[string]any); ok {
			docID, _ := m["id"].(string)
			filename, _ := m["filename"].(string)
			consumePath, _ := m["consume_path"].(string)
			if consumePath == "" {
				consumePath, _ = m["path"].(string)
			}
			if s.obligationScanner != nil && consumePath != "" {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_, _ = s.obligationScanner.ScanDocumentFile(ctx, docID, filename, consumePath, false)
				}()
			}
		}
	}
}

// Close closes database handles and frees ingress/mDNS/storage/supervisor resources.
func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		if s.stopChan != nil {
			close(s.stopChan)
		}
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.obligationScanner != nil {
		s.obligationScanner.Stop()
	}

	if s.transcodeEngine != nil {
		_ = s.transcodeEngine.Close()
	}

	if s.imapPoller != nil {
		s.imapPoller.Stop()
	}

	if s.enrichmentEngine != nil {
		_ = s.enrichmentEngine.Close()
	}

	if s.opticalRipper != nil {
		_ = s.opticalRipper.Close()
	}

	if s.dropzoneEngine != nil {
		_ = s.dropzoneEngine.Close()
	}

	if s.serviceMgr != nil {
		_ = s.serviceMgr.Close()
	}

	if s.storageEngine != nil {
		_ = s.storageEngine.Close()
	}

	if s.mdnsService != nil {
		s.mdnsService.Close()
	}

	if s.bootMgr != nil {
		_ = s.bootMgr.Close()
	}

	if s.aiGateway != nil {
		s.aiGateway.Close()
	}

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// requireSetupOrAdmin allows unauthenticated access during first-run setup, but strictly requires
// admin authentication once the appliance setup has been completed.
func (s *Server) requireSetupOrAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		authMgr := s.authMgr
		s.mu.RUnlock()

		if authMgr != nil {
			configured, err := authMgr.IsConfigured(r.Context())
			if err == nil && configured {
				authMgr.RequireAdmin(http.HandlerFunc(next)).ServeHTTP(w, r)
				return
			}
		}
		next(w, r)
	}
}

// routes registers HTTP REST handlers.
func (s *Server) routes() {
	s.router.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/system/boot-status", s.handleBootStatus)
	s.router.HandleFunc("GET /api/v1/system/ingress", s.handleIngressStatus)
	s.router.HandleFunc("GET /api/v1/system/events", s.handleEvents)
	s.router.HandleFunc("GET /api/events", s.handleEvents)
	s.router.HandleFunc("GET /api/v1/storage/disks", s.handleStorageDisks)
	s.router.HandleFunc("POST /api/v1/storage/initialize", s.requireSetupOrAdmin(s.handleStorageInitialize))
	s.router.HandleFunc("POST /api/v1/storage/nas/test", s.requireSetupOrAdmin(s.handleStorageNASTest))
	s.router.HandleFunc("POST /api/v1/storage/nas/mount", s.requireSetupOrAdmin(s.handleStorageNASMount))
	s.router.HandleFunc("GET /api/v1/storage/status", s.handleStorageStatus)

	// Auth and Setup Wizard endpoints
	s.router.HandleFunc("GET /api/v1/auth/setup-status", s.handleSetupStatus)
	s.router.HandleFunc("POST /api/v1/auth/setup", s.handleSetup)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.router.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.router.HandleFunc("GET /api/v1/auth/session", s.handleSession)

	// Protected administrative settings endpoints
	s.router.Handle("GET /api/v1/settings", s.authMgr.RequireAdmin(http.HandlerFunc(s.handleGetSettings)))
	s.router.Handle("POST /api/v1/settings", s.authMgr.RequireAdmin(http.HandlerFunc(s.handleUpdateSettings)))

	// Managed Services lifecycle endpoints
	s.router.HandleFunc("GET /api/v1/services", s.handleListServices)
	s.router.HandleFunc("GET /api/v1/services/{id}", s.handleGetService)
	s.router.HandleFunc("POST /api/v1/services/{id}/activate", s.handleActivateService)
	s.router.HandleFunc("POST /api/v1/services/{id}/deactivate", s.handleDeactivateService)
	s.router.HandleFunc("POST /api/v1/services/{id}/restart", s.requireSetupOrAdmin(s.handleRestartService))
	s.router.HandleFunc("POST /api/v1/services/{id}/refresh", s.handleRefreshService)

	// Media Dropzone ingestion endpoints
	s.router.HandleFunc("POST /api/v1/drop/upload", s.handleDropUpload)
	s.router.HandleFunc("GET /api/v1/drop/history", s.handleDropHistory)

	// Optical drive and automated disc ripping endpoints
	s.router.HandleFunc("GET /api/v1/optical/status", s.handleOpticalStatus)
	s.router.HandleFunc("POST /api/v1/optical/eject", s.handleOpticalEject)
	s.router.HandleFunc("POST /api/v1/optical/keys", s.requireSetupOrAdmin(s.handleOpticalKeys))

	// Media metadata and artwork enrichment endpoints
	s.router.HandleFunc("GET /api/v1/enrichment/status", s.handleEnrichmentStatus)
	s.router.HandleFunc("POST /api/v1/enrichment/scan", s.handleEnrichmentScan)

	// Off-peak transcoding queue and pre-encoding endpoints
	s.router.HandleFunc("GET /api/v1/transcode/queue", s.handleTranscodeQueue)
	s.router.HandleFunc("POST /api/v1/transcode/queue", s.handleEnqueueTranscode)
	s.router.HandleFunc("POST /api/v1/transcode/mobile", s.requireSetupOrAdmin(s.handleEnqueueMobile))
	s.router.HandleFunc("GET /api/v1/transcode/schedule", s.handleGetTranscodeSchedule)
	s.router.HandleFunc("POST /api/v1/transcode/schedule", s.requireSetupOrAdmin(s.handleUpdateTranscodeSchedule))
	s.router.HandleFunc("POST /api/v1/transcode/override", s.requireSetupOrAdmin(s.handleTranscodeOverride))

	// Document ingestion and IMAP email polling endpoints
	s.router.HandleFunc("POST /api/v1/documents/upload", s.requireSetupOrAdmin(s.handleDocumentUpload))
	s.router.HandleFunc("GET /api/v1/documents/imap", s.requireSetupOrAdmin(s.handleGetIMAPConfig))
	s.router.HandleFunc("POST /api/v1/documents/imap", s.requireSetupOrAdmin(s.handleSaveIMAPConfig))
	s.router.HandleFunc("POST /api/v1/documents/imap/test", s.requireSetupOrAdmin(s.handleTestIMAPConnection))

	// Document deadline extraction and obligation scanner endpoints
	s.router.HandleFunc("GET /api/v1/documents/deadlines", s.handleGetDeadlines)
	s.router.HandleFunc("POST /api/v1/documents/deadlines/{id}/dismiss", s.requireSetupOrAdmin(s.handleDismissDeadline))
	s.router.HandleFunc("POST /api/v1/documents/deadlines/scan", s.requireSetupOrAdmin(s.handleScanDeadlines))

	// Multi-channel alert dispatcher endpoints
	s.router.HandleFunc("GET /api/v1/alerts/config", s.requireSetupOrAdmin(s.handleGetAlertConfig))
	s.router.HandleFunc("PUT /api/v1/alerts/config", s.requireSetupOrAdmin(s.handleUpdateAlertConfig))
	s.router.HandleFunc("POST /api/v1/alerts/test/webhook", s.requireSetupOrAdmin(s.handleTestWebhook))
	s.router.HandleFunc("POST /api/v1/alerts/test/mqtt", s.requireSetupOrAdmin(s.handleTestMQTT))
	s.router.HandleFunc("POST /api/v1/alerts/test/email", s.requireSetupOrAdmin(s.handleTestEmail))
	s.router.HandleFunc("POST /api/v1/alerts/dispatch", s.requireSetupOrAdmin(s.handleDispatchAlerts))

	// AI Gateway mediated inference endpoints, presets, and governor status
	s.router.HandleFunc("GET /api/v1/ai/presets", s.handleAIPresets)
	s.router.HandleFunc("GET /api/v1/ai/governor/status", s.handleAIGovernorStatus)
	s.router.HandleFunc("GET /api/v1/ai/status", s.handleAIStatus)
	s.router.HandleFunc("/v1", s.handleAIGateway)
	s.router.HandleFunc("/v1/{path...}", s.handleAIGateway)
}

// handleAIGateway mediates /v1 inference requests to the internal llama-server spoke,
// falling back to Safe Stub Mode (HTTP 503 Problem Details) when no models are active or provisioned,
// or returning a 503 Problem Details response when the AI service is stopped/inactive.
func (s *Server) handleAIGateway(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	gw := s.aiGateway
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if gw == nil {
		ai.SafeStubHandler(w, r)
		return
	}

	if mgr != nil {
		st, err := mgr.GetService(r.Context(), supervisor.ServiceLlama)
		if err == nil && st != nil {
			if st.Status == supervisor.StatusInactive || st.Status == supervisor.StatusStopped {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"type":     "https://sovereign.local/problems/service-inactive",
					"title":    "AI Service Inactive",
					"status":   http.StatusServiceUnavailable,
					"detail":   "The AI inference service is currently stopped or inactive. Please activate it in Settings to enable local inference.",
					"instance": r.URL.Path,
				})
				return
			}
			if st.Health == supervisor.HealthDegraded {
				ai.SafeStubHandler(w, r)
				return
			}
		}
	}

	gw.ServeHTTP(w, r)
}

// handleAIPresets returns the catalogue of pre-calibrated generation intent presets.
func (s *Server) handleAIPresets(w http.ResponseWriter, r *http.Request) {
	presets := ai.ListPresets()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"presets": presets,
	})
}

// handleAIGovernorStatus reports live request governor concurrency metrics and queue depths.
func (s *Server) handleAIGovernorStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	gw := s.aiGateway
	s.mu.RUnlock()

	var status ai.GovernorStatus
	if gw != nil && gw.Governor() != nil {
		status = gw.Governor().Status()
	} else {
		status = ai.GovernorStatus{
			TotalSlots:          2,
			ReservedInteractive: 1,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// handleAIStatus reports real-time GPU arbitration state, idle countdown, day/night habit mode, and hybrid GPU assignments.
func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	gw := s.aiGateway
	s.mu.RUnlock()

	var status ai.ArbitratorStatus
	if gw != nil && gw.Arbitrator() != nil {
		status = gw.Arbitrator().Status()
	} else {
		status = ai.ArbitratorStatus{
			State:              ai.ArbitratorStateSafeStub,
			HardwareMode:       "cpu",
			IsGPU:              false,
			IsHybrid:           false,
			CPUOptimized:       true,
			IsDaytime:          ai.IsDaytime(time.Now()),
			IdleSeconds:        0,
			IdleTimeoutSeconds: 300,
			ReassuranceMessage: ai.ReassuranceCPUOptimized,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}


// isAllowedOrigin checks if the origin is trusted (localhost, 127.0.0.1, sovereign.local, or matching host/LAN IP).
func isAllowedOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	if originHost == "localhost" || originHost == "127.0.0.1" || originHost == "sovereign.local" {
		return true
	}
	reqHost := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		reqHost = h
	}
	if strings.EqualFold(originHost, reqHost) {
		return true
	}
	ip := net.ParseIP(originHost)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
		return true
	}
	return false
}

// Handler returns the HTTP handler for the server with CORS middleware.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		s.router.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	prob := ProblemDetails{
		Type:     fmt.Sprintf("https://sovereign.local/errors/http-%d", status),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
	_ = json.NewEncoder(w).Encode(prob)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	dbHandle := s.db
	s.mu.RUnlock()

	if dbHandle == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Database is not initialized", r.URL.Path)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := dbHandle.PingContext(ctx); err != nil {
		s.logger.Error("health probe database check failed", "error", err)
		s.writeProblem(w, http.StatusServiceUnavailable, "Database Unavailable", "Database ping probe failed", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, HealthResponse{
		Status:   "ok",
		Version:  AppVersion,
		Database: "connected",
	})
}

func (s *Server) handleBootStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	bootMgr := s.bootMgr
	s.mu.RUnlock()

	if bootMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Boot manager is not initialized", r.URL.Path)
		return
	}

	status, degraded, err := bootMgr.GetStatus()
	if degraded {
		s.writeProblem(w, http.StatusServiceUnavailable, "Storage Degraded", "State storage filesystem is unwritable or read-only", r.URL.Path)
		return
	}
	if err != nil {
		s.logger.Error("retrieving boot status failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve boot status record", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleIngressStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	s.mu.RLock()
	ingressMgr := s.ingressMgr
	mdnsSvc := s.mdnsService
	s.mu.RUnlock()

	caddyConnected := false
	if ingressMgr != nil {
		caddyStatus, err := ingressMgr.GetStatus(ctx)
		if err == nil {
			if conn, ok := caddyStatus["connected"].(bool); ok && conn {
				caddyConnected = true
			}
		}
	}

	mdnsActive := false
	hostname := "sovereign.local"
	if mdnsSvc != nil {
		mdnsActive = mdnsSvc.IsActive()
		hostname = mdnsSvc.Hostname()
	}

	overallStatus := "ok"
	if !caddyConnected {
		overallStatus = "error"
	} else if !mdnsActive {
		overallStatus = "degraded"
	}

	routes := []IngressRoute{
		{Service: "web", Subpath: "/", Upstream: "sovereign-web:3000"},
		{Service: "web", Subpath: "/drop", Upstream: "sovereign-web:3000"},
		{Service: "daemon", Subpath: "/api/*", Upstream: "sovereign-daemon:8080"},
	}

	if s.serviceMgr != nil {
		if svcs, err := s.serviceMgr.ListServices(ctx); err == nil {
			for _, svc := range svcs {
				if svc.Status == supervisor.StatusRunning {
					routes = append(routes, IngressRoute{
						Service:  svc.ID,
						Subpath:  svc.Subpath,
						Upstream: fmt.Sprintf("sovereign-%s:%d", svc.ID, svc.Port),
					})
				}
			}
		}
	}

	s.writeJSON(w, http.StatusOK, IngressStatusResponse{
		Status:         overallStatus,
		CaddyConnected: caddyConnected,
		MDNSActive:     mdnsActive,
		Hostname:       hostname,
		Routes:         routes,
	})
}

func (s *Server) getTelemetryUpdate(ctx context.Context) TelemetryUpdate {
	s.mu.RLock()
	hwCollector := s.hardwareCollector
	storageEng := s.storageEngine
	ripper := s.opticalRipper
	s.mu.RUnlock()

	var cpuPercent float64 = 5.0
	var memUsed, memTotal uint64
	var hwStorageUsed, hwStorageTotal uint64
	if hwCollector != nil {
		if snap, err := hwCollector.CollectSnapshot(ctx); err == nil && snap != nil {
			cpuPercent = snap.CPUUsagePercent
			memUsed = snap.MemoryUsedBytes
			memTotal = snap.MemoryTotalBytes
			hwStorageUsed = snap.StorageUsedBytes
			hwStorageTotal = snap.StorageTotalBytes
		}
	}

	var storageUsed, storageTotal uint64
	var storagePercent float64
	nasStatus := "not_configured"
	nasMessage := "No network storage configured"
	overallStatus := "operational"

	if storageEng != nil {
		st := storageEng.GetStorageStatus(ctx)
		storageUsed = st.UsedBytes
		storageTotal = st.TotalBytes
		if storageTotal == 0 && hwStorageTotal > 0 {
			storageUsed = hwStorageUsed
			storageTotal = hwStorageTotal
		}
		if storageTotal > 0 {
			storagePercent = math.Round((float64(storageUsed)/float64(storageTotal))*1000) / 10
		}
		if st.NAS != nil && st.NAS.Configured {
			nasStatus = st.NAS.Status
			nasMessage = st.NAS.Message
		}
		if st.Status == "error" || nasStatus == "error" {
			overallStatus = "error"
		} else if st.Status == "degraded" || nasStatus == "degraded" || nasStatus == "disconnected" {
			overallStatus = "degraded"
		}
	} else if hwStorageTotal > 0 {
		storageUsed = hwStorageUsed
		storageTotal = hwStorageTotal
		storagePercent = math.Round((float64(storageUsed)/float64(storageTotal))*1000) / 10
	}

	opticalStatus := "Idle · Ready for disc"
	if ripper != nil {
		opticalStatus = ripper.GetStatus().StatusText
	}
	if s.enrichmentEngine != nil && s.enrichmentEngine.IsEnriching() {
		opticalStatus = "Matching metadata"
	}

	gpuStatus := "Daytime (AI Priority)"
	if s.transcodeEngine != nil {
		gpuStatus = s.transcodeEngine.GetGPUStatus()
	}

	return TelemetryUpdate{
		CPUPercent:        cpuPercent,
		MemoryUsedBytes:   memUsed,
		MemoryTotalBytes:  memTotal,
		StorageUsedBytes:  storageUsed,
		StorageTotalBytes: storageTotal,
		StoragePercent:    storagePercent,
		NASStatus:         nasStatus,
		NASMessage:        nasMessage,
		OverallStatus:     overallStatus,
		OpticalStatus:     opticalStatus,
		GPUStatus:         gpuStatus,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	eventCh := make(chan sseEvent, 64)
	s.eventsMu.Lock()
	s.eventSubscribers[eventCh] = struct{}{}
	s.eventsMu.Unlock()

	defer func() {
		s.eventsMu.Lock()
		delete(s.eventSubscribers, eventCh)
		s.eventsMu.Unlock()
	}()

	// Push initial event immediately on connection
	initEvt := s.getTelemetryUpdate(r.Context())
	if data, err := json.Marshal(initEvt); err == nil {
		if _, err := fmt.Fprintf(w, "event: telemetry.update\ndata: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
	}

	interval := s.telemetryInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopChan:
			return
		case evt := <-eventCh:
			data, err := json.Marshal(evt.Data)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Event, data); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			evt := s.getTelemetryUpdate(r.Context())
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: telemetry.update\ndata: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleStorageDisks(w http.ResponseWriter, r *http.Request) {

	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	disks, err := storageEng.ListAvailableDisks(r.Context())
	if err != nil {
		s.logger.Error("failed to list available disks", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Storage Error", "Failed to enumerate block devices", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"disks": disks,
	})
}

func (s *Server) handleStorageInitialize(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	var req struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Device) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Device", "Device path cannot be empty", r.URL.Path)
		return
	}

	res, err := storageEng.InitializeStorage(r.Context(), req.Device)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidDevice) ||
			errors.Is(err, storage.ErrSystemDeviceProtected) ||
			errors.Is(err, storage.ErrDeviceInUse) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Device", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("storage initialization failed", "device", req.Device, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Storage Initialization Failed", "Failed to format and mount storage pool", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStorageNASTest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	var req storage.NASTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	res, err := storageEng.TestNAS(r.Context(), req)
	if err != nil {
		s.logger.Warn("NAS test connection failed", "host", req.Host, "error", err)
		s.writeProblem(w, http.StatusBadRequest, "NAS Connection Failed", err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStorageNASMount(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	var req storage.NASMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	res, err := storageEng.MountNAS(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidNASConfig) ||
			errors.Is(err, storage.ErrNASAuthFailed) ||
			errors.Is(err, storage.ErrNASUnreachable) {
			s.writeProblem(w, http.StatusBadRequest, "NAS Configuration Error", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("NAS mount failed", "host", req.Host, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Mount Failed", "Failed to mount network storage share", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	status := storageEng.GetStorageStatus(r.Context())
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	dbHandle := s.db
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	configured, err := authMgr.IsConfigured(r.Context())
	if err != nil {
		s.logger.Error("failed to check setup status", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to check setup configuration status", r.URL.Path)
		return
	}

	res := map[string]any{
		"is_configured": configured,
	}
	if configured && dbHandle != nil {
		if lang, err := dbHandle.GetSetting(r.Context(), "language"); err == nil && lang != "" {
			res["language"] = lang
		}
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	var req auth.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	sess, err := authMgr.CompleteSetup(r.Context(), req)
	if err != nil {
		if errors.Is(err, auth.ErrAlreadyConfigured) {
			s.writeProblem(w, http.StatusConflict, "Already Configured", "Appliance first-run setup has already been completed", r.URL.Path)
			return
		}
		if errors.Is(err, auth.ErrPasswordTooShort) || errors.Is(err, auth.ErrInvalidEmail) {
			s.writeProblem(w, http.StatusBadRequest, "Validation Error", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("setup completion failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Setup Failed", "Failed to complete appliance setup", r.URL.Path)
		return
	}

	authMgr.SetSessionCookie(w, sess.Token)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"is_configured": true,
		"session":       sess,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	sess, err := authMgr.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			s.writeProblem(w, http.StatusUnauthorized, "Unauthorized", "Invalid username or password", r.URL.Path)
			return
		}
		s.logger.Error("login authentication failed", "username", req.Username, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Authentication Error", "Authentication failed", r.URL.Path)
		return
	}

	authMgr.SetSessionCookie(w, sess.Token)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"session": sess,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr != nil {
		if sess, err := authMgr.GetSessionFromRequest(r); err == nil && sess != nil {
			_ = authMgr.RevokeSession(r.Context(), sess.Token)
		}
		authMgr.ClearSessionCookie(w)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": "Logged out successfully",
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	sess, err := authMgr.GetSessionFromRequest(r)
	if err != nil || sess == nil {
		s.writeProblem(w, http.StatusUnauthorized, "Unauthorized", "No active session", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"session":       sess,
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	dbHandle := s.db
	s.mu.RUnlock()

	if dbHandle == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database is not initialized", r.URL.Path)
		return
	}

	ctx := r.Context()
	lang, _ := dbHandle.GetSetting(ctx, "language")
	email, _ := dbHandle.GetSetting(ctx, "admin_email")
	hostname, _ := dbHandle.GetSetting(ctx, "hostname")
	features, _ := dbHandle.GetSetting(ctx, "features")

	s.writeJSON(w, http.StatusOK, map[string]any{
		"language":    lang,
		"admin_email": email,
		"hostname":    hostname,
		"features":    features,
	})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	dbHandle := s.db
	s.mu.RUnlock()

	if dbHandle == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database is not initialized", r.URL.Path)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	ctx := r.Context()
	for k, v := range payload {
		if err := dbHandle.SetSetting(ctx, k, v); err != nil {
			s.logger.Error("failed to set setting", "key", k, "error", err)
			s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to update setting", r.URL.Path)
			return
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func extractServiceID(r *http.Request, action string) string {
	id := r.PathValue("id")
	if id != "" {
		return id
	}
	// Fallback parsing for /api/v1/services/{id}/{action} or /api/v1/services/{id}
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "services" && i+1 < len(parts) {
			val := parts[i+1]
			if val != action {
				return val
			}
		}
	}
	return ""
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	services, err := mgr.ListServices(r.Context())
	if err != nil {
		s.logger.Error("failed to list services", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve service list", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"services": services,
	})
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.GetService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to get service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to get service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleActivateService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "activate")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.ActivateService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrPullFailed) {
			s.writeProblem(w, http.StatusBadGateway, "Image Pull Failed", fmt.Sprintf("Failed to pull OCI image for '%s': %v", id, err), r.URL.Path)
			return
		}
		s.logger.Error("failed to activate service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to activate service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleDeactivateService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "deactivate")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.DeactivateService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to deactivate service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to deactivate service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "restart")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.RestartService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrInactiveService) || strings.Contains(err.Error(), "cannot restart inactive service") {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("failed to restart service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to restart service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleRefreshService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "refresh")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	err := mgr.RefreshService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrInvalidOperation) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Operation", err.Error(), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrServiceInactive) || errors.Is(err, supervisor.ErrInactiveService) {
			s.writeProblem(w, http.StatusServiceUnavailable, "Service Inactive", fmt.Sprintf("Service '%s' is not active", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to refresh service library", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Refresh Failed", fmt.Sprintf("Failed to refresh service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   id,
		"refreshed": true,
	})
}

// handleDropUpload handles multipart file uploads to /api/v1/drop/upload.
// Streams file into atomic staging path, validates magic bytes, moves to /data/media/:category,
// triggers library refresh within 5s, and returns 200 with metadata.
func (s *Server) handleDropUpload(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.dropzoneEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Dropzone engine is not initialized", r.URL.Path)
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Failed to parse multipart/form-data upload: "+err.Error(), r.URL.Path)
		return
	}

	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	if category == "" {
		category = strings.ToLower(strings.TrimSpace(r.Header.Get("X-Media-Category")))
	}

	var uploadedItem *storage.DropHistoryItem
	var uploadErr error
	var fileProcessed bool

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(r.Context().Err(), context.Canceled) {
				s.writeProblem(w, 499, "Upload Aborted", "Client closed connection during upload stream", r.URL.Path)
				return
			}
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Failed reading multipart part: "+err.Error(), r.URL.Path)
			return
		}

		formName := part.FormName()
		if formName == "category" {
			catBytes, _ := io.ReadAll(part)
			_ = part.Close()
			category = strings.ToLower(strings.TrimSpace(string(catBytes)))
			continue
		}

		if formName == "file" {
			filename := part.FileName()
			if category == "" || !storage.IsValidCategory(category) {
				_ = part.Close()
				s.writeProblem(w, http.StatusBadRequest, "Invalid Category", "Category must be one of: movies, tv, music, audiobooks, documents", r.URL.Path)
				return
			}

			fileProcessed = true
			uploadedItem, uploadErr = engine.HandleUploadStream(r.Context(), category, filename, part)
			_ = part.Close()
			break
		}
		_ = part.Close()
	}

	if !fileProcessed {
		s.writeProblem(w, http.StatusBadRequest, "Missing File", "Multipart field 'file' is required", r.URL.Path)
		return
	}

	if uploadErr != nil {
		if errors.Is(uploadErr, storage.ErrInvalidCategory) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Category", "Category must be one of: movies, tv, music, audiobooks, documents", r.URL.Path)
			return
		}
		if category == storage.CategoryDocuments && (errors.Is(uploadErr, storage.ErrInvalidMedia) || errors.Is(uploadErr, storage.ErrCategoryMismatch)) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Document Format", "Only document files (.pdf, .png, .jpg, .jpeg, .tiff, .tif) are accepted.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrInvalidMedia) {
			s.writeProblem(w, http.StatusUnprocessableEntity, "Invalid Media Format", "Only media files are accepted. Please upload video or audio files.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrCategoryMismatch) {
			s.writeProblem(w, http.StatusUnprocessableEntity, "Category Mismatch", "The uploaded file format does not match the selected category.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrEmptyFile) {
			s.writeProblem(w, http.StatusUnprocessableEntity, "Empty File", "The uploaded file is empty.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, context.Canceled) || errors.Is(uploadErr, storage.ErrStagingAborted) || errors.Is(r.Context().Err(), context.Canceled) {
			s.writeProblem(w, 499, "Upload Aborted", "Client closed connection during upload stream", r.URL.Path)
			return
		}
		s.logger.Error("dropzone upload processing failed", "category", category, "error", uploadErr)
		s.writeProblem(w, http.StatusInternalServerError, "Upload Failed", "An error occurred while processing the media upload.", r.URL.Path)
		return
	}

	if category == storage.CategoryDocuments && uploadedItem != nil {
		s.BroadcastEvent("dms.document_ingested", map[string]any{
			"id":           uploadedItem.ID,
			"filename":     uploadedItem.Filename,
			"path":         uploadedItem.Path,
			"size":         uploadedItem.Size,
			"content_type": uploadedItem.ContentType,
			"source":       "dropzone",
		})
	}

	s.writeJSON(w, http.StatusOK, uploadedItem)
}

// handleDropHistory returns recent ingestion and promotion history.
func (s *Server) handleDropHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.dropzoneEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}

	items := engine.GetHistory()
	if items == nil {
		items = []storage.DropHistoryItem{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *Server) syncIngressRoutes(ctx context.Context) error {
	s.mu.RLock()
	ingressMgr := s.ingressMgr
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if ingressMgr == nil {
		return nil
	}

	var routes []ingress.RouteConfig
	if mgr != nil {
		services, err := mgr.ListServices(ctx)
		if err != nil {
			s.logger.Warn("failed to list services for ingress route sync", "error", err)
			return err
		}

		for _, svc := range services {
			if svc.Status == supervisor.StatusRunning {
				routes = append(routes, ingress.RouteConfig{
					Service: svc.ID,
					Port:    svc.Port,
					Subpath: svc.Subpath,
				})
			}
		}
	}

	syncCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := ingressMgr.SyncRoutes(syncCtx, routes); err != nil {
		s.logger.Warn("failed to sync ingress routes with caddy", "error", err)
		return err
	}
	return nil
}

// handleOpticalStatus returns current optical hardware and disc ripping state.
func (s *Server) handleOpticalStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ripper := s.opticalRipper
	s.mu.RUnlock()

	if ripper == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Optical ripping engine is not initialized", r.URL.Path)
		return
	}

	status := ripper.GetStatus()
	s.writeJSON(w, http.StatusOK, status)
}

// handleOpticalEject issues a hardware eject request to open the optical drive tray.
func (s *Server) handleOpticalEject(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ripper := s.opticalRipper
	s.mu.RUnlock()

	if ripper == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Optical ripping engine is not initialized", r.URL.Path)
		return
	}

	if err := ripper.Eject(r.Context()); err != nil {
		s.logger.Error("failed to eject optical drive", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Eject Failed", "Failed to eject optical drive: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, OpticalEjectResponse{
		Status:  "ejected",
		Message: "Disc ejected successfully",
	})
}

// handleOpticalKeys handles validation and atomic storage of KEYDB.cfg for Blu-ray decryption.
func (s *Server) handleOpticalKeys(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ripper := s.opticalRipper
	s.mu.RUnlock()

	if ripper == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Optical ripping engine is not initialized", r.URL.Path)
		return
	}

	var content []byte
	contentType := r.Header.Get("Content-Type")

	const maxKeyUploadSize = 10 << 20 // 10MB
	if r.ContentLength > maxKeyUploadSize {
		s.writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Key file payload exceeds 10MB limit", r.URL.Path)
		return
	}

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxKeyUploadSize); err != nil {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Form", "Failed to parse multipart form: "+err.Error(), r.URL.Path)
			return
		}

		var fileHeader io.Reader
		file, _, err := r.FormFile("file")
		if err == nil {
			fileHeader = file
			defer file.Close()
		} else {
			file, _, err := r.FormFile("keydb")
			if err == nil {
				fileHeader = file
				defer file.Close()
			}
		}

		if fileHeader == nil {
			s.writeProblem(w, http.StatusBadRequest, "Missing File", "No key file provided in form field 'file' or 'keydb'", r.URL.Path)
			return
		}

		var readErr error
		content, readErr = io.ReadAll(fileHeader)
		if readErr != nil {
			s.writeProblem(w, http.StatusBadRequest, "Read Error", "Failed to read uploaded key file: "+readErr.Error(), r.URL.Path)
			return
		}
	} else {
		var readErr error
		content, readErr = io.ReadAll(io.LimitReader(r.Body, maxKeyUploadSize+1))
		if readErr != nil {
			s.writeProblem(w, http.StatusBadRequest, "Read Error", "Failed to read request body: "+readErr.Error(), r.URL.Path)
			return
		}
		if len(content) > maxKeyUploadSize {
			s.writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Key file payload exceeds 10MB limit", r.URL.Path)
			return
		}
	}

	if len(content) == 0 {
		s.writeProblem(w, http.StatusBadRequest, "Empty File", "Uploaded KEYDB.cfg is empty", r.URL.Path)
		return
	}

	count, err := ripper.SaveKeys(content)
	if err != nil {
		if errors.Is(err, optical.ErrEmptyKeyFile) || errors.Is(err, optical.ErrBinaryContent) || errors.Is(err, optical.ErrInvalidFormat) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Key File", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("failed to save KEYDB.cfg", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Storage Error", "Failed to store key file: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, OpticalKeysUploadResponse{
		Status:    "active",
		KeysCount: count,
		Message:   "KEYDB.cfg uploaded successfully",
	})
}

// handleEnrichmentStatus returns current media metadata and artwork enrichment engine state.
func (s *Server) handleEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
	if s.enrichmentEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Enrichment Engine Unavailable", "Enrichment engine is not initialized", r.URL.Path)
		return
	}

	status := s.enrichmentEngine.GetStatus()
	s.writeJSON(w, http.StatusOK, status)
}

// handleEnrichmentScan initiates an immediate scan and enrichment of staged optical discs.
func (s *Server) handleEnrichmentScan(w http.ResponseWriter, r *http.Request) {
	if s.enrichmentEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Enrichment Engine Unavailable", "Enrichment engine is not initialized", r.URL.Path)
		return
	}

	count, err := s.enrichmentEngine.Scan(r.Context())
	if err != nil {
		s.logger.Error("enrichment scan failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Scan Failed", "Enrichment scan encountered an error: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, EnrichmentScanResponse{
		Status:         "ok",
		Message:        fmt.Sprintf("Enrichment scan completed successfully: %d items processed", count),
		ItemsProcessed: count,
	})
}

// handleTranscodeQueue lists all jobs currently in the transcoding queue.
func (s *Server) handleTranscodeQueue(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	jobs, err := s.transcodeEngine.GetQueue(r.Context())
	if err != nil {
		s.logger.Error("failed to list transcode queue", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Queue Query Failed", err.Error(), r.URL.Path)
		return
	}

	st := s.transcodeEngine.GetStatus()
	resp := TranscodeQueueResponse{
		Jobs:        make([]TranscodeJobResponse, 0, len(jobs)),
		Total:       len(jobs),
		ActiveJobID: st.ActiveJobID,
	}

	for _, j := range jobs {
		item := TranscodeJobResponse{
			ID:           j.ID,
			SourcePath:   j.SourcePath,
			TargetPath:   j.TargetPath,
			Profile:      j.Profile,
			Status:       string(j.Status),
			Progress:     j.Progress,
			EncoderUsed:  j.EncoderUsed,
			ErrorMessage: j.ErrorMessage,
			CreatedAt:    j.CreatedAt.Format(time.RFC3339),
		}
		if j.StartedAt != nil {
			item.StartedAt = j.StartedAt.Format(time.RFC3339)
		}
		if j.CompletedAt != nil {
			item.CompletedAt = j.CompletedAt.Format(time.RFC3339)
		}
		resp.Jobs = append(resp.Jobs, item)
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleEnqueueTranscode enqueues a media file for background off-peak transcoding.
func (s *Server) handleEnqueueTranscode(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req EnqueueTranscodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse request body: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.SourcePath) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Missing Source Path", "source_path is required", r.URL.Path)
		return
	}

	job, err := s.transcodeEngine.Enqueue(r.Context(), transcode.EnqueueRequest{
		SourcePath: req.SourcePath,
		TargetPath: req.TargetPath,
		Profile:    req.Profile,
	})
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid File Path", err.Error(), r.URL.Path)
		return
	}

	resp := TranscodeJobResponse{
		ID:           job.ID,
		SourcePath:   job.SourcePath,
		TargetPath:   job.TargetPath,
		Profile:      job.Profile,
		Status:       string(job.Status),
		Progress:     job.Progress,
		EncoderUsed:  job.EncoderUsed,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt.Format(time.RFC3339),
	}

	s.writeJSON(w, http.StatusCreated, resp)
}

// handleEnqueueMobile enqueues a media file for mobile direct-play profile generation.
func (s *Server) handleEnqueueMobile(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req EnqueueMobileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse request body: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.SourcePath) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Missing Source Path", "source_path is required", r.URL.Path)
		return
	}

	targetProfile := strings.TrimSpace(req.TargetProfile)
	if targetProfile == "" {
		targetProfile = strings.TrimSpace(req.Profile)
	}
	if targetProfile == "" {
		s.writeProblem(w, http.StatusBadRequest, "Missing Target Profile", "target_profile is required (e.g. 'mobile-720p' or 'mobile-1080p')", r.URL.Path)
		return
	}

	if !transcode.IsMobileProfile(targetProfile) {
		s.writeProblem(w, http.StatusBadRequest, "Unknown Mobile Profile", fmt.Sprintf("target_profile %q is not supported: must be 'mobile-720p' or 'mobile-1080p'", targetProfile), r.URL.Path)
		return
	}

	job, isExisting, err := s.transcodeEngine.EnqueueMobile(r.Context(), transcode.MobileEnqueueRequest{
		SourcePath:    req.SourcePath,
		TargetProfile: targetProfile,
	})
	if err != nil {
		if errors.Is(err, transcode.ErrUnknownMobileProfile) ||
			errors.Is(err, transcode.ErrInvalidSourcePath) ||
			errors.Is(err, transcode.ErrIdenticalPaths) ||
			errors.Is(err, transcode.ErrUnsupportedVideoFormat) ||
			errors.Is(err, transcode.ErrSourceOutsideDataDir) ||
			errors.Is(err, transcode.ErrSourceNotFound) ||
			errors.Is(err, transcode.ErrSourceIsDir) ||
			strings.Contains(err.Error(), "required") ||
			strings.Contains(err.Error(), "unsupported") ||
			strings.Contains(err.Error(), "cannot be empty") {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Mobile Request", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("failed to enqueue mobile transcode job", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Enqueue Failed", err.Error(), r.URL.Path)
		return
	}

	resp := TranscodeJobResponse{
		ID:           job.ID,
		SourcePath:   job.SourcePath,
		TargetPath:   job.TargetPath,
		Profile:      job.Profile,
		Status:       string(job.Status),
		Progress:     job.Progress,
		EncoderUsed:  job.EncoderUsed,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt.Format(time.RFC3339),
	}
	if job.StartedAt != nil {
		resp.StartedAt = job.StartedAt.Format(time.RFC3339)
	}
	if job.CompletedAt != nil {
		resp.CompletedAt = job.CompletedAt.Format(time.RFC3339)
	}

	statusCode := http.StatusCreated
	if isExisting {
		statusCode = http.StatusOK
	}

	s.writeJSON(w, statusCode, resp)
}

// handleGetTranscodeSchedule returns current off-peak schedule configuration and live status.
func (s *Server) handleGetTranscodeSchedule(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	sched := s.transcodeEngine.Scheduler()
	cfg := sched.GetConfig()
	resp := TranscodeScheduleResponse{
		StartHour:      cfg.StartHour,
		StartMinute:    cfg.StartMinute,
		EndHour:        cfg.EndHour,
		EndMinute:      cfg.EndMinute,
		IsOffPeak:      sched.IsOffPeakNow(),
		ManualOverride: sched.ManualOverride(),
		Status:         sched.GetGPUStatusNow(),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleUpdateTranscodeSchedule updates off-peak window hours and minutes.
func (s *Server) handleUpdateTranscodeSchedule(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req UpdateTranscodeScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse schedule config: "+err.Error(), r.URL.Path)
		return
	}

	sched := s.transcodeEngine.Scheduler()
	currentCfg := sched.GetConfig()
	currentCfg.StartHour = req.StartHour
	currentCfg.StartMinute = req.StartMinute
	currentCfg.EndHour = req.EndHour
	currentCfg.EndMinute = req.EndMinute

	if err := sched.SetConfig(currentCfg); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Schedule", err.Error(), r.URL.Path)
		return
	}

	resp := TranscodeScheduleResponse{
		StartHour:      currentCfg.StartHour,
		StartMinute:    currentCfg.StartMinute,
		EndHour:        currentCfg.EndHour,
		EndMinute:      currentCfg.EndMinute,
		IsOffPeak:      sched.IsOffPeakNow(),
		ManualOverride: sched.ManualOverride(),
		Status:         sched.GetGPUStatusNow(),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleTranscodeOverride enables or disables manual override bypassing daytime schedule restrictions.
func (s *Server) handleTranscodeOverride(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req TranscodeOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse override body: "+err.Error(), r.URL.Path)
		return
	}

	s.transcodeEngine.SetManualOverride(req.Force)

	statusStr := "inactive"
	msg := "Manual override disabled; daytime AI priority restored"
	if req.Force {
		statusStr = "active"
		msg = "Manual override enabled; daytime batch transcoding active"
	}

	s.writeJSON(w, http.StatusOK, TranscodeOverrideResponse{
		Status:         statusStr,
		ManualOverride: req.Force,
		Message:        msg,
	})
}

// IMAPConfigResponse matches the OpenAPI 3.1 contract.
type IMAPConfigResponse struct {
	ID           string `json:"id"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Folder       string `json:"folder"`
	SSLEnabled   bool   `json:"ssl_enabled"`
	Enabled      bool   `json:"enabled"`
	Connected    bool   `json:"connected"`
	LastPolledAt string `json:"last_polled_at,omitempty"`
	LastError    string `json:"last_error"`
}

// IMAPConfigSaveRequest matches the OpenAPI 3.1 contract.
type IMAPConfigSaveRequest struct {
	ID         string `json:"id,omitempty"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Folder     string `json:"folder"`
	SSLEnabled *bool  `json:"ssl_enabled,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

// IMAPTestRequest matches the OpenAPI 3.1 contract.
type IMAPTestRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Folder     string `json:"folder"`
	SSLEnabled *bool  `json:"ssl_enabled,omitempty"`
}

// IMAPTestResponse matches the OpenAPI 3.1 contract.
type IMAPTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// handleDocumentUpload handles multipart file uploads to /api/v1/documents/upload.
func (s *Server) handleDocumentUpload(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.ingestEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Document ingest engine is not initialized", r.URL.Path)
		return
	}

	engine.IngestHTTPHandler(s.writeProblem, s.writeJSON)(w, r)
}

// handleGetIMAPConfig returns the active IMAP mailbox configuration with masked password.
func (s *Server) handleGetIMAPConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	imapStore := s.imapStore
	s.mu.RUnlock()

	if imapStore == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "IMAP store is not initialized", r.URL.Path)
		return
	}

	cfg, err := imapStore.GetFirst(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrIMAPConfigNotFound) {
			s.writeJSON(w, http.StatusOK, IMAPConfigResponse{
				ID:         "",
				Host:       "",
				Port:       993,
				Username:   "",
				Password:   "",
				Folder:     "INBOX",
				SSLEnabled: true,
				Enabled:    false,
				Connected:  false,
				LastError:  "",
			})
			return
		}
		s.logger.Error("failed to get imap config", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve IMAP configuration: "+err.Error(), r.URL.Path)
		return
	}

	lastPolledStr := ""
	if cfg.LastPolledAt != nil {
		lastPolledStr = cfg.LastPolledAt.UTC().Format(time.RFC3339)
	}

	connected := cfg.Enabled && cfg.LastError == "" && cfg.LastPolledAt != nil

	resp := IMAPConfigResponse{
		ID:           cfg.ID,
		Host:         cfg.Host,
		Port:         cfg.Port,
		Username:     cfg.Username,
		Password:     "********",
		Folder:       cfg.Folder,
		SSLEnabled:   cfg.SSLEnabled,
		Enabled:      cfg.Enabled,
		Connected:    connected,
		LastPolledAt: lastPolledStr,
		LastError:    cfg.LastError,
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleSaveIMAPConfig creates or updates an IMAP mailbox configuration with encrypted credentials.
func (s *Server) handleSaveIMAPConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	imapStore := s.imapStore
	poller := s.imapPoller
	s.mu.RUnlock()

	if imapStore == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "IMAP store is not initialized", r.URL.Path)
		return
	}

	var req IMAPConfigSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Request body must be valid JSON: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Host) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'host' cannot be empty", r.URL.Path)
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'username' cannot be empty", r.URL.Path)
		return
	}
	if req.Port <= 0 {
		req.Port = 993
	}
	if strings.TrimSpace(req.Folder) == "" {
		req.Folder = "INBOX"
	}

	sslEnabled := true
	if req.SSLEnabled != nil {
		sslEnabled = *req.SSLEnabled
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Check if this is an update and password is masked
	existingPass := ""
	if req.ID != "" {
		if existing, err := imapStore.Get(r.Context(), req.ID); err == nil && existing != nil {
			existingPass = existing.Password
		}
	} else {
		if existing, err := imapStore.GetActive(r.Context()); err == nil && existing != nil {
			req.ID = existing.ID
			existingPass = existing.Password
		}
	}

	passwordToSave := req.Password
	if passwordToSave == "********" || passwordToSave == "" {
		passwordToSave = existingPass
	}

	if passwordToSave == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'password' cannot be empty", r.URL.Path)
		return
	}

	configToSave := &store.IMAPConfig{
		ID:         req.ID,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   passwordToSave,
		Folder:     req.Folder,
		SSLEnabled: sslEnabled,
		Enabled:    enabled,
	}

	if err := imapStore.Save(r.Context(), configToSave); err != nil {
		s.logger.Error("failed to save imap config", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to save IMAP configuration: "+err.Error(), r.URL.Path)
		return
	}

	// Trigger immediate background poll if enabled
	if poller != nil && enabled {
		go func() {
			_ = poller.PollOnce(context.Background())
		}()
	}

	resp := IMAPConfigResponse{
		ID:         configToSave.ID,
		Host:       configToSave.Host,
		Port:       configToSave.Port,
		Username:   configToSave.Username,
		Password:   "********",
		Folder:     configToSave.Folder,
		SSLEnabled: configToSave.SSLEnabled,
		Enabled:    configToSave.Enabled,
		Connected:  configToSave.Enabled && configToSave.LastError == "" && configToSave.LastPolledAt != nil,
		LastError:  "",
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleTestIMAPConnection tests connectivity and credentials with the specified IMAP server.
func (s *Server) handleTestIMAPConnection(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	imapStore := s.imapStore
	s.mu.RUnlock()

	var req IMAPTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Request body must be valid JSON: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Host) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'host' cannot be empty", r.URL.Path)
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'username' cannot be empty", r.URL.Path)
		return
	}
	if req.Port <= 0 {
		req.Port = 993
	}
	if strings.TrimSpace(req.Folder) == "" {
		req.Folder = "INBOX"
	}

	sslEnabled := true
	if req.SSLEnabled != nil {
		sslEnabled = *req.SSLEnabled
	}

	passwordToTest := req.Password
	if passwordToTest == "" || passwordToTest == "********" {
		if imapStore != nil {
			if active, err := imapStore.GetFirst(r.Context()); err == nil && active != nil {
				passwordToTest = active.Password
			}
		}
	}

	if passwordToTest == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'password' cannot be empty", r.URL.Path)
		return
	}

	client := documents.NewClient()
	defer func() {
		_ = client.Close()
	}()

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	tlsConfig := &tls.Config{
		ServerName: req.Host,
	}

	testCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := client.Connect(testCtx, addr, sslEnabled, tlsConfig); err != nil {
		s.logger.Warn("imap connection test failed", "host", req.Host, "error", err)
		if s.serviceMgr != nil {
			s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "degraded", documents.ReassuranceMailboxDisconnected)
		}
		s.writeProblem(w, http.StatusBadGateway, "Mailbox Connection Failed", documents.ReassuranceMailboxDisconnected, r.URL.Path)
		return
	}

	if err := client.Login(testCtx, req.Username, passwordToTest); err != nil {
		s.logger.Warn("imap login test failed", "host", req.Host, "username", req.Username, "error", err)
		if s.serviceMgr != nil {
			s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "degraded", documents.ReassuranceMailboxDisconnected)
		}
		s.writeProblem(w, http.StatusBadGateway, "Mailbox Connection Failed", documents.ReassuranceMailboxDisconnected, r.URL.Path)
		return
	}

	if _, err := client.Select(testCtx, req.Folder); err != nil {
		s.logger.Warn("imap folder select test failed", "folder", req.Folder, "error", err)
		if s.serviceMgr != nil {
			s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "degraded", documents.ReassuranceMailboxDisconnected)
		}
		s.writeProblem(w, http.StatusBadGateway, "Mailbox Connection Failed", documents.ReassuranceMailboxDisconnected, r.URL.Path)
		return
	}

	// On successful connection test, restore Paperless health if it was degraded
	if s.serviceMgr != nil {
		s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "healthy", "Automatic Text Search Active · Mailbox Connected")
	}

	s.writeJSON(w, http.StatusOK, IMAPTestResponse{
		Success: true,
		Message: "Mailbox connection successful",
	})
}

// IMAPStore returns the active IMAP store.
func (s *Server) IMAPStore() *store.IMAPStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.imapStore
}

// IMAPPoller returns the active IMAP poller.
func (s *Server) IMAPPoller() *documents.Poller {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.imapPoller
}

// IngestEngine returns the active document ingestion engine.
func (s *Server) IngestEngine() *documents.IngestEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ingestEngine
}

// DeadlineStore returns the active document deadline repository.
func (s *Server) DeadlineStore() *store.DeadlineStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deadlineStore
}

// ObligationScanner returns the active obligation scanner coordinator.
func (s *Server) ObligationScanner() *documents.ObligationScanner {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.obligationScanner
}

// handleGetDeadlines handles GET /api/v1/documents/deadlines.
func (s *Server) handleGetDeadlines(w http.ResponseWriter, r *http.Request) {
	if s.deadlineStore == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Store Unavailable", "Deadline store is not initialized", r.URL.Path)
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.DeadlineStatusActive
	}

	withinDays := 0
	if status == store.DeadlineStatusPending || status == store.DeadlineStatusActive {
		withinDays = 14
	}
	if q := r.URL.Query().Get("within_days"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			withinDays = parsed
		}
	}

	deadlines, err := s.deadlineStore.List(r.Context(), status, withinDays, time.Now())
	if err != nil {
		s.writeProblem(w, http.StatusInternalServerError, "Query Failed", fmt.Sprintf("Failed to list deadlines: %v", err), r.URL.Path)
		return
	}

	if deadlines == nil {
		deadlines = []*store.DocumentDeadline{}
	}

	s.writeJSON(w, http.StatusOK, deadlines)
}

// handleDismissDeadline handles POST /api/v1/documents/deadlines/{id}/dismiss.
func (s *Server) handleDismissDeadline(w http.ResponseWriter, r *http.Request) {
	if s.deadlineStore == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Store Unavailable", "Deadline store is not initialized", r.URL.Path)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid ID", "Deadline ID is required in path", r.URL.Path)
		return
	}

	updated, err := s.deadlineStore.Dismiss(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrDeadlineNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Not Found", fmt.Sprintf("Deadline with ID %q was not found", id), r.URL.Path)
			return
		}
		s.writeProblem(w, http.StatusInternalServerError, "Dismissal Failed", fmt.Sprintf("Failed to dismiss deadline: %v", err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, updated)
}

// ScanDeadlinesRequest matches the OpenAPI contract for manual obligation scan trigger.
type ScanDeadlinesRequest struct {
	Force      bool   `json:"force"`
	DocumentID string `json:"document_id,omitempty"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
}

// ScanDeadlinesResponse matches the OpenAPI contract for manual obligation scan results.
type ScanDeadlinesResponse struct {
	Status         string                    `json:"status"`
	ScannedCount   int                       `json:"scanned_count"`
	DeadlinesFound int                       `json:"deadlines_found"`
	Deadlines      []*store.DocumentDeadline `json:"deadlines"`
}

// handleScanDeadlines handles POST /api/v1/documents/deadlines/scan.
func (s *Server) handleScanDeadlines(w http.ResponseWriter, r *http.Request) {
	if s.obligationScanner == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Scanner Unavailable", "Obligation scanner is not initialized", r.URL.Path)
		return
	}

	var req ScanDeadlinesRequest
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err == nil && len(strings.TrimSpace(string(bodyBytes))) > 0 {
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				s.writeProblem(w, http.StatusBadRequest, "Invalid Request", fmt.Sprintf("Failed to decode scan request: %v", err), r.URL.Path)
				return
			}
		}
	}

	var found []*store.DocumentDeadline
	scannedCount := 0

	if strings.TrimSpace(req.Text) != "" {
		docID := req.DocumentID
		if docID == "" {
			docID = "manual-scan"
		}
		title := req.Title
		if title == "" {
			title = "Scanned Document"
		}
		results, err := s.obligationScanner.ScanDocumentText(r.Context(), docID, title, req.Text, req.Force)
		if err != nil {
			s.writeProblem(w, http.StatusInternalServerError, "Scan Failed", fmt.Sprintf("Document text scan failed: %v", err), r.URL.Path)
			return
		}
		found = append(found, results...)
		scannedCount = 1
	} else {
		// Run batch pass over queued/pending documents
		processed, err := s.obligationScanner.ProcessNightlyBatch(r.Context())
		if err != nil {
			s.logger.Warn("batch scan completed with warnings", "error", err)
		}
		scannedCount = processed
		if s.deadlineStore != nil {
			list, err := s.deadlineStore.List(r.Context(), store.DeadlineStatusPending, s.obligationScanner.AlertThreshold(), time.Now())
			if err == nil {
				found = list
			}
		}
	}

	if found == nil {
		found = []*store.DocumentDeadline{}
	}

	s.writeJSON(w, http.StatusOK, ScanDeadlinesResponse{
		Status:         "completed",
		ScannedCount:   scannedCount,
		DeadlinesFound: len(found),
		Deadlines:      found,
	})
}

// AlertConfigResponse matches OpenAPI contract for GET /api/v1/alerts/config.
type AlertConfigResponse = notify.AlertConfig

// UpdateAlertConfigRequest matches OpenAPI contract for PUT /api/v1/alerts/config.
type UpdateAlertConfigRequest struct {
	HAWebhookURL           string `json:"ha_webhook_url"`
	MQTTBrokerHost         string `json:"mqtt_broker_host"`
	MQTTBrokerPort         int    `json:"mqtt_broker_port"`
	MQTTTopic              string `json:"mqtt_topic"`
	MQTTUsername           string `json:"mqtt_username"`
	MQTTPassword           string `json:"mqtt_password"`
	AlertEmail             string `json:"alert_email"`
	SMTPHost               string `json:"smtp_host"`
	SMTPPort               int    `json:"smtp_port"`
	SMTPUsername           string `json:"smtp_username"`
	SMTPPassword           string `json:"smtp_password"`
	SMTPFrom               string `json:"smtp_from"`
	DeadlineAlertThreshold int    `json:"deadline_alert_threshold"`
}

// TestWebhookRequest matches OpenAPI contract for POST /api/v1/alerts/test/webhook.
type TestWebhookRequest struct {
	WebhookURL string `json:"webhook_url,omitempty"`
}

// TestMQTTRequest matches OpenAPI contract for POST /api/v1/alerts/test/mqtt.
type TestMQTTRequest struct {
	BrokerHost string `json:"broker_host,omitempty"`
	BrokerPort int    `json:"broker_port,omitempty"`
	Topic      string `json:"topic,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
}

// TestEmailRequest matches OpenAPI contract for POST /api/v1/alerts/test/email.
type TestEmailRequest struct {
	AlertEmail   string `json:"alert_email,omitempty"`
	SMTPHost     string `json:"smtp_host,omitempty"`
	SMTPPort     int    `json:"smtp_port,omitempty"`
	SMTPUsername string `json:"smtp_username,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"`
	SMTPFrom     string `json:"smtp_from,omitempty"`
}

// AlertTestResponse matches OpenAPI contract for alert test endpoints.
type AlertTestResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// AlertDispatchResponse matches OpenAPI contract for POST /api/v1/alerts/dispatch.
type AlertDispatchResponse struct {
	Status          string `json:"status"`
	DispatchedCount int    `json:"dispatched_count"`
	Message         string `json:"message"`
}

// AlertDispatcher returns the active alert dispatcher.
func (s *Server) AlertDispatcher() *notify.AlertDispatcher {
	return s.alertDispatcher
}

// handleGetAlertConfig handles GET /api/v1/alerts/config.
func (s *Server) handleGetAlertConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database not initialized", r.URL.Path)
		return
	}
	cfg, err := notify.LoadConfig(r.Context(), s.db)
	if err != nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to load alert configuration: "+err.Error(), r.URL.Path)
		return
	}
	s.writeJSON(w, http.StatusOK, cfg.Redacted())
}

// handleUpdateAlertConfig handles PUT /api/v1/alerts/config.
func (s *Server) handleUpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database not initialized", r.URL.Path)
		return
	}
	var req UpdateAlertConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Failed to decode request body: "+err.Error(), r.URL.Path)
		return
	}

	current, err := notify.LoadConfig(r.Context(), s.db)
	if err != nil || current == nil {
		current = &notify.AlertConfig{
			MQTTBrokerPort:         notify.DefaultMQTTBrokerPort,
			MQTTTopic:              notify.DefaultMQTTTopic,
			SMTPPort:               notify.DefaultSMTPPort,
			SMTPFrom:               notify.DefaultSMTPFrom,
			DeadlineAlertThreshold: notify.DefaultDeadlineAlertThreshold,
		}
	}

	if req.HAWebhookURL != "" {
		current.HAWebhookURL = req.HAWebhookURL
	}
	if req.MQTTBrokerHost != "" {
		current.MQTTBrokerHost = req.MQTTBrokerHost
	}
	if req.MQTTBrokerPort > 0 {
		current.MQTTBrokerPort = req.MQTTBrokerPort
	}
	if req.MQTTTopic != "" {
		current.MQTTTopic = req.MQTTTopic
	}
	if req.MQTTUsername != "" {
		current.MQTTUsername = req.MQTTUsername
	}
	if req.MQTTPassword != "" {
		current.MQTTPassword = req.MQTTPassword
	}
	if req.AlertEmail != "" {
		current.AlertEmail = req.AlertEmail
	}
	if req.SMTPHost != "" {
		current.SMTPHost = req.SMTPHost
	}
	if req.SMTPPort > 0 {
		current.SMTPPort = req.SMTPPort
	}
	if req.SMTPUsername != "" {
		current.SMTPUsername = req.SMTPUsername
	}
	if req.SMTPPassword != "" {
		current.SMTPPassword = req.SMTPPassword
	}
	if req.SMTPFrom != "" {
		current.SMTPFrom = req.SMTPFrom
	}
	if req.DeadlineAlertThreshold > 0 {
		current.DeadlineAlertThreshold = req.DeadlineAlertThreshold
	}

	if err := notify.SaveConfig(r.Context(), s.db, current); err != nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to save alert configuration: "+err.Error(), r.URL.Path)
		return
	}

	if s.obligationScanner != nil && current.DeadlineAlertThreshold > 0 {
		s.obligationScanner.SetAlertThreshold(current.DeadlineAlertThreshold)
	}

	saved, err := notify.LoadConfig(r.Context(), s.db)
	if err != nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to reload alert configuration: "+err.Error(), r.URL.Path)
		return
	}
	s.writeJSON(w, http.StatusOK, saved.Redacted())
}

// handleTestWebhook handles POST /api/v1/alerts/test/webhook.
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	if s.alertDispatcher == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Alert dispatcher not initialized", r.URL.Path)
		return
	}

	var req TestWebhookRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	targetURL := strings.TrimSpace(req.WebhookURL)
	if targetURL == "" && s.db != nil {
		cfg, _ := notify.LoadConfig(r.Context(), s.db)
		if cfg != nil {
			targetURL = cfg.HAWebhookURL
		}
	}

	if targetURL == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Home Assistant webhook URL is not configured", r.URL.Path)
		return
	}

	if err := s.alertDispatcher.TestWebhook(r.Context(), targetURL); err != nil {
		s.logger.Warn("webhook test failed", "url", targetURL, "error", err)
		s.writeProblem(w, http.StatusBadGateway, "Webhook Delivery Failed", "Unable to deliver webhook notification: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, AlertTestResponse{
		Status:  "success",
		Message: "Home Assistant webhook notification delivered successfully",
	})
}

// handleTestMQTT handles POST /api/v1/alerts/test/mqtt.
func (s *Server) handleTestMQTT(w http.ResponseWriter, r *http.Request) {
	if s.alertDispatcher == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Alert dispatcher not initialized", r.URL.Path)
		return
	}

	var req TestMQTTRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	host := strings.TrimSpace(req.BrokerHost)
	port := req.BrokerPort
	topic := strings.TrimSpace(req.Topic)
	username := strings.TrimSpace(req.Username)
	password := req.Password

	if s.db != nil {
		cfg, _ := notify.LoadConfig(r.Context(), s.db)
		if cfg != nil {
			if host == "" {
				host = cfg.MQTTBrokerHost
			}
			if port <= 0 {
				port = cfg.MQTTBrokerPort
			}
			if topic == "" {
				topic = cfg.MQTTTopic
			}
			if username == "" {
				username = cfg.MQTTUsername
			}
			if password == "" || password == notify.PasswordMask {
				password = cfg.MQTTPassword
			}
		}
	}

	if host == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "MQTT broker host is not configured", r.URL.Path)
		return
	}
	if port <= 0 {
		port = notify.DefaultMQTTBrokerPort
	}
	if topic == "" {
		topic = notify.DefaultMQTTTopic
	}

	if err := s.alertDispatcher.TestMQTT(r.Context(), host, port, topic, username, password); err != nil {
		s.logger.Warn("mqtt test failed", "host", host, "port", port, "error", err)
		s.writeProblem(w, http.StatusBadGateway, "MQTT Test Failed", "Unable to publish test alert to MQTT broker: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, AlertTestResponse{
		Status:  "success",
		Message: "MQTT test alert published successfully",
	})
}

// handleTestEmail handles POST /api/v1/alerts/test/email.
func (s *Server) handleTestEmail(w http.ResponseWriter, r *http.Request) {
	if s.alertDispatcher == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Alert dispatcher not initialized", r.URL.Path)
		return
	}

	var req TestEmailRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	alertEmail := strings.TrimSpace(req.AlertEmail)
	host := strings.TrimSpace(req.SMTPHost)
	port := req.SMTPPort
	username := strings.TrimSpace(req.SMTPUsername)
	password := req.SMTPPassword
	from := strings.TrimSpace(req.SMTPFrom)

	if s.db != nil {
		cfg, _ := notify.LoadConfig(r.Context(), s.db)
		if cfg != nil {
			if alertEmail == "" {
				alertEmail = cfg.AlertEmail
			}
			if host == "" {
				host = cfg.SMTPHost
			}
			if port <= 0 {
				port = cfg.SMTPPort
			}
			if username == "" {
				username = cfg.SMTPUsername
			}
			if password == "" || password == notify.PasswordMask {
				password = cfg.SMTPPassword
			}
			if from == "" {
				from = cfg.SMTPFrom
			}
		}
	}

	if alertEmail == "" || host == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "SMTP host or alert email recipient is not configured", r.URL.Path)
		return
	}
	if port <= 0 {
		port = notify.DefaultSMTPPort
	}
	if from == "" {
		from = notify.DefaultSMTPFrom
	}

	if err := s.alertDispatcher.TestEmail(r.Context(), alertEmail, host, port, username, password, from); err != nil {
		s.logger.Warn("smtp test failed", "host", host, "to", alertEmail, "error", err)
		s.writeProblem(w, http.StatusBadGateway, "Email Test Failed", "Unable to deliver test email: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, AlertTestResponse{
		Status:  "success",
		Message: "Test email sent successfully",
	})
}

// handleDispatchAlerts handles POST /api/v1/alerts/dispatch.
func (s *Server) handleDispatchAlerts(w http.ResponseWriter, r *http.Request) {
	if s.alertDispatcher == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Alert dispatcher not initialized", r.URL.Path)
		return
	}

	count, err := s.alertDispatcher.DispatchCycle(r.Context())
	if err != nil {
		s.logger.Error("manual alert dispatch cycle failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Dispatch Failed", "Alert dispatch cycle encountered an error: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, AlertDispatchResponse{
		Status:          "success",
		DispatchedCount: count,
		Message:         "Alert evaluation and dispatch cycle completed",
	})
}




