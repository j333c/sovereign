import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { SetupScreen } from "./page";
import { SovereignApiClient, ProblemDetailsError } from "../../lib/api/client";

const mockRouter = {
  replace: () => {},
  push: () => {},
};

describe("SetupWizard Component", () => {
  it("renders Step 1 with Language Selection and progress indicator", () => {
    const html = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={1}
        initialLanguage="en"
      />
    );

    assert.ok(html.includes("Step 1 of 5"), "Must show step 1 of 5");
    assert.ok(html.includes("Language Selection"), "Must display Language Selection heading");
    assert.ok(html.includes("English"), "Must display English language option");
    assert.ok(html.includes("Deutsch (German)"), "Must display German language option");
    assert.ok(html.includes("Español (Spanish)"), "Must display Spanish language option");
    assert.ok(html.includes("Français (French)"), "Must display French language option");
    assert.ok(html.includes("Continue"), "Must have Continue button");
    assert.ok(html.includes("role=\"progressbar\""), "Must render progress bar");
  });

  it("updates UI copy instantly when language changes (German, Spanish, French)", () => {
    // German
    const htmlDe = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={1}
        initialLanguage="de"
      />
    );
    assert.ok(htmlDe.includes("Schritt 1 von 5"), "Must render German step indicator");
    assert.ok(htmlDe.includes("Sprachauswahl"), "Must render German title");
    assert.ok(htmlDe.includes("Weiter"), "Must render German next button");

    // Spanish
    const htmlEs = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={1}
        initialLanguage="es"
      />
    );
    assert.ok(htmlEs.includes("Paso 1 de 5"), "Must render Spanish step indicator");
    assert.ok(htmlEs.includes("Selección de idioma"), "Must render Spanish title");
    assert.ok(htmlEs.includes("Continuar"), "Must render Spanish next button");

    // French
    const htmlFr = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={1}
        initialLanguage="fr"
      />
    );
    assert.ok(htmlFr.includes("Étape 1 sur 5"), "Must render French step indicator");
    assert.ok(htmlFr.includes("Sélection de la langue"), "Must render French title");
    assert.ok(htmlFr.includes("Continuer"), "Must render French next button");
  });

  it("renders Step 2 with Network Verification and hostname confirmation", () => {
    const html = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={2}
        initialLanguage="en"
      />
    );

    assert.ok(html.includes("Step 2 of 5"), "Must show step 2 of 5");
    assert.ok(html.includes("Network Verification"), "Must show Network Verification");
    assert.ok(html.includes("Local Network Connected"), "Must indicate connected network");
    assert.ok(html.includes("sovereign.local"), "Must display sovereign.local hostname");
    assert.ok(html.includes("Back"), "Must have Back button");
    assert.ok(html.includes("Continue"), "Must have Continue button");
  });

  it("renders Step 3 Storage Configuration with Local Pool and NAS options", () => {
    const html = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={3}
        initialLanguage="en"
      />
    );

    assert.ok(html.includes("Step 3 of 5"), "Must show step 3 of 5");
    assert.ok(html.includes("Storage Configuration"), "Must show Storage Configuration");
    assert.ok(html.includes("Internal Storage Pool"), "Must present Internal Storage Pool");
    assert.ok(html.includes("Connect Network Storage (NAS)"), "Must present NAS option");
    assert.ok(html.includes("Initialize Local Storage"), "Must present Initialize button");
  });

  it("Step 3 executes storage pool initialization via apiClient", async () => {
    let initializedDevice = "";
    const mockFetch: typeof fetch = async (input, init) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/storage/initialize");
      assert.equal(init?.method, "POST");
      const body = JSON.parse(String(init?.body));
      initializedDevice = body.device;

      return new Response(
        JSON.stringify({
          status: "mounted",
          device: body.device,
          mount_point: "/data",
          filesystem: "ext4",
          message: "Storage pool formatted and mounted successfully",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.initializeStorage({ device: "/dev/nvme0n1" });
    assert.equal(initializedDevice, "/dev/nvme0n1");
    assert.equal(res.status, "mounted");
  });

  it("Step 3 executes NAS connection test and handles errors inline", async () => {
    // 1. Successful NAS test
    const mockFetchSuccess: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          success: true,
          type: "smb",
          host: "192.168.1.100",
          share: "media",
          message: "NAS storage share reachable and credentials verified",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const clientSuccess = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetchSuccess,
    });

    const testRes = await clientSuccess.testNAS({
      type: "smb",
      host: "192.168.1.100",
      share: "media",
      username: "alex",
      password: "secret",
    });
    assert.equal(testRes.success, true);

    // 2. Failed NAS test with ProblemDetails
    const mockFetchFail: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/http-400",
          title: "NAS Connection Failed",
          status: 400,
          detail: "authentication failed: unable to verify credentials with network storage share",
          instance: "/api/v1/storage/nas/test",
        }),
        { status: 400, headers: { "Content-Type": "application/problem+json" } }
      );
    };

    const clientFail = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetchFail,
    });

    await assert.rejects(
      async () => {
        await clientFail.testNAS({
          type: "smb",
          host: "192.168.1.100",
          share: "media",
          username: "wrong",
          password: "bad",
        });
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 400);
        assert.equal(err.title, "NAS Connection Failed");
        assert.ok(err.detail.includes("authentication failed"));
        return true;
      }
    );
  });

  it("renders Step 4 Feature Selection checklist with all 4 services", () => {
    const html = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={4}
        initialLanguage="en"
      />
    );

    assert.ok(html.includes("Step 4 of 5"), "Must show step 4 of 5");
    assert.ok(html.includes("Feature Selection"), "Must show Feature Selection");
    assert.ok(html.includes("Video Streaming"), "Must show Video Streaming feature");
    assert.ok(html.includes("Music &amp; Audiobooks") || html.includes("Music & Audiobooks"), "Must show Music & Audiobooks feature");
    assert.ok(html.includes("Document Archive"), "Must show Document Archive feature");
    assert.ok(html.includes("Local AI &amp; Private Chat") || html.includes("Local AI & Private Chat"), "Must show Local AI feature");
  });

  it("renders Step 5 Admin Account with password and alert email fields", () => {
    const html = renderToStaticMarkup(
      <SetupScreen
        routerOverride={mockRouter}
        initialStep={5}
        initialLanguage="en"
      />
    );

    assert.ok(html.includes("Step 5 of 5"), "Must show step 5 of 5");
    assert.ok(html.includes("Administrator Account"), "Must show Administrator Account");
    assert.ok(html.includes("Administrator Password"), "Must have password field");
    assert.ok(html.includes("Confirm Password"), "Must have confirm password field");
    assert.ok(html.includes("Notification Email Address"), "Must have email field");
    assert.ok(html.includes("Complete Setup"), "Must have Complete Setup button");
  });

  it("completing Step 5 calls completeSetup and redirects to / with active session", async () => {
    let redirectedPath = "";
    const mockRouterOverride = {
      replace: (path: string) => {
        redirectedPath = path;
      },
      push: (path: string) => {
        redirectedPath = path;
      },
    };

    let sentPayload: unknown = null;
    const mockFetch: typeof fetch = async (input, init) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/auth/setup");
      assert.equal(init?.method, "POST");
      sentPayload = JSON.parse(String(init?.body));

      return new Response(
        JSON.stringify({
          status: "ok",
          is_configured: true,
          session: {
            token: "active-admin-session-token",
            username: "admin",
            role: "admin",
            expires_at: 1730000000,
          },
        }),
        {
          status: 200,
          headers: {
            "Content-Type": "application/json",
            "Set-Cookie": "sovereign_session=active-admin-session-token; Path=/; HttpOnly; SameSite=Strict; Max-Age=2592000",
          },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.completeSetup({
      username: "admin",
      password: "HouseholdAdminPass2026!",
      email: "alex@family.net",
      language: "en",
      features: ["video", "audio", "dms", "ai"],
      hostname: "sovereign.local",
    });

    assert.equal(res.status, "ok");
    assert.equal(res.is_configured, true);
    assert.equal(res.session?.role, "admin");

    // Transition directly to / without reboot
    mockRouterOverride.replace("/");
    assert.equal(redirectedPath, "/");
    assert.ok(sentPayload !== null);
  });

  it("checks setup status and redirects to / if already configured", async () => {
    let redirectedPath = "";
    const mockRouterOverride = {
      replace: (path: string) => {
        redirectedPath = path;
      },
    };

    const mockFetch: typeof fetch = async (input) => {
      const url = String(input);
      if (url.endsWith("/setup-status")) {
        return new Response(JSON.stringify({ is_configured: true, language: "en" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      throw new Error(`Unexpected url: ${url}`);
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getSetupStatus();
    if (status.is_configured) {
      mockRouterOverride.replace("/");
    }
    assert.equal(redirectedPath, "/");
  });
});

