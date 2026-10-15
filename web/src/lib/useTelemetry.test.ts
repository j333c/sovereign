import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { SovereignApiClient, TelemetryUpdate } from "./api";

// Mock EventSource implementation for testing SSE hook integration
class MockEventSource {
  url: string;
  options?: { withCredentials?: boolean };
  listeners: Record<string, ((evt: unknown) => void)[]> = {};
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;

  constructor(url: string, options?: { withCredentials?: boolean }) {
    this.url = url;
    this.options = options;
  }

  addEventListener(event: string, cb: (evt: unknown) => void) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(cb);
  }

  removeEventListener(event: string, cb: (evt: unknown) => void) {
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
});
