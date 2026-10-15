package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	var (
		listenAddr string
		dataDir    string
		dbPath     string
	)

	defaultDataDir := os.Getenv("SOVEREIGN_DATA_DIR")
	if defaultDataDir == "" {
		defaultDataDir = "/data"
	}

	defaultAddr := os.Getenv("SOVEREIGN_ADDR")
	if defaultAddr == "" {
		defaultAddr = ":8080"
	}

	defaultDBPath := os.Getenv("SOVEREIGN_DB_PATH")

	flag.StringVar(&listenAddr, "addr", defaultAddr, "HTTP service address to listen on")
	flag.StringVar(&dataDir, "data-dir", defaultDataDir, "Root storage directory for appliance data")
	flag.StringVar(&dbPath, "db-path", defaultDBPath, "Explicit SQLite database path (defaults to <data-dir>/state/sovereign.db)")
	flag.Parse()

	logger.Info("starting sovereign-daemon supervisor",
		"version", AppVersion,
		"addr", listenAddr,
		"data_dir", dataDir,
	)

	server, err := NewServer(Config{
		ListenAddr: listenAddr,
		DataDir:    dataDir,
		DBPath:     dbPath,
		Logger:     logger,
	})
	if err != nil {
		logger.Error("fatal initialization error", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	// Start background supervisor routines (mDNS discovery responder, Caddy route sync)
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	server.StartBackgroundServices(bgCtx)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Server shutdown coordination
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("sovereign-daemon HTTP control plane listening", "addr", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server error encountered", "error", err)
		os.Exit(1)
	case sig := <-shutdownSig:
		logger.Info("received shutdown signal, draining connections...", "signal", sig.String())
		bgCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("graceful server shutdown failed, forcing close", "error", err)
			_ = httpServer.Close()
		}
		logger.Info("sovereign-daemon shut down cleanly")
	}
}
