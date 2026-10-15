import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { SovereignApiClient, ProblemDetailsError } from "./client";

describe("SovereignApiClient", () => {
  it("successfully fetches getHealth with typed response", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      assert.equal(url, "http://localhost:8080/api/v1/health");

      return new Response(
        JSON.stringify({
          status: "ok",
          version: "0.1.0",
          database: "connected",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const health = await client.getHealth();
    assert.equal(health.status, "ok");
    assert.equal(health.version, "0.1.0");
    assert.equal(health.database, "connected");
  });

  it("successfully fetches getBootStatus with typed response", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      assert.equal(url, "http://localhost:8080/api/v1/system/boot-status");

      return new Response(
        JSON.stringify({
          mode: "operational",
          clean_shutdown: true,
          diagnostics_completed: true,
          message: "All systems operational",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getBootStatus();
    assert.equal(status.mode, "operational");
    assert.equal(status.clean_shutdown, true);
    assert.equal(status.diagnostics_completed, true);
    assert.equal(status.message, "All systems operational");
  });

  it("throws typed ProblemDetailsError on HTTP 503 problem response", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/database-unavailable",
          title: "Database Unavailable",
          status: 503,
          detail: "SQLite WAL database failed health probe.",
          instance: "/api/v1/health",
        }),
        {
          status: 503,
          headers: { "Content-Type": "application/problem+json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getHealth();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 503);
        assert.equal(err.title, "Database Unavailable");
        assert.equal(err.detail, "SQLite WAL database failed health probe.");
        assert.equal(err.instance, "/api/v1/health");
        return true;
      }
    );
  });

  it("handles non-JSON error responses gracefully as ProblemDetailsError", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response("Bad Gateway", {
        status: 502,
        statusText: "Bad Gateway",
      });
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getHealth();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 502);
        assert.equal(err.title, "Bad Gateway");
        return true;
      }
    );
  });

  it("normalizes trailing slashes in baseUrl", async () => {
    let requestedUrl = "";
    const mockFetch: typeof fetch = async (input) => {
      requestedUrl = String(input);
      return new Response(JSON.stringify({ status: "ok", version: "0.1.0", database: "connected" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080///",
      fetchFn: mockFetch,
    });

    assert.equal(client.baseUrl, "http://localhost:8080");
    await client.getHealth();
    assert.equal(requestedUrl, "http://localhost:8080/api/v1/health");
  });

  it("defaults baseUrl to http://localhost:8080 in node environment when options are omitted", () => {
    const client = new SovereignApiClient();
    assert.equal(client.baseUrl, "http://localhost:8080");
  });

  it("safely falls back when receiving non-RFC7807 JSON error response", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          error: "Unauthorized action",
        }),
        {
          status: 401,
          statusText: "Unauthorized",
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getHealth();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 401);
        assert.equal(err.title, "Unauthorized");
        assert.equal(err.detail, "Unauthorized action");
        assert.equal(err.instance, "/api/v1/health");
        return true;
      }
    );
  });
});
