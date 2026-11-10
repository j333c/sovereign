package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AppVersion = "0.1.0"
)

// ProblemDetails implements RFC 7807 error responses.
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
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
	AlertQueue        *store.AlertQueueStore
	LlamaProvisioner  *supervisor.LlamaProvisioner
	AIGateway         ai.Gateway
	LlamaURL          string
	PersonaStore      *ai.PersonaStore
	RAGEngine         *ai.RAGEngine
	ModelManager      *ai.ModelManager
	DiskFree          ai.DiskFreeFunc
	DockerSocketPath  string
	RepoDir           string
	UpdateEngine      *supervisor.UpdateDiscoveryEngine
}

// Server is the appliance core supervisor HTTP daemon.
type Server struct {
	cfg               Config
	dockerSocketPath  string
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
	personaStore      *ai.PersonaStore
	ragEngine         *ai.RAGEngine
	modelManager      *ai.ModelManager
	updateEngine      *supervisor.UpdateDiscoveryEngine
	stopChan          chan struct{}
	closeOnce         sync.Once
	mu                sync.RWMutex

	eventsMu         sync.RWMutex
	eventSubscribers map[chan sseEvent]struct{}
}

type serviceReloaderAdapter struct {
	mgr *supervisor.ServiceManager
}

func (a *serviceReloaderAdapter) ReloadService(ctx context.Context, id string) error {
	if a.mgr != nil {
		st, err := a.mgr.GetService(ctx, id)
		if err == nil && st != nil && (st.Status == supervisor.StatusInactive || st.Status == supervisor.StatusStopped) {
			_, err = a.mgr.ActivateService(ctx, id)
			return err
		}
		_, err = a.mgr.RestartService(ctx, id)
		return err
	}
	return nil
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

	sockPath := cfg.DockerSocketPath
	if sockPath == "" {
		sockPath = "/var/run/docker.sock"
	}
	s.dockerSocketPath = sockPath

	dockerFound := false
	if fi, err := os.Stat(sockPath); err == nil && fi != nil && (fi.Mode()&os.ModeSocket != 0) {
		dockerFound = true
	} else {
		cfg.Logger.Warn("Docker socket unavailable; container operations paused",
			"socket", sockPath,
			"status", "degraded",
			"reassurance", "Container engine unavailable — background tasks paused",
		)
	}

	if cfg.ServiceMgr == nil {
		var engine supervisor.RuntimeEngine
		if dockerFound {
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
			RevisionStore:    s.db,
		})
	} else {
		s.serviceMgr = cfg.ServiceMgr
		s.serviceMgr.SetEventBroadcaster(s.BroadcastEvent)
		if s.serviceMgr.RevisionStore() == nil && s.db != nil {
			s.serviceMgr.SetRevisionStore(s.db)
		}
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

	if cfg.PersonaStore == nil {
		openwebuiStateDir := filepath.Join(cfg.DataDir, "state", "services", "openwebui")
		s.personaStore = ai.NewPersonaStore(openwebuiStateDir, cfg.Logger)
	} else {
		s.personaStore = cfg.PersonaStore
	}

	if cfg.RAGEngine == nil {
		var client ai.InferenceClient
		if s.aiGateway != nil {
			client = ai.NewGatewayInferenceClient(s.aiGateway)
		}
		s.ragEngine = ai.NewRAGEngine(ai.RAGConfig{
			DataDir:         cfg.DataDir,
			Logger:          cfg.Logger,
			InferenceClient: client,
		})
	} else {
		s.ragEngine = cfg.RAGEngine
	}

	if cfg.ModelManager == nil {
		modelsDir := filepath.Join(cfg.DataDir, "models")
		var arb *ai.GPUArbitrator
		if s.aiGateway != nil {
			arb = s.aiGateway.Arbitrator()
		}
		var reloader ai.ServiceReloader
		if s.serviceMgr != nil {
			reloader = &serviceReloaderAdapter{mgr: s.serviceMgr}
		}
		s.modelManager = ai.NewModelManager(ai.ModelManagerConfig{
			ModelsDir:  modelsDir,
			Store:      s.db,
			Reloader:   reloader,
			Arbitrator: arb,
			Logger:     cfg.Logger,
			DiskFree:   cfg.DiskFree,
		})
	} else {
		s.modelManager = cfg.ModelManager
	}

	if s.llamaProvisioner != nil && s.db != nil {
		s.llamaProvisioner.SetSettingStore(s.db)
	}

	if s.db != nil && s.aiGateway != nil && s.aiGateway.Arbitrator() != nil {
		if timeoutStr, err := s.db.GetSetting(context.Background(), "ai_idle_timeout_seconds"); err == nil && timeoutStr != "" {
			if sec, pErr := strconv.Atoi(timeoutStr); pErr == nil && sec > 0 {
				s.aiGateway.Arbitrator().SetIdleTimeout(time.Duration(sec) * time.Second)
			}
		}
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
				Logger:     cfg.Logger,
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

	// 13. Initialize AlertDispatcher and wire into ObligationScanner & ServiceManager
	if cfg.AlertDispatcher == nil && db != nil {
		cfg.AlertDispatcher = notify.NewAlertDispatcher(db, s.deadlineStore, cfg.Logger)
	}
	s.alertDispatcher = cfg.AlertDispatcher
	if s.alertDispatcher != nil && cfg.AlertQueue != nil {
		s.alertDispatcher.SetAlertQueue(cfg.AlertQueue)
	}
	if s.obligationScanner != nil && s.alertDispatcher != nil {
		s.obligationScanner.SetAlertDispatcher(s.alertDispatcher)
	}
	if s.serviceMgr != nil && s.alertDispatcher != nil {
		s.serviceMgr.SetAlertNotifier(s.alertDispatcher)
	}
	if s.obligationScanner != nil && db != nil {
		if nCfg, err := notify.LoadConfig(context.Background(), db); err == nil && nCfg != nil && nCfg.DeadlineAlertThreshold > 0 {
			s.obligationScanner.SetAlertThreshold(nCfg.DeadlineAlertThreshold)
		}
	}

	// Update channel seeding: check SQLite, fallback to /data/state/update_channel, default to "stable"
	updateChannel := ""
	if db != nil {
		updateChannel, _ = db.GetSetting(context.Background(), "update_channel")
		if updateChannel == "" {
			seedPath := filepath.Join(cfg.DataDir, "state", "update_channel")
			if raw, err := os.ReadFile(seedPath); err == nil {
				updateChannel = strings.TrimSpace(string(raw))
			}
			if updateChannel == "" {
				updateChannel = supervisor.ChannelStable
			}
			_ = db.SetSetting(context.Background(), "update_channel", updateChannel)
		}
	}
	if updateChannel == "" {
		updateChannel = supervisor.ChannelStable
	}

	repoDir := cfg.RepoDir
	if repoDir == "" {
		repoDir = os.Getenv("SOVEREIGN_REPO_DIR")
		if repoDir == "" {
			repoDir = "/opt/sovereign"
		}
	}

	updateEngine := cfg.UpdateEngine
	if updateEngine == nil {
		updateEngine = supervisor.NewUpdateDiscoveryEngine(supervisor.UpdateDiscoveryConfig{
			RepoDir:        repoDir,
			CurrentVersion: AppVersion,
			Channel:        updateChannel,
			Logger:         s.logger,
			OnUpdateFound: func(candidate *supervisor.UpdateCandidate) {
				s.BroadcastEvent("system.update_available", map[string]any{
					"target_tag":      candidate.TargetTag,
					"target_version":  candidate.TargetVersion.String(),
					"current_version": candidate.CurrentVersion.String(),
					"channel":         candidate.Channel,
					"changelog":       candidate.Changelog,
				})
			},
		})
	}
	s.updateEngine = updateEngine

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

	// 13. Start background alert dispatch queue retry runner
	if s.alertDispatcher != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-s.stopChan:
					return
				case <-ticker.C:
					retryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
					_, _ = s.alertDispatcher.ProcessQueue(retryCtx)
					cancel()
				}
			}
		}()
	}

	// 14. Start background git-native update discovery engine
	if s.updateEngine != nil {
		s.updateEngine.Start(ctx)
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

	if s.ragEngine != nil {
		_ = s.ragEngine.Close()
	}

	if s.updateEngine != nil {
		s.updateEngine.Stop()
	}

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// UpdateEngine returns the active update discovery engine.
func (s *Server) UpdateEngine() *supervisor.UpdateDiscoveryEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updateEngine
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

// PersonaStore returns the active PersonaStore instance.
func (s *Server) PersonaStore() *ai.PersonaStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.personaStore
}

// SetPersonaStore updates the PersonaStore instance.
func (s *Server) SetPersonaStore(store *ai.PersonaStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.personaStore = store
}

// RAGEngine returns the active document RAG engine.
func (s *Server) RAGEngine() *ai.RAGEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ragEngine
}

// SetRAGEngine updates the document RAG engine.
func (s *Server) SetRAGEngine(engine *ai.RAGEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ragEngine = engine
}

// ModelManager returns the active ModelManager instance.
func (s *Server) ModelManager() *ai.ModelManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelManager
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

func (s *Server) writeProblemWithType(w http.ResponseWriter, problemType string, status int, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	prob := ProblemDetails{
		Type:     problemType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
	_ = json.NewEncoder(w).Encode(prob)
}

func (s *Server) writeProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	problemType := fmt.Sprintf("https://sovereign.local/errors/http-%d", status)
	if status == http.StatusInsufficientStorage {
		problemType = "https://sovereign.local/errors/insufficient-storage"
	}
	s.writeProblemWithType(w, problemType, status, title, detail, instance)
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

// AlertDispatcher returns the active alert dispatcher.
func (s *Server) AlertDispatcher() *notify.AlertDispatcher {
	return s.alertDispatcher
}
