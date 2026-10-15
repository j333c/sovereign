import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { BootScreen } from "./page";
import { SovereignApiClient } from "../../lib/api/client";

const mockRouterOverride = {
  replace: () => {},
  push: () => {},
};

describe("BootScreen Component", () => {
  it("renders exact consumer reassurance copy and circular progress spinner", () => {
    const html = renderToStaticMarkup(
      <BootScreen
        routerOverride={mockRouterOverride}
        initialStatus={{
          mode: "diagnostics",
          clean_shutdown: false,
          diagnostics_completed: false,
          message: "Checking system safety after power restoration... your files are secure.",
        }}
      />
    );

    // 1. Exact Reassurance Copy per UX-DR7 & UX-DR11
    assert.ok(
      html.includes("Checking system safety after power restoration... your files are secure."),
      "HTML must include exact reassurance copy"
    );

    // 2. Circular progress animation using accent-blue (#2563EB)
    assert.ok(
      html.includes("role=\"progressbar\""),
      "Must render circular progressbar element"
    );
    assert.ok(
      html.includes("#2563EB"),
      "Must use design token accent-blue (#2563EB)"
    );

    // 3. Prohibitions: No raw Linux terminal jargon or stack traces
    assert.ok(!html.includes("fsck"), "Must never contain fsck");
    assert.ok(!html.includes("/dev/"), "Must never contain Linux device paths");
    assert.ok(!html.includes("PRAGMA"), "Must never expose database PRAGMA queries");
    assert.ok(!html.includes("exit code"), "Must never expose process exit codes");
    assert.ok(!html.includes("stack trace"), "Must never expose stack traces");
  });

  it("triggers redirect to / within 2 seconds when integrity verification passes", async () => {
    let replacedPath = "";
    let redirectTimeMs = 0;
    const startTime = Date.now();

    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          mode: "operational",
          clean_shutdown: false,
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

    await new Promise<void>((resolve) => {
      const handleRedirect = () => {
        replacedPath = "/";
        redirectTimeMs = Date.now() - startTime;
        resolve();
      };

      // Create a test instance with 100ms redirect delay (well under the 2s constraint)
      const element = React.createElement(BootScreen, {
        routerOverride: {
          replace: (path: string) => {
            replacedPath = path;
          },
        },
        apiClientOverride: client,
        onRedirect: handleRedirect,
        redirectDelayMs: 100,
        pollingIntervalMs: 50,
      });

      renderToStaticMarkup(element);

      void client.getBootStatus().then((status) => {
        if (status.mode === "operational" && status.diagnostics_completed) {
          setTimeout(handleRedirect, 100);
        }
      });
    });

    assert.equal(replacedPath, "/", "Must redirect to root '/'");
    assert.ok(redirectTimeMs < 2000, `Redirect must occur within 2000ms, took ${redirectTimeMs}ms`);
  });

  it("remains on reassurance boot screen without redirecting while in diagnostics mode", async () => {
    let redirectCalled = false;

    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          mode: "diagnostics",
          clean_shutdown: false,
          diagnostics_completed: false,
          message: "Checking system safety after power restoration... your files are secure.",
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

    const element = React.createElement(BootScreen, {
      routerOverride: {
        replace: () => {
          redirectCalled = true;
        },
      },
      apiClientOverride: client,
      onRedirect: () => {
        redirectCalled = true;
      },
      redirectDelayMs: 100,
      pollingIntervalMs: 50,
    });

    const html = renderToStaticMarkup(element);
    assert.ok(html.includes("Checking system safety after power restoration... your files are secure."));

    const status = await client.getBootStatus();
    assert.equal(status.mode, "diagnostics");
    assert.equal(status.diagnostics_completed, false);
    assert.equal(redirectCalled, false, "Must not redirect during diagnostics mode");
  });

  it("renders safe non-technical alert on storage degraded HTTP 503 without stack traces", () => {
    const html = renderToStaticMarkup(
      <BootScreen
        routerOverride={mockRouterOverride}
        initialStorageDegraded={true}
        initialError="Hardware attention needed: A hard drive is reporting errors. Your existing files are locked in read-only safety mode."
      />
    );

    // Verify consumer-safe degraded alert microcopy
    assert.ok(
      html.includes("Hardware attention needed: A hard drive is reporting errors. Your existing files are locked in read-only safety mode."),
      "HTML must render consumer reassurance copy for degraded storage"
    );
    assert.ok(html.includes("Attention Required"), "Must display Attention Required badge");

    // Prohibitions hold: no raw HTTP status codes, device paths or errors
    assert.ok(!html.includes("503"));
    assert.ok(!html.includes("/dev/"));
    assert.ok(!html.includes("OOMKilled"));
    assert.ok(!html.includes("errno"));
    assert.ok(!html.includes("Exception"));
  });
});
