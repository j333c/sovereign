package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"sovereign/internal/store"
)

// AlertDispatcher coordinates multi-channel delivery across Home Assistant webhooks,
// MQTT broker events, and consolidated SMTP email digests.
type AlertDispatcher struct {
	db            *store.DB
	deadlineStore *store.DeadlineStore
	webhookClient *WebhookClient
	mqttClient    *MQTTClient
	emailClient   *EmailClient
	logger        *slog.Logger
	baseURL       string
	nowFunc       func() time.Time
	dispatchMu    sync.Mutex
}

// NewAlertDispatcher initializes an AlertDispatcher with production defaults.
func NewAlertDispatcher(db *store.DB, dStore *store.DeadlineStore, logger *slog.Logger) *AlertDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &AlertDispatcher{
		db:            db,
		deadlineStore: dStore,
		webhookClient: NewWebhookClient(),
		mqttClient:    NewMQTTClient(),
		emailClient:   NewEmailClient(),
		logger:        logger,
		baseURL:       "http://sovereign.local",
		nowFunc:       time.Now,
	}
}

// SetNowFunc allows overriding the current time function for deterministic testing.
func (d *AlertDispatcher) SetNowFunc(fn func() time.Time) {
	d.nowFunc = fn
}

// SetBaseURL configures the base URL for direct document links (default: http://sovereign.local).
func (d *AlertDispatcher) SetBaseURL(baseURL string) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed != "" {
		d.baseURL = trimmed
	}
}

// BaseURL returns the configured base URL for document links.
func (d *AlertDispatcher) BaseURL() string {
	return d.baseURL
}

// DocumentURL constructs a direct link to an archived document.
func (d *AlertDispatcher) DocumentURL(documentID string) string {
	return fmt.Sprintf("%s/docs/documents/%s", d.baseURL, documentID)
}

// DispatchCycle evaluates pending deadlines within the alert threshold and delivers
// notifications across all enabled channels. Successfully notified deadlines transition to 'notified'.
func (d *AlertDispatcher) DispatchCycle(ctx context.Context) (int, error) {
	d.dispatchMu.Lock()
	defer d.dispatchMu.Unlock()

	if d.db == nil || d.deadlineStore == nil {
		return 0, errors.New("database or deadline store is not initialized")
	}

	cfg, err := LoadConfig(ctx, d.db)
	if err != nil {
		return 0, fmt.Errorf("failed to load alert configuration: %w", err)
	}

	threshold := cfg.DeadlineAlertThreshold
	if threshold <= 0 {
		threshold = DefaultDeadlineAlertThreshold
	}

	now := d.nowFunc()

	// Query pending deadlines within the alert threshold
	pendingDeadlines, err := d.deadlineStore.List(ctx, store.DeadlineStatusPending, threshold, now)
	if err != nil {
		return 0, fmt.Errorf("failed to list pending document deadlines: %w", err)
	}

	if len(pendingDeadlines) == 0 {
		d.logger.Debug("no impending pending deadlines found within threshold", "threshold_days", threshold)
		return 0, nil
	}

	hasWebhook := cfg.HAWebhookURL != ""
	hasMQTT := cfg.MQTTBrokerHost != ""
	hasEmail := cfg.AlertEmail != "" && cfg.SMTPHost != ""

	if !hasWebhook && !hasMQTT && !hasEmail {
		d.logger.Debug("no alert channels configured, skipping alert dispatch cycle", "pending_count", len(pendingDeadlines))
		return 0, nil
	}

	d.logger.Info("starting alert dispatch cycle",
		"pending_count", len(pendingDeadlines),
		"webhook_enabled", hasWebhook,
		"mqtt_enabled", hasMQTT,
		"email_enabled", hasEmail,
	)

	// 1. Channel: Home Assistant Webhook
	webhookSucceeded := false
	if hasWebhook {
		webhookCount := 0
		for _, dl := range pendingDeadlines {
			daysRemaining := computeDaysRemaining(dl.DueDate, now)
			payload := &WebhookDeadlinePayload{
				Event:         EventDocumentDeadline,
				ID:            dl.ID,
				DocumentID:    dl.DocumentID,
				Title:         dl.Title,
				DueDate:       dl.DueDate,
				DaysRemaining: daysRemaining,
				Amount:        dl.Amount,
				Currency:      dl.Currency,
				URL:           d.DocumentURL(dl.DocumentID),
				Snippet:       dl.Snippet,
			}

			if err := d.webhookClient.SendWebhook(ctx, cfg.HAWebhookURL, payload); err != nil {
				d.logger.Warn("failed to deliver alert to Home Assistant webhook",
					"id", dl.ID,
					"url", cfg.HAWebhookURL,
					"error", err,
				)
			} else {
				d.logger.Debug("dispatched webhook alert", "id", dl.ID, "title", dl.Title)
				webhookCount++
			}
		}
		if webhookCount > 0 {
			webhookSucceeded = true
		}
	}

	// 2. Channel: MQTT Publication (QoS 1)
	mqttSucceeded := false
	if hasMQTT {
		mqttOpts := MQTTConnectOptions{
			Host:     cfg.MQTTBrokerHost,
			Port:     cfg.MQTTBrokerPort,
			Username: cfg.MQTTUsername,
			Password: cfg.MQTTPassword,
		}

		mqttCount := 0
		for _, dl := range pendingDeadlines {
			daysRemaining := computeDaysRemaining(dl.DueDate, now)
			payloadMap := map[string]any{
				"event":          EventDocumentDeadline,
				"id":             dl.ID,
				"document_id":    dl.DocumentID,
				"title":          dl.Title,
				"due_date":       dl.DueDate,
				"days_remaining": daysRemaining,
				"amount":         dl.Amount,
				"currency":       dl.Currency,
				"url":            d.DocumentURL(dl.DocumentID),
				"snippet":        dl.Snippet,
			}
			payloadBytes, _ := json.Marshal(payloadMap)

			if err := d.mqttClient.PublishQoS1(ctx, mqttOpts, cfg.MQTTTopic, payloadBytes); err != nil {
				d.logger.Warn("failed to publish alert to MQTT broker",
					"id", dl.ID,
					"topic", cfg.MQTTTopic,
					"error", err,
				)
			} else {
				d.logger.Debug("published MQTT alert", "id", dl.ID, "topic", cfg.MQTTTopic)
				mqttCount++
			}
		}
		if mqttCount > 0 {
			mqttSucceeded = true
		}
	}

	// 3. Channel: Consolidated Email Digest
	emailSucceeded := true
	if hasEmail {
		emailSucceeded = false
		smtpCfg := SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		}

		var items []DeadlineDigestItem
		for _, dl := range pendingDeadlines {
			daysRemaining := computeDaysRemaining(dl.DueDate, now)
			items = append(items, DeadlineDigestItem{
				ID:            dl.ID,
				DocumentID:    dl.DocumentID,
				Title:         dl.Title,
				DueDate:       dl.DueDate,
				DaysRemaining: daysRemaining,
				Amount:        dl.Amount,
				Currency:      dl.Currency,
				URL:           d.DocumentURL(dl.DocumentID),
				Snippet:       dl.Snippet,
			})
		}

		if err := d.emailClient.SendDigest(ctx, smtpCfg, cfg.AlertEmail, items); err != nil {
			d.logger.Error("failed to send consolidated email alert digest",
				"recipient", cfg.AlertEmail,
				"error", err,
			)
		} else {
			d.logger.Info("sent consolidated email alert digest",
				"recipient", cfg.AlertEmail,
				"item_count", len(items),
			)
			emailSucceeded = true
		}
	}

	// If email was enabled and failed to deliver, do NOT transition to notified
	// so the next evaluation cycle retries the digest
	if hasEmail && !emailSucceeded {
		d.logger.Warn("retaining pending status for deadlines due to email delivery failure")
		return 0, nil
	}

	// If only webhook and/or MQTT were enabled, and all enabled channels failed, do NOT transition to notified
	if !hasEmail && (hasWebhook || hasMQTT) && !webhookSucceeded && !mqttSucceeded {
		d.logger.Warn("retaining pending status for deadlines because all configured ambient alert channels failed")
		return 0, nil
	}

	// Transition deadlines from 'pending' to 'notified'
	notifiedCount := 0
	for _, dl := range pendingDeadlines {
		if err := d.deadlineStore.MarkNotified(ctx, dl.ID); err != nil {
			d.logger.Error("failed to mark deadline as notified", "id", dl.ID, "error", err)
		} else {
			notifiedCount++
		}
	}

	d.logger.Info("alert dispatch cycle completed", "notified_count", notifiedCount)
	return notifiedCount, nil
}

// TestWebhook sends a test payload to the specified webhook URL.
func (d *AlertDispatcher) TestWebhook(ctx context.Context, webhookURL string) error {
	payload := &WebhookDeadlinePayload{
		Event:         EventDocumentDeadline,
		ID:            "test-alert-id",
		DocumentID:    "test-document-id",
		Title:         "Test Notification: Insurance Renewal",
		DueDate:       time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
		DaysRemaining: 7,
		URL:           d.DocumentURL("test-document-id"),
		Snippet:       "Upcoming Bill: Insurance payment of 120.00 USD is due in 7 days.",
	}
	amt := 120.00
	payload.Amount = &amt
	payload.Currency = "USD"

	return d.webhookClient.SendWebhook(ctx, webhookURL, payload)
}

// TestMQTT connects to the MQTT broker, publishes a test message, and verifies delivery.
func (d *AlertDispatcher) TestMQTT(ctx context.Context, host string, port int, topic, username, password string) error {
	opts := MQTTConnectOptions{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	}

	testPayload := map[string]any{
		"event":          "alert_test",
		"message":        "Sovereign MQTT test alert notification",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"days_remaining": 7,
	}
	payloadBytes, _ := json.Marshal(testPayload)

	return d.mqttClient.PublishQoS1(ctx, opts, topic, payloadBytes)
}

// TestEmail sends a test notification via SMTP to the target address.
func (d *AlertDispatcher) TestEmail(ctx context.Context, to, host string, port int, username, password, from string) error {
	smtpCfg := SMTPConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
	}

	return d.emailClient.SendTestEmail(ctx, smtpCfg, to)
}

// SendRollbackAlert dispatches notifications when an automated or manual rollback occurs.
func (d *AlertDispatcher) SendRollbackAlert(ctx context.Context, serviceID, serviceName, failedVersion, restoredVersion, reason string) error {
	d.dispatchMu.Lock()
	defer d.dispatchMu.Unlock()

	if d.db == nil {
		return errors.New("database is not initialized")
	}

	cfg, err := LoadConfig(ctx, d.db)
	if err != nil {
		return fmt.Errorf("failed to load alert config: %w", err)
	}

	// 1. Home Assistant Webhook
	if cfg.HAWebhookURL != "" {
		payload := map[string]any{
			"event":               "service_rollback",
			"service_id":          serviceID,
			"service_name":        serviceName,
			"failed_version":      failedVersion,
			"restored_version":    restoredVersion,
			"reason":              reason,
			"reassurance_message": "Update couldn't start properly. We've safely restored your previous working version.",
			"timestamp":           d.nowFunc().UTC().Format(time.RFC3339),
		}
		_ = d.webhookClient.SendWebhookJSON(ctx, cfg.HAWebhookURL, payload)
	}

	// 2. MQTT
	if cfg.MQTTBrokerHost != "" {
		mqttOpts := MQTTConnectOptions{
			Host:     cfg.MQTTBrokerHost,
			Port:     cfg.MQTTBrokerPort,
			Username: cfg.MQTTUsername,
			Password: cfg.MQTTPassword,
		}
		payloadBytes, _ := json.Marshal(map[string]any{
			"event":               "service_rollback",
			"service_id":          serviceID,
			"service_name":        serviceName,
			"failed_version":      failedVersion,
			"restored_version":    restoredVersion,
			"reason":              reason,
			"reassurance_message": "Update couldn't start properly. We've safely restored your previous working version.",
			"timestamp":           d.nowFunc().UTC().Format(time.RFC3339),
		})
		topic := cfg.MQTTTopic
		if topic == "" {
			topic = "sovereign/alerts/services"
		}
		_ = d.mqttClient.PublishQoS1(ctx, mqttOpts, topic, payloadBytes)
	}

	// 3. Email
	if cfg.AlertEmail != "" && cfg.SMTPHost != "" {
		smtpCfg := SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		}
		if err := d.emailClient.SendRollbackAlert(ctx, smtpCfg, cfg.AlertEmail, serviceName, failedVersion, restoredVersion, reason); err != nil {
			d.logger.Error("failed to send rollback alert email", "error", err, "recipient", cfg.AlertEmail)
			return err
		}
	}

	return nil
}

// computeDaysRemaining calculates calendar days between due date and now,
// normalized to midnight UTC.
func computeDaysRemaining(dueDateStr string, now time.Time) int {
	dueDate, err := time.Parse("2006-01-02", dueDateStr)
	if err != nil {
		return 0
	}
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(math.Round(dueDate.Sub(nowDate).Hours() / 24.0))
}
