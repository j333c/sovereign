package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sovereign/internal/notify"
	"strings"
)

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
