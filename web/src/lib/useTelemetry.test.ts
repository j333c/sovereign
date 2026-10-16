import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { SovereignApiClient, TelemetryUpdate, ServiceHealthChangedEvent, ServiceErrorEvent } from "./api";
import { attachTelemetryListeners } from "./useTelemetry";

// Mock EventSource implementation for testing SSE hook integration
class MockEventSource {
  url: string;
  options?: { withCredentials?: boolean };
  listeners: Record<string, ((evt: any) => void)[]> = {};
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;

  constructor(url: string, options?: { withCredentials?: boolean }) {
    this.url = url;
    this.options = options;
  }

  addEventListener(event: string, cb: (evt: any) => void) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(cb);
  }

  removeEventListener(event: string, cb: (evt: any) => void) {
    if (this.listeners[event]) {
      this.listeners[event] = this.listeners[event].filter((l) => l !== cb);
    }
  }

  close() {
    this.closed = true;
  }

  // Helper to trigger message
  emit(event: string, data: unknown) {
    const handlers = this.listeners[event] || [];
    for (const h of handlers) {
      h({ data: JSON.stringify(data) });
    }
  }
}

describe("useTelemetry Hook & SSE streaming contract", () => {
  it("generates correct SSE events URL from SovereignApiClient", () => {
    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080" });
    assert.equal(client.getEventsUrl(), "http://localhost:8080/api/v1/system/events");

    const defaultClient = new SovereignApiClient({ baseUrl: "" });
    assert.equal(defaultClient.getEventsUrl(), "/api/v1/system/events");
  });

  it("handles mock EventSource connection and telemetry update messages", () => {
    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080" });
    let createdES: MockEventSource | null = null;

    const factory = (url: string) => {
      createdES = new MockEventSource(url, { withCredentials: true });
      return createdES as unknown as EventSource;
    };

    const mockES = factory(client.getEventsUrl()) as unknown as MockEventSource;
    assert.ok(mockES);
    assert.equal(mockES.url, "http://localhost:8080/api/v1/system/events");
    assert.equal(mockES.options?.withCredentials, true);

    let receivedTelemetry: TelemetryUpdate | null = null;
    mockES.addEventListener("telemetry.update", (evt: unknown) => {
      const msg = evt as { data: string };
      receivedTelemetry = JSON.parse(msg.data) as TelemetryUpdate;
    });

    const sampleUpdate: TelemetryUpdate = {
      cpu_percent: 22.4,
      memory_used_bytes: 8589934592,
      memory_total_bytes: 17179869184,
      storage_used_bytes: 1099511627776,
      storage_total_bytes: 4398046511104,
      storage_percent: 25.0,
      nas_status: "connected",
      nas_message: "Mounted nfs://nas.local/media",
      overall_status: "operational",
      optical_status: "Idle · Ready for disc",
      gpu_status: "Daytime (AI Priority)",
      timestamp: "2026-10-15T18:00:00Z",
    };

    mockES.emit("telemetry.update", sampleUpdate);
    assert.deepEqual(receivedTelemetry, sampleUpdate);

    mockES.close();
    assert.equal(mockES.closed, true);
  });

  it("dispatches service.health_changed, service.status_changed, and service.error events via MockEventSource", () => {
    const mockES = new MockEventSource("http://localhost:8080/api/v1/system/events");

    let receivedHealth: ServiceHealthChangedEvent | null = null;
    let receivedStatus: ServiceHealthChangedEvent | null = null;
    let receivedError: ServiceErrorEvent | null = null;

    attachTelemetryListeners(mockES, {
      onServiceHealthChanged: (evt) => {
        receivedHealth = evt;
      },
      onServiceStatusChanged: (evt) => {
        receivedStatus = evt;
      },
      onServiceError: (evt) => {
        receivedError = evt;
      },
    });

    const healthEvt: ServiceHealthChangedEvent = {
      service_id: "jellyfin",
      status: "degraded",
      health: "unhealthy",
      detail: "High memory pressure",
      timestamp: "2026-10-15T19:00:00Z",
    };
    mockES.emit("service.health_changed", healthEvt);
    assert.deepEqual(receivedHealth, healthEvt);

    const statusEvt: ServiceHealthChangedEvent = {
      service_id: "navidrome",
      status: "running",
      health: "healthy",
      timestamp: "2026-10-15T19:01:00Z",
    };
    mockES.emit("service.status_changed", statusEvt);
    assert.deepEqual(receivedStatus, statusEvt);

    const errorEvt: ServiceErrorEvent = {
      service_id: "paperless",
      message: "Worker thread crashed",
      timestamp: "2026-10-15T19:02:00Z",
    };
    mockES.emit("service.error", errorEvt);
    assert.deepEqual(receivedError, errorEvt);
  });

  it("dispatches dms.mailbox_status and dms.document_ingested events via MockEventSource", () => {
    const mockES = new MockEventSource("http://localhost:8080/api/v1/system/events");

    let receivedMailboxStatus: any = null;
    let receivedDocIngested: any = null;

    attachTelemetryListeners(mockES, {
      onDMSMailboxStatus: (evt) => {
        receivedMailboxStatus = evt;
      },
      onDMSDocumentIngested: (evt) => {
        receivedDocIngested = evt;
      },
    });

    const mailboxEvt = {
      connected: false,
      last_error: "imap authentication failed: NO [AUTHENTICATIONFAILED] Authentication failed",
      last_polled_at: "2026-10-16T10:00:00Z",
    };
    mockES.emit("dms.mailbox_status", mailboxEvt);
    assert.deepEqual(receivedMailboxStatus, mailboxEvt);

    const docEvt = {
      id: "test-doc-uuid",
      filename: "invoice.pdf",
      format: "pdf",
      size_bytes: 1048576,
      source: "imap",
      ingested_at: "2026-10-16T10:00:05Z",
    };
    mockES.emit("dms.document_ingested", docEvt);
    assert.deepEqual(receivedDocIngested, docEvt);
  });
});
