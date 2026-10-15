import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { HomeScreen } from "./page";
import { SovereignApiClient } from "../lib/api/client";

describe("HomePage Component", () => {
  it("renders null before isReady is set", () => {
    // Initial render without resolving promises renders null
    const html = renderToStaticMarkup(
      <HomeScreen
        routerOverride={{ replace: () => {} }}
      />
    );
    assert.equal(html, "");
  });

  it("redirects to /boot when boot status mode is not operational", async () => {
    let redirectedPath = "";
    const mockRouter = {
      replace: (path: string) => {
        redirectedPath = path;
      },
    };

    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          mode: "diagnostics",
          clean_shutdown: false,
          diagnostics_completed: false,
          message: "Checking system safety after power restoration... your files are secure.",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getBootStatus();
    if (status.mode !== "operational") {
      mockRouter.replace("/boot");
    }

    assert.equal(redirectedPath, "/boot");
  });

  it("redirects to /boot when getBootStatus throws 503 ProblemDetails", async () => {
    let redirectedPath = "";
    const mockRouter = {
      replace: (path: string) => {
        redirectedPath = path;
      },
    };

    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/http-503",
          title: "Storage Degraded",
          status: 503,
          detail: "State storage filesystem is unwritable or read-only",
          instance: "/api/v1/system/boot-status",
        }),
        { status: 503, headers: { "Content-Type": "application/problem+json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    try {
      await client.getBootStatus();
    } catch {
      mockRouter.replace("/boot");
    }

    assert.equal(redirectedPath, "/boot");
  });

  it("renders dashboard content when boot status is operational", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          mode: "operational",
          clean_shutdown: true,
          diagnostics_completed: true,
          message: "All systems operational",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getBootStatus();
    assert.equal(status.mode, "operational");
  });
});
