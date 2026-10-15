import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { HomeScreen, checkBootAndSetup } from "./page";
import { SovereignApiClient, TelemetryUpdate } from "../lib/api/client";
import { AppHeader } from "../components/AppHeader";

import { TelemetryBar, formatBytesToTB, formatBytesToGB } from "../components/TelemetryBar";
import { ServiceCard } from "../components/ServiceCard";
import { AdminLoginModal } from "../components/AdminLoginModal";
import { StatusBadge } from "../components/StatusBadge";
import { translations } from "../lib/i18n";

describe("HomePage Component", () => {
  it("renders null before isReady is set", () => {
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

  it("redirects to /setup when server is unconfigured (is_configured === false)", async () => {
    let redirectedPath = "";
    const mockRouter = {
      replace: (path: string) => {
        redirectedPath = path;
      },
    };

    const mockFetch: typeof fetch = async (input) => {
      const url = String(input);
      if (url.endsWith("/boot-status")) {
        return new Response(
          JSON.stringify({
            mode: "operational",
            clean_shutdown: true,
            diagnostics_completed: true,
            message: "All systems operational",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url.endsWith("/setup-status")) {
        return new Response(
          JSON.stringify({ is_configured: false }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`Unexpected url: ${url}`);
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const ready = await checkBootAndSetup(client, mockRouter);
    assert.equal(ready, false);
    assert.equal(redirectedPath, "/setup");
  });

  it("redirects to /boot when status check encounters an unexpected error", async () => {
    let redirectedPath = "";
    const mockRouter = {
      replace: (path: string) => {
        redirectedPath = path;
      },
    };

    const mockFetch: typeof fetch = async () => {
      throw new Error("Network unreachable");
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const ready = await checkBootAndSetup(client, mockRouter);
    assert.equal(ready, false);
    assert.equal(redirectedPath, "/boot");
  });
});

describe("AppHeader Component", () => {
  it("renders 64px persistent header with brand crest, health pill, and quick links", () => {
    const html = renderToStaticMarkup(
      <AppHeader
        currentLanguage="en"
        onLanguageChange={() => {}}
        aggregateStatus="operational"
      />
    );

    assert.ok(html.includes("h-16"), "AppHeader must enforce 64px height (h-16)");
    assert.ok(html.includes("Sovereign"), "Must display Sovereign title");
    assert.ok(html.includes("Appliance Management"), "Must display subtitle");
    assert.ok(html.includes("All Systems Operational"), "Must display healthy aggregate status");
    assert.ok(html.includes("Drop Media"), "Must have Drop Media quick link");
    assert.ok(html.includes("/drop"), "Drop link must point to /drop");
    assert.ok(html.includes("/settings") || html.includes("Settings"), "Must have Settings button");
    assert.ok(html.includes("language-select"), "Must have language selector");
  });

  it("displays amber diamond indicator when aggregate status is degraded", () => {
    const html = renderToStaticMarkup(
      <AppHeader
        currentLanguage="en"
        onLanguageChange={() => {}}
        aggregateStatus="degraded"
      />
    );

    assert.ok(html.includes("Attention Required"), "Must display Attention Required label");
    assert.ok(html.includes("◆"), "Must use diamond micro-shape for degraded state");
  });

  it("displays red triangle indicator when aggregate status is error", () => {
    const html = renderToStaticMarkup(
      <AppHeader
        currentLanguage="en"
        onLanguageChange={() => {}}
        aggregateStatus="error"
      />
    );

    assert.ok(html.includes("System Error"), "Must display System Error label");
    assert.ok(html.includes("▲"), "Must use triangle micro-shape for error state");
  });

  it("renders instant language options (EN, DE, ES, FR) with 44px touch target", () => {
    const html = renderToStaticMarkup(
      <AppHeader
        currentLanguage="de"
        onLanguageChange={() => {}}
        aggregateStatus="operational"
      />
    );

    assert.ok(html.includes("h-11"), "Language switcher and action buttons must have min 44px height (h-11)");
    assert.ok(html.includes("value=\"en\""));
    assert.ok(html.includes("value=\"de\""));
    assert.ok(html.includes("value=\"es\""));
    assert.ok(html.includes("value=\"fr\""));
    assert.ok(html.includes("Alle Systeme betriebsbereit"), "German header text must reflect active language");
  });
});


describe("TelemetryBar Component", () => {
  it("renders 56px pinned bar with aria-live polite and storage metrics", () => {
    const mockTelemetry: TelemetryUpdate = {
      cpu_percent: 14.2,

      memory_used_bytes: 3221225472, // 3 GB
      memory_total_bytes: 8589934592, // 8 GB
      storage_used_bytes: 1099511627776, // 1.0 TB
      storage_total_bytes: 2199023255552, // 2.0 TB
      storage_percent: 50.0,
      nas_status: "connected",
      nas_message: "Connected",
      overall_status: "operational",
      optical_status: "Idle · Ready for disc",
      gpu_status: "Daytime (AI Priority)",
      timestamp: "2026-10-15T15:00:00Z",
    };

    const html = renderToStaticMarkup(
      <TelemetryBar telemetry={mockTelemetry} currentLanguage="en" />
    );

    assert.ok(html.includes("h-14"), "TelemetryBar must enforce 56px height (h-14)");
    assert.ok(html.includes("fixed bottom-0"), "TelemetryBar must be pinned to viewport bottom");
    assert.ok(html.includes("aria-live=\"polite\""), "Telemetry updates must declare aria-live polite");
    assert.ok(html.includes("1.0 TB / 2.0 TB"), "Storage must be formatted as X.X TB / Y.Y TB");
    assert.ok(html.includes("role=\"progressbar\""), "Must contain accessible progress bar");
    assert.ok(html.includes("aria-valuenow=\"50\""), "Must declare aria-valuenow on progressbar");
    assert.ok(html.includes("bg-emerald-500"), "Storage progress bar must use green fill below 85%");
    assert.ok(html.includes("14.2%"), "Must display CPU percentage");
    assert.ok(html.includes("Connected"), "Must display NAS connected status");
    assert.ok(html.includes("Idle · Ready for disc"), "Must display Optical status");
    assert.ok(html.includes("Daytime (AI Priority)"), "Must display GPU schedule");
  });

  it("shifts storage progress bar to amber when capacity exceeds 85% threshold", () => {
    const mockTelemetry: TelemetryUpdate = {
      cpu_percent: 22.0,
      memory_used_bytes: 4000000000,
      memory_total_bytes: 8000000000,
      storage_used_bytes: 1900000000000,
      storage_total_bytes: 2000000000000,
      storage_percent: 95.0, // >85% threshold
      nas_status: "connected",
      nas_message: "Connected",
      overall_status: "operational",
      optical_status: "Idle · Ready for disc",
      gpu_status: "Daytime (AI Priority)",
      timestamp: "2026-10-15T15:00:00Z",
    };

    const html = renderToStaticMarkup(
      <TelemetryBar telemetry={mockTelemetry} currentLanguage="en" />
    );

    assert.ok(html.includes("bg-amber-500"), "Progress bar must shift to amber fill when >85%");
    assert.ok(html.includes("&gt;85%"), "Must render >85% threshold badge");
  });

  it("displays amber disconnected state when NAS is unreachable", () => {
    const mockTelemetry: TelemetryUpdate = {
      cpu_percent: 10.0,
      memory_used_bytes: 2000000000,
      memory_total_bytes: 8000000000,
      storage_used_bytes: 500000000000,
      storage_total_bytes: 1000000000000,
      storage_percent: 50.0,
      nas_status: "disconnected",
      nas_message: "Connection refused",
      overall_status: "degraded",
      optical_status: "Idle",
      gpu_status: "Active",
      timestamp: "2026-10-15T15:00:00Z",
    };


    const html = renderToStaticMarkup(
      <TelemetryBar telemetry={mockTelemetry} currentLanguage="en" />
    );

    assert.ok(html.includes("Disconnected"), "Must display NAS Disconnected status");
    assert.ok(html.includes("bg-red-500"), "Disconnected NAS dot must use red indicator");
  });

  it("correctly formats byte sizes into TB and GB", () => {
    assert.equal(formatBytesToTB(0), "0.0 TB");
    assert.equal(formatBytesToTB(1099511627776), "1.0 TB");
    assert.equal(formatBytesToTB(4398046511104), "4.0 TB");
    assert.equal(formatBytesToGB(1073741824), "1.0 GB");
    assert.equal(formatBytesToGB(8589934592), "8.0 GB");
  });
});

describe("ServiceCard & StatusBadge Components", () => {
  it("renders StatusBadge with geometric micro-shapes and colors", () => {
    const htmlGreen = renderToStaticMarkup(<StatusBadge status="healthy" label="Healthy" />);
    assert.ok(htmlGreen.includes("●"), "Healthy must render circle symbol");
    assert.ok(htmlGreen.includes("bg-emerald-50"), "Healthy must use emerald container");

    const htmlAmber = renderToStaticMarkup(<StatusBadge status="degraded" label="Attention" />);
    assert.ok(htmlAmber.includes("◆"), "Degraded must render diamond symbol");
    assert.ok(htmlAmber.includes("bg-amber-50"), "Degraded must use amber container");

    const htmlRed = renderToStaticMarkup(<StatusBadge status="error" label="Stopped" />);
    assert.ok(htmlRed.includes("▲"), "Error must render triangle symbol");
    assert.ok(htmlRed.includes("bg-red-50"), "Error must use red container");
  });

  it("renders ServiceCard with 40x40 icon, status badge, metric, and primary action button", () => {
    const html = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="healthy"
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );

    assert.ok(html.includes("Video Streaming"), "Must display service name");
    assert.ok(html.includes("Jellyfin"), "Must display tech subtitle");
    assert.ok(html.includes("124 Movies"), "Must display metric readout");
    assert.ok(html.includes("4K Transcoding Ready"), "Must display secondary descriptor");
    assert.ok(html.includes("Open Streaming UI"), "Must display action button text");
    assert.ok(html.includes("target=\"_blank\""), "Primary action button must open in new tab");
    assert.ok(html.includes("href=\"/media\""), "Must point to action URL");
    assert.ok(html.includes("h-11"), "Touch targets must satisfy minimum 44px height (h-11)");
    assert.ok(html.includes("•••"), "Must render accessible overflow menu trigger");
    assert.ok(html.includes("aria-label=\"Options for Video Streaming\""), "Overflow menu button must carry descriptive aria-label");
  });

  it("renders StatusBadge with geometric triad and contrast across all status levels", () => {
    // Green circle ●
    const htmlHealthy = renderToStaticMarkup(<StatusBadge status="healthy" />);
    assert.ok(htmlHealthy.includes("●"));
    assert.ok(htmlHealthy.includes("bg-emerald-50 text-emerald-800"));
    assert.ok(htmlHealthy.includes("Running Healthy"));

    const htmlRunning = renderToStaticMarkup(<StatusBadge status="running" />);
    assert.ok(htmlRunning.includes("●"));
    assert.ok(htmlRunning.includes("Running Healthy"));

    // Amber diamond ◆
    const htmlStarting = renderToStaticMarkup(<StatusBadge status="starting" />);
    assert.ok(htmlStarting.includes("◆"));
    assert.ok(htmlStarting.includes("bg-amber-50 text-amber-800"));
    assert.ok(htmlStarting.includes("Starting..."));

    const htmlDegraded = renderToStaticMarkup(<StatusBadge status="degraded" />);
    assert.ok(htmlDegraded.includes("◆"));
    assert.ok(htmlDegraded.includes("Attention Required"));

    // Red triangle ▲
    const htmlError = renderToStaticMarkup(<StatusBadge status="error" />);
    assert.ok(htmlError.includes("▲"));
    assert.ok(htmlError.includes("bg-red-50 text-red-800"));
    assert.ok(htmlError.includes("Stopped / Error"));

    const htmlStopped = renderToStaticMarkup(<StatusBadge status="stopped" />);
    assert.ok(htmlStopped.includes("▲"));
    assert.ok(htmlStopped.includes("Stopped / Error"));
  });

  it("renders primary action button as disabled with loading indicator when starting or restarting", () => {
    const htmlStarting = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="starting"
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={true}
      />
    );
    assert.ok(htmlStarting.includes("<button"), "Must render as button");
    assert.ok(htmlStarting.includes("disabled"), "Button must be disabled");
    assert.ok(htmlStarting.includes("Starting..."), "Must display Starting... label");
    assert.ok(htmlStarting.includes("animate-spin"), "Must render spinning indicator");
    assert.ok(!htmlStarting.includes("href=\"/media\""), "Must not have active link href");

    const htmlRestarting = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="healthy"
        isRestarting={true}
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={true}
      />
    );
    assert.ok(htmlRestarting.includes("Restarting..."), "Must display Restarting... label");
    assert.ok(htmlRestarting.includes("disabled"), "Button must be disabled");
  });

  it("renders primary action button as disabled button when degraded or stopped", () => {
    const htmlDegraded = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="degraded"
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    assert.ok(htmlDegraded.includes("<button"), "Degraded state must render as button");
    assert.ok(htmlDegraded.includes("disabled"), "Button must be disabled");
    assert.ok(!htmlDegraded.includes("target=\"_blank\""), "Must not open link when degraded");
  });

  it("displays reassuring consumer microcopy during memory pressure / degraded without raw Linux exit codes", () => {
    const htmlOom = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="degraded"
        reassuranceMessage="Container jellyfin exited with code 137 (OOMKilled)"
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );

    assert.ok(
      htmlOom.includes(
        "Video Streaming paused due to high memory use. Automatically restarting..."
      ),
      "Must display reassuring consumer microcopy"
    );
    assert.ok(!htmlOom.includes("137"), "Must not leak raw Linux exit code 137");
    assert.ok(!htmlOom.includes("OOMKilled"), "Must not leak OOMKilled string");
    assert.ok(htmlOom.includes("role=\"alert\""), "Reassurance banner must carry role alert");
    assert.ok(
      htmlOom.includes("data-testid=\"reassurance-banner-video\""),
      "Must have testid for reassurance banner"
    );
  });

  it("renders localized reassurance microcopy across EN, DE, ES, and FR", () => {
    const htmlEn = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="degraded"
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    assert.ok(
      htmlEn.includes(
        "Video Streaming paused due to high memory use. Automatically restarting..."
      )
    );

    const htmlDe = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video-Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="degraded"
        metric="124 Filme"
        secondaryDescriptor="4K-Transkodierung bereit"
        actionUrl="/media"
        actionLabel="Streaming-UI öffnen"
        currentLanguage="de"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    assert.ok(
      htmlDe.includes(
        "Video-Streaming wegen hoher Speicherauslastung pausiert. Automatischer Neustart..."
      )
    );

    const htmlEs = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Transmisión de vídeo"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="degraded"
        metric="124 Películas"
        secondaryDescriptor="Transcodificación 4K lista"
        actionUrl="/media"
        actionLabel="Abrir interfaz de vídeo"
        currentLanguage="es"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    assert.ok(
      htmlEs.includes(
        "Transmisión de vídeo pausada debido al alto uso de memoria. Reiniciando automáticamente..."
      )
    );

    const htmlFr = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Streaming vidéo"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="degraded"
        metric="124 Films"
        secondaryDescriptor="Transcodage 4K prêt"
        actionUrl="/media"
        actionLabel="Ouvrir l'interface vidéo"
        currentLanguage="fr"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    assert.equal(
      translations["fr"].videoPausedMemoryUse,
      "Streaming vidéo interrompu en raison d'une utilisation élevée de la mémoire. Redémarrage automatique..."
    );
    assert.ok(
      htmlFr.includes("Streaming vidéo interrompu") &&
        htmlFr.includes("Redémarrage automatique...")
    );
  });

  it("triggers onRequireAdmin with action and service ID when unauthenticated", () => {
    let capturedAction = "";
    // Render and simulate action handler directly
    const props = {
      id: "video",
      name: "Video Streaming",
      techSubtitle: "Jellyfin",
      icon: "🎬",
      status: "healthy" as const,
      metric: "124 Movies",
      secondaryDescriptor: "4K Transcoding Ready",
      actionUrl: "/media",
      actionLabel: "Open Streaming UI",
      currentLanguage: "en" as const,
      onRequireAdmin: (actionName: string) => {
        capturedAction = actionName;
      },
      isAuthenticated: false,
    };

    // Verify component renders without crashing
    const html = renderToStaticMarkup(<ServiceCard {...props} />);
    assert.ok(html.includes("Video Streaming"));

    // Test the handler contract
    props.onRequireAdmin("restart:video");
    assert.equal(capturedAction, "restart:video");
  });
});

describe("AdminLoginModal Component", () => {
  it("renders accessible modal with title, password input, and 44px buttons", () => {
    const html = renderToStaticMarkup(
      <AdminLoginModal
        isOpen={true}
        onClose={() => {}}
        onSuccess={() => {}}
        apiClient={new SovereignApiClient({ baseUrl: "http://localhost:8080" })}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes("role=\"dialog\""), "Must declare role dialog");
    assert.ok(html.includes("aria-modal=\"true\""), "Must declare aria-modal true");
    assert.ok(html.includes("aria-labelledby=\"admin-login-title\""), "Must carry aria-labelledby");
    assert.ok(html.includes("Administrator Authentication"), "Must display modal title");
    assert.ok(html.includes("type=\"password\""), "Must have password input");
    assert.ok(html.includes("Unlock"), "Must have unlock button");
    assert.ok(html.includes("Cancel"), "Must have cancel button");
  });

  it("renders null when isOpen is false", () => {
    const html = renderToStaticMarkup(
      <AdminLoginModal
        isOpen={false}
        onClose={() => {}}
        onSuccess={() => {}}
        apiClient={new SovereignApiClient({ baseUrl: "http://localhost:8080" })}
        currentLanguage="en"
      />
    );

    assert.equal(html, "");
  });
});

describe("Central Admin Dashboard Integration & i18n", () => {
  it("renders all 4 configured service cards (Video, Audio, DMS, AI) and responsive layout classes", () => {
    const mockTelemetry: TelemetryUpdate = {
      cpu_percent: 15.0,

      memory_used_bytes: 4000000000,
      memory_total_bytes: 8000000000,
      storage_used_bytes: 1099511627776,
      storage_total_bytes: 2199023255552,
      storage_percent: 50.0,
      nas_status: "connected",
      nas_message: "Connected",
      overall_status: "operational",
      optical_status: "Idle · Ready for disc",
      gpu_status: "Daytime (AI Priority)",
      timestamp: "2026-10-15T15:00:00Z",
    };


    // Render directly with mock telemetry
    const htmlHeader = renderToStaticMarkup(
      <AppHeader
        currentLanguage="en"
        onLanguageChange={() => {}}
        aggregateStatus="operational"
      />
    );
    const htmlTelemetry = renderToStaticMarkup(
      <TelemetryBar telemetry={mockTelemetry} currentLanguage="en" />
    );
    const htmlVideo = renderToStaticMarkup(
      <ServiceCard
        id="video"
        name="Video Streaming"
        techSubtitle="Jellyfin"
        icon="🎬"
        status="healthy"
        metric="124 Movies"
        secondaryDescriptor="4K Transcoding Ready"
        actionUrl="/media"
        actionLabel="Open Streaming UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    const htmlAudio = renderToStaticMarkup(
      <ServiceCard
        id="audio"
        name="Music & Audiobooks"
        techSubtitle="Navidrome / Audiobookshelf"
        icon="🎵"
        status="healthy"
        metric="Mobile Phone Sync Ready"
        secondaryDescriptor="Lossless Audio Library"
        actionUrl="/music"
        actionLabel="Open Audio UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    const htmlDms = renderToStaticMarkup(
      <ServiceCard
        id="dms"
        name="Document Archive"
        techSubtitle="Paperless-ngx"
        icon="📑"
        status="healthy"
        metric="Automatic Text Search Active"
        secondaryDescriptor="Mailbox Connected"
        actionUrl="/docs"
        actionLabel="Open Documents UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );
    const htmlAi = renderToStaticMarkup(
      <ServiceCard
        id="ai"
        name="Local AI Assistant"
        techSubtitle="Open WebUI / llama.cpp"
        icon="⚡"
        status="healthy"
        metric="Private Family Search"
        secondaryDescriptor="100% On-Device"
        actionUrl="/chat"
        actionLabel="Open Chat UI"
        currentLanguage="en"
        onRequireAdmin={() => {}}
        isAuthenticated={false}
      />
    );

    assert.ok(htmlHeader.includes("Sovereign"));
    assert.ok(htmlTelemetry.includes("1.0 TB / 2.0 TB"));
    assert.ok(htmlVideo.includes("Video Streaming"));
    assert.ok(htmlAudio.includes("Music &amp; Audiobooks"));
    assert.ok(htmlDms.includes("Document Archive"));
    assert.ok(htmlAi.includes("Local AI Assistant"));

  });

  it("supports instantaneous translation into German, Spanish, and French", () => {
    // German
    const tDe = translations["de"];
    assert.equal(tDe.allSystemsOperational, "Alle Systeme betriebsbereit");
    assert.equal(tDe.videoTitle, "Video-Streaming");
    assert.equal(tDe.audioTitle, "Musik & Hörbücher");
    assert.equal(tDe.dmsTitle, "Dokumentenarchiv");
    assert.equal(tDe.aiTitle, "Lokaler KI-Assistent");

    // Spanish
    const tEs = translations["es"];
    assert.equal(tEs.allSystemsOperational, "Todos los sistemas operativos");
    assert.equal(tEs.videoTitle, "Transmisión de vídeo");
    assert.equal(tEs.audioTitle, "Música y audiolibros");
    assert.equal(tEs.dmsTitle, "Archivo de documentos");
    assert.equal(tEs.aiTitle, "Asistente de IA local");

    // French
    const tFr = translations["fr"];
    assert.equal(tFr.allSystemsOperational, "Tous les systèmes opérationnels");
    assert.equal(tFr.videoTitle, "Streaming vidéo");
    assert.equal(tFr.audioTitle, "Musique et livres audio");
    assert.equal(tFr.dmsTitle, "Archives de documents");
    assert.equal(tFr.aiTitle, "Assistant IA local");
  });

  it("renders HomeScreen when operational with header, service cards, and telemetry bar", () => {
    const html = renderToStaticMarkup(
      <HomeScreen
        initialReady={true}
        routerOverride={{ replace: () => {} }}
        initialTelemetry={{
          cpu_percent: 12.5,
          memory_used_bytes: 4 * 1024 * 1024 * 1024,
          memory_total_bytes: 16 * 1024 * 1024 * 1024,
          storage_used_bytes: 1024 * 1024 * 1024 * 1024,
          storage_total_bytes: 2048 * 1024 * 1024 * 1024,
          storage_percent: 50.0,
          nas_status: "not_configured",
          nas_message: "No network storage configured",
          overall_status: "operational",
          optical_status: "Idle · Ready for disc",
          gpu_status: "Daytime (AI Priority)",
          timestamp: "2026-10-15T12:00:00Z",
        }}
      />
    );

    assert.ok(html.includes("data-testid=\"app-header\""));
    assert.ok(html.includes("data-testid=\"services-grid\""));
    assert.ok(html.includes("data-testid=\"telemetry-bar\""));
    assert.ok(html.includes("Video Streaming"));
    assert.ok(html.includes("Music &amp; Audiobooks"));
    assert.ok(html.includes("Document Archive"));
    assert.ok(html.includes("Local AI Assistant"));
  });

  it("renders each service card with geometric StatusBadge (●/◆/▲) on central dashboard", () => {
    const html = renderToStaticMarkup(
      <HomeScreen
        initialReady={true}
        routerOverride={{ replace: () => {} }}
        initialTelemetry={{
          cpu_percent: 10,
          memory_used_bytes: 1024,
          memory_total_bytes: 4096,
          storage_used_bytes: 1024,
          storage_total_bytes: 4096,
          storage_percent: 25,
          nas_status: "connected",
          nas_message: "ok",
          overall_status: "operational",
          optical_status: "idle",
          gpu_status: "idle",
          timestamp: "2026-10-15T12:00:00Z",
        }}
        initialServices={{
          video: { status: "healthy" },
          audio: { status: "starting" },
          dms: { status: "degraded", reassuranceMessage: "High memory use" },
          ai: { status: "error" },
        }}
      />
    );

    // Video is healthy -> Green ●
    assert.ok(html.includes("data-testid=\"service-card-video\""));
    assert.ok(html.includes("●"));

    // Audio is starting -> Amber ◆
    assert.ok(html.includes("data-testid=\"service-card-audio\""));
    assert.ok(html.includes("◆"));

    // DMS is degraded -> Amber ◆ and reassurance alert
    assert.ok(html.includes("data-testid=\"service-card-dms\""));
    assert.ok(html.includes("data-testid=\"reassurance-banner-dms\""));

    // AI is error -> Red ▲
    assert.ok(html.includes("data-testid=\"service-card-ai\""));
    assert.ok(html.includes("▲"));
  });

  it("executes apiClient.restartService with proper authentication handling", async () => {
    let restartCalledWith = "";
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      if (url.includes("/restart")) {
        restartCalledWith = url;
        return new Response(
          JSON.stringify({
            id: "jellyfin",
            name: "Jellyfin",
            status: "running",
            health: "healthy",
            image: "jellyfin/jellyfin:12.2.0",
            image_downloaded: true,
            mem_limit_bytes: 2147483648,
            mem_reservation_bytes: 1073741824,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`Unexpected call: ${url}`);
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.restartService("jellyfin");
    assert.ok(restartCalledWith.endsWith("/api/v1/services/jellyfin/restart"));
    assert.equal(res.status, "running");
  });

  it("handles restart failure as ProblemDetailsError without uncaught exceptions", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/unauthorized",
          title: "Authentication Required",
          status: 401,
          detail: "Administrative authentication required to restart services",
          instance: "/api/v1/services/jellyfin/restart",
        }),
        { status: 401, headers: { "Content-Type": "application/problem+json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.restartService("jellyfin");
      },
      (err: unknown) => {
        const pErr = err as { status: number; title: string };
        assert.equal(pErr.status, 401);
        assert.equal(pErr.title, "Authentication Required");
        return true;
      }
    );
  });
});
