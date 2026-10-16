import type { components } from "./schema";

export type HealthResponse = components["schemas"]["HealthResponse"];
export type BootStatusResponse = components["schemas"]["BootStatusResponse"];
export type IngressStatusResponse = components["schemas"]["IngressStatusResponse"];
export type IngressRoute = components["schemas"]["IngressRoute"];
export type ProblemDetails = components["schemas"]["ProblemDetails"];
export type DiskInfo = components["schemas"]["DiskInfo"];
export type DisksResponse = components["schemas"]["DisksResponse"];
export type StorageInitializeRequest = components["schemas"]["StorageInitializeRequest"];
export type StorageInitializeResponse = components["schemas"]["StorageInitializeResponse"];
export type StorageNASTestRequest = components["schemas"]["StorageNASTestRequest"];
export type StorageNASTestResponse = components["schemas"]["StorageNASTestResponse"];
export type StorageNASMountRequest = components["schemas"]["StorageNASMountRequest"];
export type StorageNASMountResponse = components["schemas"]["StorageNASMountResponse"];
export type StorageLocalPool = components["schemas"]["StorageLocalPool"];
export type StorageNASStatus = components["schemas"]["StorageNASStatus"];
export type StorageStatusResponse = components["schemas"]["StorageStatusResponse"];
export type SetupStatusResponse = components["schemas"]["SetupStatusResponse"];
export type SetupRequest = components["schemas"]["SetupRequest"];
export type AuthSessionResponse = components["schemas"]["AuthSessionResponse"];
export type SessionInfo = components["schemas"]["SessionInfo"];
export type LoginRequest = components["schemas"]["LoginRequest"];
export type SessionResponse = components["schemas"]["SessionResponse"];
export type LogoutResponse = components["schemas"]["LogoutResponse"];
export type SettingsResponse = components["schemas"]["SettingsResponse"];
export type TelemetryUpdate = components["schemas"]["TelemetryUpdate"];
export type ServiceStatus = components["schemas"]["ServiceStatus"];
export type ServicesListResponse = components["schemas"]["ServicesListResponse"];
export type ServicePullProgressEvent = components["schemas"]["ServicePullProgressEvent"];
export type ServiceHealthChangedEvent = components["schemas"]["ServiceHealthChangedEvent"];
export type ServiceErrorEvent = components["schemas"]["ServiceErrorEvent"];
export type DropUploadResponse = components["schemas"]["DropUploadResponse"];
export type DropHistoryItem = components["schemas"]["DropHistoryItem"];
export type DropHistoryResponse = components["schemas"]["DropHistoryResponse"];
export type DocumentUploadResponse = components["schemas"]["DocumentUploadResponse"];
export type IMAPConfigRequest = components["schemas"]["IMAPConfigRequest"];
export type IMAPConfigResponse = components["schemas"]["IMAPConfigResponse"];
export type IMAPTestRequest = components["schemas"]["IMAPTestRequest"];
export type IMAPTestResponse = components["schemas"]["IMAPTestResponse"];
export type DMSDocumentIngestedEvent = components["schemas"]["DMSDocumentIngestedEvent"];
export type DMSMailboxStatusEvent = components["schemas"]["DMSMailboxStatusEvent"];
export type DocumentDeadline = components["schemas"]["DocumentDeadline"];
export type ScanDeadlinesRequest = components["schemas"]["ScanDeadlinesRequest"];
export type ScanDeadlinesResponse = components["schemas"]["ScanDeadlinesResponse"];
export type DMSDeadlineDetectedEvent = components["schemas"]["DMSDeadlineDetectedEvent"];
export type AlertConfigResponse = components["schemas"]["AlertConfigResponse"];
export type UpdateAlertConfigRequest = components["schemas"]["UpdateAlertConfigRequest"];
export type TestWebhookRequest = components["schemas"]["TestWebhookRequest"];
export type TestMQTTRequest = components["schemas"]["TestMQTTRequest"];
export type TestEmailRequest = components["schemas"]["TestEmailRequest"];
export type AlertTestResponse = components["schemas"]["AlertTestResponse"];
export type AlertDispatchResponse = components["schemas"]["AlertDispatchResponse"];
export type AIPreset = components["schemas"]["AIPreset"];
export type AIPresetsResponse = components["schemas"]["AIPresetsResponse"];
export type AIGovernorStatusResponse = components["schemas"]["AIGovernorStatusResponse"];
export type AIStatusResponse = components["schemas"]["AIStatusResponse"];
export type Persona = components["schemas"]["Persona"];
export type PersonaListResponse = components["schemas"]["PersonaListResponse"];
export type SetActivePersonaRequest = components["schemas"]["SetActivePersonaRequest"];
export type PromptTemplate = components["schemas"]["PromptTemplate"];
export type PromptsResponse = components["schemas"]["PromptsResponse"];
export type RAGSearchRequest = components["schemas"]["RAGSearchRequest"];
export type RAGSearchResult = components["schemas"]["RAGSearchResult"];
export type RAGSearchResponse = components["schemas"]["RAGSearchResponse"];
export type RAGQueryRequest = components["schemas"]["RAGQueryRequest"];
export type RAGQueryResponse = components["schemas"]["RAGQueryResponse"];
export type AIModelInfo = components["schemas"]["AIModelInfo"];
export type AIModelsResponse = components["schemas"]["AIModelsResponse"];
export type AIModelDownloadRequest = components["schemas"]["AIModelDownloadRequest"];
export type AIModelDownloadProgress = components["schemas"]["AIModelDownloadProgress"];
export type AISettingsResponse = components["schemas"]["AISettingsResponse"];
export type AISettingsUpdateRequest = components["schemas"]["AISettingsUpdateRequest"];
export type AIInferenceLogEntry = components["schemas"]["AIInferenceLogEntry"];
export type AIInferenceLogsResponse = components["schemas"]["AIInferenceLogsResponse"];
export type ComposeRevision = components["schemas"]["ComposeRevision"];
export type ComposeRevisionsResponse = components["schemas"]["ComposeRevisionsResponse"];
export type ServiceUpgradeRequest = components["schemas"]["ServiceUpgradeRequest"];


/**
 * ProblemDetailsError represents an RFC 7807 problem details error thrown by the daemon API.
 */
export class ProblemDetailsError extends Error {
  readonly type: string;
  readonly title: string;
  readonly status: number;
  readonly detail: string;
  readonly instance: string;

  constructor(problem: ProblemDetails) {
    super(`[${problem.status}] ${problem.title}: ${problem.detail}`);
    this.name = "ProblemDetailsError";
    this.type = problem.type;
    this.title = problem.title;
    this.status = problem.status;
    this.detail = problem.detail;
    this.instance = problem.instance;
  }
}

export interface ApiClientOptions {
  baseUrl?: string;
  fetchFn?: typeof fetch;
}

/**
 * SovereignApiClient provides contract-driven, type-safe communication
 * with the sovereign-daemon core supervisor.
 */
export class SovereignApiClient {
  readonly baseUrl: string;
  private readonly fetchFn: typeof fetch;

  constructor(options: ApiClientOptions = {}) {
    let base = options.baseUrl;
    if (base === undefined) {
      const envUrl = typeof process !== "undefined" ? process.env?.NEXT_PUBLIC_API_URL : undefined;
      if (envUrl) {
        base = envUrl;
      } else if (typeof window === "undefined") {
        base = "http://localhost:8080";
      } else {
        base = "";
      }
    }
    this.baseUrl = base.replace(/\/+$/, "");
    this.fetchFn = options.fetchFn ?? fetch.bind(globalThis);
  }

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const url = this.baseUrl ? `${this.baseUrl}${path}` : path;
    const response = await this.fetchFn(url, {
      credentials: "include",
      ...init,
      headers: {
        Accept: "application/json, application/problem+json",
        ...init?.headers,
      },
    });

    if (!response.ok) {
      let problem: ProblemDetails;
      try {
        const body = (await response.json()) as Record<string, unknown> | null;
        problem = {
          type: typeof body?.type === "string" ? body.type : "https://sovereign.local/errors/unknown",
          title: typeof body?.title === "string" ? body.title : (response.statusText || "HTTP Error"),
          status: typeof body?.status === "number" ? body.status : response.status,
          detail: typeof body?.detail === "string"
            ? body.detail
            : typeof body?.message === "string"
            ? body.message
            : typeof body?.error === "string"
            ? body.error
            : `Request to ${path} failed with HTTP status ${response.status}`,
          instance: typeof body?.instance === "string" ? body.instance : path,
        };
      } catch {
        problem = {
          type: "https://sovereign.local/errors/unknown",
          title: response.statusText || "HTTP Error",
          status: response.status,
          detail: `Request to ${path} failed with HTTP status ${response.status}`,
          instance: path,
        };
      }
      throw new ProblemDetailsError(problem);
    }

    if (response.status === 204) {
      return {} as T;
    }
    const text = await response.text();
    if (!text || text.trim() === "") {
      return {} as T;
    }
    return JSON.parse(text) as T;
  }

  /**
   * Health probe verifying daemon and SQLite WAL state.
   */
  async getHealth(): Promise<HealthResponse> {
    return this.request<HealthResponse>("/api/v1/health");
  }

  /**
   * Boot lifecycle status and diagnostic results.
   */
  async getBootStatus(): Promise<BootStatusResponse> {
    return this.request<BootStatusResponse>("/api/v1/system/boot-status");
  }

  /**
   * Ingress gateway, route configuration, and mDNS discovery status.
   */
  async getIngressStatus(): Promise<IngressStatusResponse> {
    return this.request<IngressStatusResponse>("/api/v1/system/ingress");
  }

  /**
   * Enumerate unmounted SATA/NVMe block devices with capacity.
   */
  async getDisks(): Promise<DisksResponse> {
    return this.request<DisksResponse>("/api/v1/storage/disks");
  }

  /**
   * Format and initialize local storage pool under /data with ext4 journaling.
   */
  async initializeStorage(request: StorageInitializeRequest): Promise<StorageInitializeResponse> {
    return this.request<StorageInitializeResponse>("/api/v1/storage/initialize", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
  }

  /**
   * Non-destructive test of network storage credentials and connectivity.
   */
  async testNAS(request: StorageNASTestRequest): Promise<StorageNASTestResponse> {
    return this.request<StorageNASTestResponse>("/api/v1/storage/nas/test", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
  }

  /**
   * Save and mount network storage share to /data/media or target folder.
   */
  async mountNAS(request: StorageNASMountRequest): Promise<StorageNASMountResponse> {
    return this.request<StorageNASMountResponse>("/api/v1/storage/nas/mount", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
  }

  /**
   * Get storage pool, capacity telemetry, and NAS connection health.
   */
  async getStorageStatus(): Promise<StorageStatusResponse> {
    return this.request<StorageStatusResponse>("/api/v1/storage/status");
  }

  /**
   * Check first-run setup completion status.
   */
  async getSetupStatus(): Promise<SetupStatusResponse> {
    return this.request<SetupStatusResponse>("/api/v1/auth/setup-status");
  }

  /**
   * Complete the 5-step onboarding setup wizard and establish administrator session.
   */
  async completeSetup(request: SetupRequest): Promise<AuthSessionResponse> {
    return this.request<AuthSessionResponse>("/api/v1/auth/setup", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
  }

  /**
   * Administrative login with username and password.
   */
  async login(request: LoginRequest): Promise<AuthSessionResponse> {
    return this.request<AuthSessionResponse>("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
  }

  /**
   * Terminate active administrative session and clear session cookie.
   */
  async logout(): Promise<LogoutResponse> {
    return this.request<LogoutResponse>("/api/v1/auth/logout", {
      method: "POST",
    });
  }

  /**
   * Retrieve active session state.
   */
  async getSession(): Promise<SessionResponse> {
    return this.request<SessionResponse>("/api/v1/auth/session");
  }

  /**
   * Retrieve administrative settings.
   */
  async getSettings(): Promise<SettingsResponse> {
    return this.request<SettingsResponse>("/api/v1/settings");
  }

  /**
   * Update administrative settings.
   */
  async updateSettings(settings: Record<string, string>): Promise<{ status: string }> {
    return this.request<{ status: string }>("/api/v1/settings", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(settings),
    });
  }

  /**
   * List managed appliance services and their lifecycle states.
   */
  async getServices(): Promise<ServicesListResponse> {
    return this.request<ServicesListResponse>("/api/v1/services");
  }

  /**
   * Retrieve lifecycle and health status for a specific service ID.
   */
  async getService(id: string): Promise<ServiceStatus> {
    return this.request<ServiceStatus>(`/api/v1/services/${encodeURIComponent(id)}`);
  }

  /**
   * Activate a managed service on-demand.
   */
  async activateService(id: string): Promise<ServiceStatus> {
    return this.request<ServiceStatus>(`/api/v1/services/${encodeURIComponent(id)}/activate`, {
      method: "POST",
    });
  }

  /**
   * Deactivate a running service while preserving persistent data volumes.
   */
  async deactivateService(id: string): Promise<ServiceStatus> {
    return this.request<ServiceStatus>(`/api/v1/services/${encodeURIComponent(id)}/deactivate`, {
      method: "POST",
    });
  }

  /**
   * Restart a service container runtime.
   */
  async restartService(id: string): Promise<ServiceStatus> {
    return this.request<ServiceStatus>(`/api/v1/services/${encodeURIComponent(id)}/restart`, {
      method: "POST",
    });
  }

  /**
   * Upgrade a service container image with pre-upgrade snapshots and declarative revision tracking.
   */
  async upgradeService(id: string, req?: ServiceUpgradeRequest): Promise<ComposeRevision> {
    return this.request<ComposeRevision>(`/api/v1/services/${encodeURIComponent(id)}/upgrade`, {
      method: "POST",
      headers: req ? { "Content-Type": "application/json" } : undefined,
      body: req ? JSON.stringify(req) : undefined,
    });
  }

  /**
   * List declarative Compose revisions and snapshots.
   */
  async getRevisions(): Promise<ComposeRevisionsResponse> {
    return this.request<ComposeRevisionsResponse>("/api/v1/system/revisions", {
      method: "GET",
    });
  }

  /**
   * Get the active Compose revision metadata.
   */
  async getActiveRevision(): Promise<ComposeRevision> {
    return this.request<ComposeRevision>("/api/v1/system/revisions/active", {
      method: "GET",
    });
  }

  /**
   * Upload media file to staged dropzone pipeline.
   */
  async uploadDropMedia(
    category: string,
    file: File | Blob,
    filename?: string,
    onProgress?: (progress: number) => void
  ): Promise<DropUploadResponse> {
    const formData = new FormData();
    formData.append("category", category);
    if (filename) {
      formData.append("file", file, filename);
    } else {
      formData.append("file", file);
    }

    const url = this.baseUrl ? `${this.baseUrl}/api/v1/drop/upload` : "/api/v1/drop/upload";

    if (typeof XMLHttpRequest !== "undefined" && onProgress) {
      return new Promise<DropUploadResponse>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open("POST", url);
        xhr.withCredentials = true;
        xhr.setRequestHeader("Accept", "application/json, application/problem+json");

        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable) {
            const percent = Math.round((event.loaded / event.total) * 100);
            onProgress(percent);
          }
        };

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            try {
              const data = JSON.parse(xhr.responseText) as DropUploadResponse;
              onProgress(100);
              resolve(data);
            } catch (e) {
              reject(e);
            }
          } else {
            let problem: ProblemDetails;
            try {
              const body = JSON.parse(xhr.responseText) as Record<string, unknown>;
              problem = {
                type: typeof body?.type === "string" ? body.type : "https://sovereign.local/errors/unknown",
                title: typeof body?.title === "string" ? body.title : `HTTP Error ${xhr.status}`,
                status: xhr.status,
                detail: typeof body?.detail === "string" ? body.detail : xhr.statusText || "An error occurred",
                instance: typeof body?.instance === "string" ? body.instance : "/api/v1/drop/upload",
              };
            } catch {
              problem = {
                type: "https://sovereign.local/errors/unknown",
                title: `HTTP Error ${xhr.status}`,
                status: xhr.status,
                detail: xhr.statusText || "An error occurred",
                instance: "/api/v1/drop/upload",
              };
            }
            reject(new ProblemDetailsError(problem));
          }
        };

        xhr.onerror = () => {
          reject(new Error("Network request failed"));
        };

        xhr.send(formData);
      });
    }

    if (onProgress) onProgress(50);
    const result = await this.request<DropUploadResponse>("/api/v1/drop/upload", {
      method: "POST",
      body: formData,
    });
    if (onProgress) onProgress(100);
    return result;
  }

  /**
   * Retrieve recent dropzone upload and ingestion history.
   */
  async getDropHistory(): Promise<DropHistoryItem[]> {
    const res = await this.request<DropHistoryResponse>("/api/v1/drop/history");
    return res.items ?? [];
  }

  /**
   * Direct document upload to /data/documents/consume/ for Paperless OCR.
   */
  async uploadDocument(file: File | Blob, filename?: string): Promise<DocumentUploadResponse> {
    const formData = new FormData();
    formData.append("file", file, filename || (file as File).name || "document.pdf");
    return this.request<DocumentUploadResponse>("/api/v1/documents/upload", {
      method: "POST",
      body: formData,
    });
  }

  /**
   * Retrieve active IMAP mailbox configuration with masked credentials.
   */
  async getIMAPConfig(): Promise<IMAPConfigResponse> {
    return this.request<IMAPConfigResponse>("/api/v1/documents/imap");
  }

  /**
   * Save or update IMAP mailbox configuration.
   */
  async saveIMAPConfig(config: IMAPConfigRequest): Promise<IMAPConfigResponse> {
    return this.request<IMAPConfigResponse>("/api/v1/documents/imap", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
  }

  /**
   * Test IMAP server connectivity and credentials.
   */
  async testIMAPConnection(config: IMAPTestRequest): Promise<IMAPTestResponse> {
    return this.request<IMAPTestResponse>("/api/v1/documents/imap/test", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
  }

  /**
   * Retrieve extracted document deadlines and impending obligations.
   * @param status Filter deadlines by status (default: "pending").
   * @param withinDays Filter impending deadlines within the specified days threshold (default: 14).
   */
  async getDeadlines(status?: string, withinDays?: number): Promise<DocumentDeadline[]> {
    const params = new URLSearchParams();
    if (status) params.set("status", status);
    if (withinDays !== undefined) params.set("within_days", withinDays.toString());
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request<DocumentDeadline[]>(`/api/v1/documents/deadlines${query}`);
  }

  /**
   * Dismiss a detected document deadline.
   * @param id Unique deadline UUID identifier.
   */
  async dismissDeadline(id: string): Promise<DocumentDeadline> {
    return this.request<DocumentDeadline>(`/api/v1/documents/deadlines/${encodeURIComponent(id)}/dismiss`, {
      method: "POST",
    });
  }

  /**
   * Trigger a manual obligation and deadline extraction scan.
   */
  async scanDeadlines(req?: ScanDeadlinesRequest): Promise<ScanDeadlinesResponse> {
    return this.request<ScanDeadlinesResponse>("/api/v1/documents/deadlines/scan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: req ? JSON.stringify(req) : undefined,
    });
  }

  /**
   * Retrieve alert channel configurations with passwords redacted.
   */
  async getAlertConfig(): Promise<AlertConfigResponse> {
    return this.request<AlertConfigResponse>("/api/v1/alerts/config");
  }

  /**
   * Update alert channel configurations.
   */
  async updateAlertConfig(config: UpdateAlertConfigRequest): Promise<AlertConfigResponse> {
    return this.request<AlertConfigResponse>("/api/v1/alerts/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
  }

  /**
   * Test Home Assistant webhook alert delivery.
   */
  async testWebhook(req?: TestWebhookRequest): Promise<AlertTestResponse> {
    return this.request<AlertTestResponse>("/api/v1/alerts/test/webhook", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: req ? JSON.stringify(req) : JSON.stringify({}),
    });
  }

  /**
   * Test MQTT broker connectivity and test alert publishing.
   */
  async testMQTT(req?: TestMQTTRequest): Promise<AlertTestResponse> {
    return this.request<AlertTestResponse>("/api/v1/alerts/test/mqtt", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: req ? JSON.stringify(req) : JSON.stringify({}),
    });
  }

  /**
   * Test SMTP server connectivity and alert email sending.
   */
  async testEmail(req?: TestEmailRequest): Promise<AlertTestResponse> {
    return this.request<AlertTestResponse>("/api/v1/alerts/test/email", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: req ? JSON.stringify(req) : JSON.stringify({}),
    });
  }

  /**
   * Trigger an immediate deadline evaluation and alert dispatch cycle.
   */
  async dispatchAlerts(): Promise<AlertDispatchResponse> {
    return this.request<AlertDispatchResponse>("/api/v1/alerts/dispatch", {
      method: "POST",
    });
  }

  /**
   * Retrieve list of generation intent presets.
   */
  async getAIPresets(): Promise<AIPresetsResponse> {
    return this.request<AIPresetsResponse>("/api/v1/ai/presets");
  }

  /**
   * Retrieve live Request Governor concurrency and queue metrics.
   */
  async getAIGovernorStatus(): Promise<AIGovernorStatusResponse> {
    return this.request<AIGovernorStatusResponse>("/api/v1/ai/governor/status");
  }

  /**
   * Retrieve real-time GPU arbitration state, idle surrender countdown, and hybrid GPU assignments.
   */
  async getAIStatus(): Promise<AIStatusResponse> {
    return this.request<AIStatusResponse>("/api/v1/ai/status");
  }

  /**
   * Retrieve list of AI personas with localized names, descriptions, and presets.
   */
  async getPersonas(lang?: string): Promise<PersonaListResponse> {
    const query = lang ? `?lang=${encodeURIComponent(lang)}` : "";
    return this.request<PersonaListResponse>(`/api/v1/ai/personas${query}`);
  }

  /**
   * Retrieve single AI persona detail including localized system prompt.
   */
  async getPersona(id: string, lang?: string): Promise<Persona> {
    const query = lang ? `?lang=${encodeURIComponent(lang)}` : "";
    return this.request<Persona>(`/api/v1/ai/personas/${encodeURIComponent(id)}${query}`);
  }

  /**
   * Set active default AI persona.
   */
  async setActivePersona(id: string, language?: string): Promise<Persona> {
    const payload: SetActivePersonaRequest = { id, language };
    return this.request<Persona>("/api/v1/ai/personas/active", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  }

  /**
   * Retrieve list of system prompt templates across languages and personas.
   */
  async getAIPrompts(lang?: string): Promise<PromptsResponse> {
    const query = lang ? `?lang=${encodeURIComponent(lang)}` : "";
    return this.request<PromptsResponse>(`/api/v1/ai/prompts${query}`);
  }

  /**
   * Search Paperless document archive using FTS5.
   */
  async searchRAG(req: RAGSearchRequest): Promise<RAGSearchResponse> {
    return this.request<RAGSearchResponse>("/api/v1/ai/rag/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    });
  }

  /**
   * Query document archive with Single-Source-of-Truth RAG and context window headroom truncator.
   */
  async queryRAG(req: RAGQueryRequest): Promise<RAGQueryResponse> {
    return this.request<RAGQueryResponse>("/api/v1/ai/rag/query", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    });
  }

  /**
   * List installed AI GGUF models.
   */
  async getAIModels(): Promise<AIModelsResponse> {
    return this.request<AIModelsResponse>("/api/v1/ai/models");
  }

  /**
   * Switch active AI GGUF model by filename.
   */
  async setActiveAIModel(filename: string): Promise<{ status: string; active_model: string; message?: string }> {
    return this.request<{ status: string; active_model: string; message?: string }>("/api/v1/ai/models/active", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ filename }),
    });
  }

  /**
   * Initiate background model download with pre-flight storage safety check.
   */
  async downloadAIModel(req: AIModelDownloadRequest): Promise<AIModelDownloadProgress> {
    return this.request<AIModelDownloadProgress>("/api/v1/ai/models/download", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    });
  }

  /**
   * List active and past model downloads.
   */
  async getAIModelDownloads(): Promise<{ downloads: AIModelDownloadProgress[] }> {
    return this.request<{ downloads: AIModelDownloadProgress[] }>("/api/v1/ai/models/downloads");
  }

  /**
   * Cancel an active model download by ID.
   */
  async cancelAIModelDownload(id: string): Promise<{ status: string; id: string }> {
    return this.request<{ status: string; id: string }>(`/api/v1/ai/models/downloads/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
  }

  /**
   * Delete an unmounted GGUF model from storage.
   */
  async deleteAIModel(filename: string): Promise<{ status: string; deleted: string }> {
    return this.request<{ status: string; deleted: string }>(`/api/v1/ai/models/${encodeURIComponent(filename)}`, {
      method: "DELETE",
    });
  }

  /**
   * Get AI operational parameters and memory footprints.
   */
  async getAISettings(): Promise<AISettingsResponse> {
    return this.request<AISettingsResponse>("/api/v1/ai/settings");
  }

  /**
   * Update AI operational parameters (gpu_layers, context_length, idle_timeout_seconds).
   */
  async updateAISettings(req: AISettingsUpdateRequest): Promise<AISettingsResponse> {
    return this.request<AISettingsResponse>("/api/v1/ai/settings", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    });
  }

  /**
   * Trigger graceful reload of the AI inference engine container.
   */
  async reloadAIEngine(): Promise<{ status: string; message: string }> {
    return this.request<{ status: string; message: string }>("/api/v1/ai/reload", {
      method: "POST",
    });
  }

  /**
   * Get scrubbed recent inference logs.
   */
  async getAIInferenceLogs(): Promise<AIInferenceLogsResponse> {
    return this.request<AIInferenceLogsResponse>("/api/v1/ai/logs");
  }

  /**
   * Returns the endpoint URL for real-time SSE telemetry streaming.
   */
  getEventsUrl(): string {
    return this.baseUrl ? `${this.baseUrl}/api/v1/system/events` : "/api/v1/system/events";
  }
}


// Export singleton instance initialized with environment defaults
export const apiClient = new SovereignApiClient();
