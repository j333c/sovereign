package main

import (
	"net/http"
)

// routes registers HTTP REST handlers.
func (s *Server) routes() {
	s.router.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/system/boot-status", s.handleBootStatus)
	s.router.HandleFunc("POST /api/v1/system/update-status/dismiss", s.handleDismissUpdateStatus)
	s.router.HandleFunc("GET /api/v1/system/ingress", s.handleIngressStatus)
	s.router.HandleFunc("GET /api/v1/system/revisions", s.handleListRevisions)
	s.router.HandleFunc("GET /api/v1/system/revisions/active", s.handleGetActiveRevision)
	s.router.HandleFunc("POST /api/v1/system/reboot", s.requireSetupOrAdmin(s.handleSystemReboot))
	s.router.HandleFunc("POST /api/v1/system/factory-reset", s.requireSetupOrAdmin(s.handleFactoryReset))
	s.router.HandleFunc("GET /api/v1/system/events", s.handleEvents)
	s.router.HandleFunc("GET /api/events", s.handleEvents)
	s.router.HandleFunc("GET /api/v1/events", s.handleEvents)
	s.router.HandleFunc("GET /api/v1/telemetry/events", s.handleEvents)
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
	s.router.Handle("PUT /api/v1/settings", s.authMgr.RequireAdmin(http.HandlerFunc(s.handleUpdateSettings)))

	// Managed Services lifecycle endpoints
	s.router.HandleFunc("GET /api/v1/services", s.handleListServices)
	s.router.HandleFunc("GET /api/v1/services/{id}", s.handleGetService)
	s.router.HandleFunc("POST /api/v1/services/{id}/activate", s.requireSetupOrAdmin(s.handleActivateService))
	s.router.HandleFunc("POST /api/v1/services/{id}/deactivate", s.requireSetupOrAdmin(s.handleDeactivateService))
	s.router.HandleFunc("POST /api/v1/services/{id}/restart", s.requireSetupOrAdmin(s.handleRestartService))
	s.router.HandleFunc("POST /api/v1/services/{id}/upgrade", s.requireSetupOrAdmin(s.handleUpgradeService))
	s.router.HandleFunc("POST /api/v1/services/{id}/rollback", s.requireSetupOrAdmin(s.handleRollbackService))
	s.router.HandleFunc("POST /api/v1/services/{id}/refresh", s.requireSetupOrAdmin(s.handleRefreshService))

	// Media Dropzone ingestion endpoints
	s.router.HandleFunc("POST /api/v1/drop/upload", s.handleDropUpload)
	s.router.HandleFunc("GET /api/v1/drop/history", s.handleDropHistory)

	// Optical drive and automated disc ripping endpoints
	s.router.HandleFunc("GET /api/v1/optical/status", s.handleOpticalStatus)
	s.router.HandleFunc("POST /api/v1/optical/eject", s.requireSetupOrAdmin(s.handleOpticalEject))
	s.router.HandleFunc("POST /api/v1/optical/keys", s.requireSetupOrAdmin(s.handleOpticalKeys))

	// Media metadata and artwork enrichment endpoints
	s.router.HandleFunc("GET /api/v1/enrichment/status", s.handleEnrichmentStatus)
	s.router.HandleFunc("POST /api/v1/enrichment/scan", s.requireSetupOrAdmin(s.handleEnrichmentScan))

	// Off-peak transcoding queue and pre-encoding endpoints
	s.router.HandleFunc("GET /api/v1/transcode/queue", s.handleTranscodeQueue)
	s.router.HandleFunc("POST /api/v1/transcode/queue", s.requireSetupOrAdmin(s.handleEnqueueTranscode))
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
	s.router.HandleFunc("GET /api/v1/documents/deadlines", s.requireSetupOrAdmin(s.handleGetDeadlines))
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

	// AI Persona and Prompt Customization Endpoints
	s.router.HandleFunc("GET /api/v1/ai/personas", s.handleListAIPersonas)
	s.router.HandleFunc("GET /api/v1/ai/personas/{id}", s.handleGetAIPersona)
	s.router.HandleFunc("POST /api/v1/ai/personas/active", s.requireSetupOrAdmin(s.handleSetActiveAIPersona))
	s.router.HandleFunc("GET /api/v1/ai/prompts", s.handleListAIPrompts)

	// Single-Source-of-Truth RAG Endpoints
	s.router.HandleFunc("POST /api/v1/ai/rag/search", s.requireSetupOrAdmin(s.handleRAGSearch))
	s.router.HandleFunc("POST /api/v1/ai/rag/query", s.requireSetupOrAdmin(s.handleRAGQuery))

	// AI Model and Backend Administration Endpoints
	s.router.HandleFunc("GET /api/v1/ai/models", s.handleListAIModels)
	s.router.HandleFunc("POST /api/v1/ai/models/active", s.requireSetupOrAdmin(s.handleSetActiveAIModel))
	s.router.HandleFunc("POST /api/v1/ai/models/download", s.requireSetupOrAdmin(s.handleDownloadAIModel))
	s.router.HandleFunc("GET /api/v1/ai/models/downloads", s.handleListAIModelDownloads)
	s.router.HandleFunc("DELETE /api/v1/ai/models/downloads/{id}", s.requireSetupOrAdmin(s.handleCancelAIModelDownload))
	s.router.HandleFunc("DELETE /api/v1/ai/models/{filename}", s.requireSetupOrAdmin(s.handleDeleteAIModel))
	s.router.HandleFunc("GET /api/v1/ai/settings", s.handleGetAISettings)
	s.router.HandleFunc("POST /api/v1/ai/settings", s.requireSetupOrAdmin(s.handleUpdateAISettings))
	s.router.HandleFunc("POST /api/v1/ai/reload", s.requireSetupOrAdmin(s.handleReloadAIEngine))
	s.router.HandleFunc("GET /api/v1/ai/logs", s.requireSetupOrAdmin(s.handleGetAIInferenceLogs))
}
