import type { components } from "./schema";

export type HealthResponse = components["schemas"]["HealthResponse"];
export type BootStatusResponse = components["schemas"]["BootStatusResponse"];
export type ProblemDetails = components["schemas"]["ProblemDetails"];

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

    return (await response.json()) as T;
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
}

// Export singleton instance initialized with environment defaults
export const apiClient = new SovereignApiClient();
