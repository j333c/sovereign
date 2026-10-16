package notify

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"sovereign/internal/store"
)

const (
	SettingHAWebhookURL           = "ha_webhook_url"
	SettingMQTTBrokerHost         = "mqtt_broker_host"
	SettingMQTTBrokerPort         = "mqtt_broker_port"
	SettingMQTTTopic              = "mqtt_topic"
	SettingMQTTUsername           = "mqtt_username"
	SettingMQTTPassword           = "mqtt_password"
	SettingAlertEmail             = "alert_email"
	SettingSMTPHost               = "smtp_host"
	SettingSMTPPort               = "smtp_port"
	SettingSMTPUsername           = "smtp_username"
	SettingSMTPPassword           = "smtp_password"
	SettingSMTPFrom               = "smtp_from"
	SettingDeadlineAlertThreshold = "deadline_alert_threshold"

	DefaultMQTTBrokerPort         = 1883
	DefaultMQTTTopic              = "sovereign/alerts/documents"
	DefaultSMTPPort               = 587
	DefaultDeadlineAlertThreshold = 14
	DefaultSMTPFrom               = "alerts@sovereign.local"
	PasswordMask                  = "********"
)

// AlertConfig holds all configuration parameters for the alert dispatch channels.
type AlertConfig struct {
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

// Redacted returns a copy of AlertConfig with sensitive passwords masked as "********".
func (c *AlertConfig) Redacted() AlertConfig {
	copyCfg := *c
	if copyCfg.MQTTPassword != "" {
		copyCfg.MQTTPassword = PasswordMask
	}
	if copyCfg.SMTPPassword != "" {
		copyCfg.SMTPPassword = PasswordMask
	}
	return copyCfg
}

// LoadConfig reads the alert configuration from SQLite settings.
func LoadConfig(ctx context.Context, db *store.DB) (*AlertConfig, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle cannot be nil")
	}

	cfg := &AlertConfig{
		MQTTBrokerPort:         DefaultMQTTBrokerPort,
		MQTTTopic:              DefaultMQTTTopic,
		SMTPPort:               DefaultSMTPPort,
		SMTPFrom:               DefaultSMTPFrom,
		DeadlineAlertThreshold: DefaultDeadlineAlertThreshold,
	}

	if val, err := db.GetSetting(ctx, SettingHAWebhookURL); err == nil && val != "" {
		cfg.HAWebhookURL = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingMQTTBrokerHost); err == nil && val != "" {
		cfg.MQTTBrokerHost = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingMQTTBrokerPort); err == nil && val != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && p > 0 {
			cfg.MQTTBrokerPort = p
		}
	}

	if val, err := db.GetSetting(ctx, SettingMQTTTopic); err == nil && val != "" {
		cfg.MQTTTopic = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingMQTTUsername); err == nil {
		cfg.MQTTUsername = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingMQTTPassword); err == nil {
		cfg.MQTTPassword = val
	}

	if val, err := db.GetSetting(ctx, SettingAlertEmail); err == nil && val != "" {
		cfg.AlertEmail = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingSMTPHost); err == nil && val != "" {
		cfg.SMTPHost = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingSMTPPort); err == nil && val != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && p > 0 {
			cfg.SMTPPort = p
		}
	}

	if val, err := db.GetSetting(ctx, SettingSMTPUsername); err == nil {
		cfg.SMTPUsername = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingSMTPPassword); err == nil {
		cfg.SMTPPassword = val
	}

	if val, err := db.GetSetting(ctx, SettingSMTPFrom); err == nil && val != "" {
		cfg.SMTPFrom = strings.TrimSpace(val)
	}

	if val, err := db.GetSetting(ctx, SettingDeadlineAlertThreshold); err == nil && val != "" {
		if t, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && t > 0 {
			cfg.DeadlineAlertThreshold = t
		}
	}

	return cfg, nil
}

// SaveConfig updates the alert configuration in SQLite settings, preserving existing
// passwords if the incoming values are masked or empty.
func SaveConfig(ctx context.Context, db *store.DB, cfg *AlertConfig) error {
	if db == nil {
		return fmt.Errorf("database handle cannot be nil")
	}
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Password preservation logic
	mqttPass := cfg.MQTTPassword
	if mqttPass == PasswordMask || mqttPass == "" {
		existing, _ := db.GetSetting(ctx, SettingMQTTPassword)
		mqttPass = existing
	}

	smtpPass := cfg.SMTPPassword
	if smtpPass == PasswordMask || smtpPass == "" {
		existing, _ := db.GetSetting(ctx, SettingSMTPPassword)
		smtpPass = existing
	}

	mqttPort := cfg.MQTTBrokerPort
	if mqttPort <= 0 {
		mqttPort = DefaultMQTTBrokerPort
	}

	mqttTopic := strings.TrimSpace(cfg.MQTTTopic)
	if mqttTopic == "" {
		mqttTopic = DefaultMQTTTopic
	}

	smtpPort := cfg.SMTPPort
	if smtpPort <= 0 {
		smtpPort = DefaultSMTPPort
	}

	smtpFrom := strings.TrimSpace(cfg.SMTPFrom)
	if smtpFrom == "" {
		smtpFrom = DefaultSMTPFrom
	}

	threshold := cfg.DeadlineAlertThreshold
	if threshold <= 0 {
		threshold = DefaultDeadlineAlertThreshold
	}

	settings := map[string]string{
		SettingHAWebhookURL:           strings.TrimSpace(cfg.HAWebhookURL),
		SettingMQTTBrokerHost:         strings.TrimSpace(cfg.MQTTBrokerHost),
		SettingMQTTBrokerPort:         strconv.Itoa(mqttPort),
		SettingMQTTTopic:              mqttTopic,
		SettingMQTTUsername:           strings.TrimSpace(cfg.MQTTUsername),
		SettingMQTTPassword:           mqttPass,
		SettingAlertEmail:             strings.TrimSpace(cfg.AlertEmail),
		SettingSMTPHost:               strings.TrimSpace(cfg.SMTPHost),
		SettingSMTPPort:               strconv.Itoa(smtpPort),
		SettingSMTPUsername:           strings.TrimSpace(cfg.SMTPUsername),
		SettingSMTPPassword:           smtpPass,
		SettingSMTPFrom:               smtpFrom,
		SettingDeadlineAlertThreshold: strconv.Itoa(threshold),
	}

	for k, v := range settings {
		if err := db.SetSetting(ctx, k, v); err != nil {
			return fmt.Errorf("failed to save setting %q: %w", k, err)
		}
	}

	return nil
}
