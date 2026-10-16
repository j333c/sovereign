package documents

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sovereign/internal/store"
)

const (
	ReassuranceMailboxDisconnected = "Mailbox disconnected: We couldn't connect to your email account. Please check your email password in Settings."
	EventDocumentIngested          = "dms.document_ingested"
	EventMailboxStatus             = "dms.mailbox_status"
	DefaultPollInterval            = 60 * time.Second
)

// ServiceHealthSetterFn updates a service's runtime health status.
type ServiceHealthSetterFn func(serviceID, status, health, detail string)

// PollerConfig configures the background IMAP poller.
type PollerConfig struct {
	Store               *store.IMAPStore
	ConsumeDir          string
	PollInterval        time.Duration
	Broadcaster         func(event string, data any)
	ServiceHealthSetter ServiceHealthSetterFn
	Logger              *slog.Logger
	ClientFactory       func() IMAPClient
}

// Poller runs periodic IMAP polling to download and queue email attachments into Paperless-ngx.
type Poller struct {
	store               *store.IMAPStore
	consumeDir          string
	pollInterval        time.Duration
	broadcaster         func(event string, data any)
	serviceHealthSetter ServiceHealthSetterFn
	logger              *slog.Logger
	clientFactory       func() IMAPClient

	stopChan  chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
	mu        sync.Mutex
	isPolling bool
}

// NewPoller initializes a Poller with the supplied configuration.
func NewPoller(cfg PollerConfig) (*Poller, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("imap store cannot be nil")
	}
	if cfg.ConsumeDir == "" {
		cfg.ConsumeDir = "/data/documents/consume"
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ClientFactory == nil {
		cfg.ClientFactory = func() IMAPClient {
			return NewClient()
		}
	}

	if err := os.MkdirAll(cfg.ConsumeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create documents consume directory %q: %w", cfg.ConsumeDir, err)
	}

	return &Poller{
		store:               cfg.Store,
		consumeDir:          cfg.ConsumeDir,
		pollInterval:        cfg.PollInterval,
		broadcaster:         cfg.Broadcaster,
		serviceHealthSetter: cfg.ServiceHealthSetter,
		logger:              cfg.Logger,
		clientFactory:       cfg.ClientFactory,
		stopChan:            make(chan struct{}),
	}, nil
}

// Start launches the background polling ticker.
func (p *Poller) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		// Initial poll on startup
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-time.After(2 * time.Second):
			_ = p.PollOnce(ctx)
		}

		ticker := time.NewTicker(p.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopChan:
				return
			case <-ticker.C:
				_ = p.PollOnce(ctx)
			}
		}
	}()
}

// Stop terminates the background polling loop.
func (p *Poller) Stop() {
	p.closeOnce.Do(func() {
		close(p.stopChan)
		p.wg.Wait()
	})
}

// PollOnce executes a single polling iteration across the active IMAP mailbox.
func (p *Poller) PollOnce(ctx context.Context) error {
	p.mu.Lock()
	if p.isPolling {
		p.mu.Unlock()
		return nil
	}
	p.isPolling = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.isPolling = false
		p.mu.Unlock()
	}()

	cfg, err := p.store.GetActive(ctx)
	if err != nil {
		if errors.Is(err, store.ErrIMAPConfigNotFound) {
			return nil
		}
		p.logger.Error("failed to retrieve active imap config", "error", err)
		return err
	}

	if !cfg.Enabled {
		return nil
	}

	client := p.clientFactory()
	defer func() {
		_ = client.Close()
	}()

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	tlsConfig := &tls.Config{
		ServerName: cfg.Host,
	}

	// 1. Connect
	if err := client.Connect(ctx, addr, cfg.SSLEnabled, tlsConfig); err != nil {
		p.handlePollFailure(ctx, cfg, fmt.Errorf("connection failed: %w", err))
		return err
	}

	// 2. Login
	if err := client.Login(ctx, cfg.Username, cfg.Password); err != nil {
		p.handlePollFailure(ctx, cfg, fmt.Errorf("authentication failed: %w", err))
		return err
	}

	// 3. Select folder
	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	if _, err := client.Select(ctx, folder); err != nil {
		p.handlePollFailure(ctx, cfg, fmt.Errorf("folder select failed: %w", err))
		return err
	}

	// 4. Search unseen messages
	unseenSeqNos, err := client.SearchUnseen(ctx)
	if err != nil {
		p.handlePollFailure(ctx, cfg, fmt.Errorf("search unseen failed: %w", err))
		return err
	}

	// 5. Fetch and extract attachments
	for _, seqNo := range unseenSeqNos {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		msgBytes, fetchErr := client.FetchMessage(ctx, seqNo)
		if fetchErr != nil {
			p.logger.Warn("failed to fetch unread message", "seqNo", seqNo, "error", fetchErr)
			continue
		}

		attachments, extractErr := ExtractAttachments(msgBytes)
		if extractErr != nil {
			p.logger.Warn("failed to extract attachments from message", "seqNo", seqNo, "error", extractErr)
			continue
		}

		hasErrors := false
		for _, att := range attachments {
			docID := generateDocUUID()
			cleanName := filepath.Base(filepath.Clean(att.Filename))
			if cleanName == "." || cleanName == "/" || cleanName == "" {
				cleanName = "attachment.pdf"
			}
			targetName := fmt.Sprintf("%s_%s", docID, cleanName)
			targetPath := filepath.Join(p.consumeDir, targetName)

			if writeErr := os.WriteFile(targetPath, att.Data, 0644); writeErr != nil {
				p.logger.Error("failed to write extracted document attachment to consume directory",
					"path", targetPath, "error", writeErr)
				hasErrors = true
				continue
			}

			// Secure file permissions
			_ = os.Chmod(targetPath, 0644)

			p.logger.Info("ingested document from email attachment",
				"filename", cleanName, "target", targetName, "size", len(att.Data))

			// Emit dms.document_ingested matching OpenAPI schema and backward-compat fields
			if p.broadcaster != nil {
				nowStr := time.Now().UTC().Format(time.RFC3339)
				p.broadcaster(EventDocumentIngested, map[string]any{
					"id":           docID,
					"filename":     cleanName,
					"format":       att.ContentType,
					"size_bytes":   int64(len(att.Data)),
					"source":       "imap",
					"ingested_at":  nowStr,
					"target":       targetName,
					"path":         targetPath,
					"size":         int64(len(att.Data)),
					"content_type": att.ContentType,
				})
			}
		}

		// Only mark processed message as seen if all attachments were safely written
		if !hasErrors {
			if markErr := client.MarkSeen(ctx, seqNo); markErr != nil {
				p.logger.Warn("failed to mark message as seen", "seqNo", seqNo, "error", markErr)
			}
		}
	}

	// Successful poll completion
	now := time.Now().UTC()
	if updateErr := p.store.UpdateStatus(ctx, cfg.ID, &now, ""); updateErr != nil {
		p.logger.Warn("failed to update imap store status on success", "error", updateErr)
	}

	if p.serviceHealthSetter != nil {
		p.serviceHealthSetter("paperless", "running", "healthy", "Automatic Text Search Active · Mailbox Connected")
	}

	if p.broadcaster != nil {
		p.broadcaster(EventMailboxStatus, map[string]any{
			"connected":      true,
			"error":          "",
			"last_error":     "",
			"last_polled_at": now.Format(time.RFC3339),
		})
	}

	return nil
}

func (p *Poller) handlePollFailure(ctx context.Context, cfg *store.IMAPConfig, err error) {
	errMsg := err.Error()
	p.logger.Warn("imap polling failed; degrading Document Archive status to Amber",
		"host", cfg.Host, "username", cfg.Username, "error", errMsg)

	now := time.Now().UTC()
	_ = p.store.UpdateStatus(ctx, cfg.ID, &now, errMsg)

	if p.serviceHealthSetter != nil {
		p.serviceHealthSetter("paperless", "running", "degraded", ReassuranceMailboxDisconnected)
	}

	if p.broadcaster != nil {
		p.broadcaster(EventMailboxStatus, map[string]any{
			"connected":      false,
			"error":          ReassuranceMailboxDisconnected,
			"last_error":     errMsg,
			"last_polled_at": now.Format(time.RFC3339),
		})
	}
}

func generateDocUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
