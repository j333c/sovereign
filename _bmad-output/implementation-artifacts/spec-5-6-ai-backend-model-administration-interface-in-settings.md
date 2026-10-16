---
title: 'Story 5.6: AI Backend & Model Administration Interface in Settings'
type: 'feature'
created: '2026-10-16'
status: 'ready-for-dev'
route: 'dispatch'
baseline_commit: 'a531ca41c992636b1912e6453862f8b853493429'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Appliance administrators need an accessible administrative interface in Settings to manage local GGUF models, view RAM/VRAM resource footprint and inference speeds, tune performance (GPU offload layers `-ngl`, context length `-c`, idle surrender timeout), trigger backend engine reloads, inspect scrubbed inference logs, and download new models safely without using command-line commands. Downloads must be atomic with resumable HTTP range downloads, SHA-256 integrity verification, and pre-flight storage safety checks (+5 GB buffer) to prevent storage exhaustion or corrupted half-downloaded model files from crashing the inference engine.

**Approach:** Implement full-stack AI model administration in backend daemon and web UI:
1. **Model Download & Lifecycle Engine (`internal/ai/models.go`)**:
   - Manages models in `/data/models` (or configured storage dir).
   - `ValidateGGUFHeader` binary magic check (`GGUF` bytes) before allowing activation.
   - Resumable download manager:
     - Downloads to `<filename>.downloading` temporary files with `Range: bytes=...` support.
     - Enforces pre-flight storage check: `CheckModelStorageSpace` (required bytes + 5.0 GB safety buffer).
     - Optional SHA-256 hash checksum calculation and verification.
     - Atomic rename from `.downloading` to `.gguf` upon complete verification.
     - Progress tracking: bytes downloaded, total bytes, transfer speed (bytes/sec), ETA, error states, and pause/resume/cancel controls.
   - Model switching: validates GGUF header, updates active model setting in SQLite persistence, triggers reload of llama.cpp backend container and resets GPU arbitrator.
   - Model deletion: safely deletes inactive `.gguf` files from disk, preventing deletion of the currently active model.
2. **Daemon REST Endpoints & Parameter Tuning (`cmd/sovereign-daemon/server.go`)**:
   - `GET /api/v1/ai/models`: Lists installed models (active status, size, header metadata, RAM/VRAM footprint, tokens/sec benchmark metric, FlashAttention status).
   - `POST /api/v1/ai/models/active`: Switches active model by filename (validates header magic first).
   - `POST /api/v1/ai/models/download`: Initiates a background model download from URL with optional sha256 checksum and model name.
   - `GET /api/v1/ai/models/downloads`: Lists active and past downloads with live progress (percent, speed, ETA, status).
   - `DELETE /api/v1/ai/models/downloads/:id`: Cancels an active download.
   - `DELETE /api/v1/ai/models/:filename`: Deletes an unmounted GGUF model file.
   - `GET /api/v1/ai/settings`: Returns current AI operational parameters (`active_model`, `gpu_layers`, `context_length`, `idle_timeout_seconds`, `flash_attention`, `tokens_per_sec`, `vram_usage_bytes`, `ram_usage_bytes`).
   - `POST /api/v1/ai/settings`: Updates AI operational parameters (`gpu_layers`, `context_length`, `idle_timeout_seconds`) with immediate effect.
   - `POST /api/v1/ai/reload`: Gracefully reloads the AI backend engine.
   - `GET /api/v1/ai/logs`: Returns scrubbed recent inference logs (stripped of personal prompt text).
3. **OpenAPI Specification & TypeScript API Client (`api/openapi.yaml`, `web/src/lib/api/`)**:
   - Declare schemas: `AIModelInfo`, `AIModelsResponse`, `AIModelDownloadRequest`, `AIModelDownloadProgress`, `AISettingsResponse`, `AISettingsUpdateRequest`.
   - Add client methods in `SovereignApiClient`: `getAIModels`, `setActiveAIModel`, `downloadAIModel`, `getAIModelDownloads`, `cancelAIModelDownload`, `deleteAIModel`, `getAISettings`, `updateAISettings`, `reloadAIEngine`, `getAIInferenceLogs`.
4. **Settings Web Interface (`web/src/app/settings/`)**:
   - Administrative Settings page at `/settings` with tabbed navigation: General, Services, AI (`#ai`), and System.
   - `AIBackendSettings` card / panel matching `EXPERIENCE.md`:
     - Active Model Card: displays RAM/VRAM footprint, tokens/sec inference speed, and FlashAttention status pill (*"FlashAttention Active · KV Cache Optimized"*).
     - Model Selector Dropdown & Model Table: shows all `.gguf` files in storage with size, modified date, and active indicator; switching models triggers reload with progress spinner.
     - Download Manager: input URL + optional SHA-256 + download button; live progress bar with percentage, speed, ETA, and Cancel/Resume controls; pre-flight storage warning banner.
     - Parameter Controls:
       - Context Length Selector: segmented buttons (`2048`, `4096`, `8192`) with memory estimation badge.
       - GPU Offload Layer Slider (`-ngl`): 0 to 100% (or layer count 0 to 99), locked to CPU on headless zero-GPU systems.
       - Dynamic Idle Surrender Timeout Slider: 60s to 3600s with caption (*"Release GPU memory after X minutes of inactivity"*).
     - Operational Action Buttons: *"Reload Backend Engine"* and *"View Inference Logs"* with slide-over/modal log viewer.
     - Full four-language localization (EN, DE, ES, FR).

</frozen-after-approval>

## Acceptance Criteria

1. **Active Model Inspection**: Opening the AI settings panel renders the active model card with RAM/VRAM footprint, tokens/sec, and FlashAttention status badge.
2. **Atomic & Resumable Downloads**: Downloading a model writes to a `.downloading` file, supports HTTP Range resumption, enforces the 5.0 GB safety buffer floor, verifies SHA-256 checksum (if provided), checks GGUF magic bytes, and atomically renames to `.gguf`.
3. **Safe Model Switching**: Selecting a downloaded model verifies GGUF header magic (`GGUF`) before switching, persists active model in configuration, and gracefully reloads the inference backend.
4. **Live Parameter Tuning**: Adjusting GPU offload layers (`-ngl`), context length (`-c`), or idle surrender timeout immediately persists to settings and reconfigures the runtime environment.
5. **Operational Controls & Logs**: The administrator can trigger an engine reload and view scrubbed inference logs without terminal access.
6. **Accessibility & Responsive Design**: Complies with WCAG AA/AAA, tactile 44px touch targets, full keyboard accessibility, semantic status micro-shapes, and zero unhandled errors.

## Key Files to Create / Modify
- `internal/ai/models.go` -- Model management, download worker, GGUF file scanner, and parameter store.
- `internal/ai/models_test.go` -- Unit tests for download resumption, atomic rename, SHA-256 verification, and model switching.
- `cmd/sovereign-daemon/server.go` -- Register `/api/v1/ai/models`, `/downloads`, `/settings`, `/reload`, `/logs`.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for AI settings, model listing, switching, and download endpoints.
- `api/openapi.yaml` -- OpenAPI schemas and endpoints for AI administration.
- `web/src/lib/api/schema.d.ts` -- Generated TypeScript types.
- `web/src/lib/api/client.ts` -- TypeScript API client methods for AI settings and model administration.
- `web/src/app/settings/page.tsx` -- Settings hub with AI administration panel.
- `web/src/app/settings/ai-settings.test.tsx` -- Component tests for model card, sliders, download manager, and switching.
- `web/src/lib/i18n.ts` -- Localized microcopy for AI settings in EN, DE, ES, FR.
