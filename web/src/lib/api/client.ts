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
}

// Export singleton instance initialized with environment defaults
export const apiClient = new SovereignApiClient();
