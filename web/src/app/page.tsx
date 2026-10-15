"use client";

import React, { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  apiClient as defaultApiClient,
  SovereignApiClient,
  TelemetryUpdate,
  ProblemDetailsError,
  ServiceHealthChangedEvent,
  ServiceErrorEvent,
} from "@/lib/api";
import { Language, translations } from "@/lib/i18n";
import { useTelemetry } from "@/lib/useTelemetry";
import { AppHeader } from "@/components/AppHeader";
import { TelemetryBar } from "@/components/TelemetryBar";
import { ServiceCard } from "@/components/ServiceCard";
import { AdminLoginModal } from "@/components/AdminLoginModal";
import { StatusLevel } from "@/components/StatusBadge";

export interface ServiceStateOverride {
  status?: StatusLevel;
  reassuranceMessage?: string;
  isRestarting?: boolean;
}

export interface HomePageProps {
  apiClientOverride?: SovereignApiClient;
  routerOverride?: { replace: (url: string) => void };
  initialTelemetry?: TelemetryUpdate | null;
  eventSourceFactory?: (url: string) => EventSource;
  initialLanguage?: Language;
  initialAuthenticated?: boolean;
  initialReady?: boolean;
  initialServices?: Record<string, ServiceStateOverride>;
}

export function HomeScreen(props: HomePageProps) {
  if (props.routerOverride) {
    return <HomeScreenContent {...props} router={props.routerOverride} />;
  }
  return <HomeScreenWithRouter {...props} />;
}

function HomeScreenWithRouter(props: HomePageProps) {
  const router = useRouter();
  return <HomeScreenContent {...props} router={router} />;
}

export async function checkBootAndSetup(
  client: SovereignApiClient,
  router?: { replace: (url: string) => void }
): Promise<boolean> {
  try {
    const status = await client.getBootStatus();
    if (status.mode !== "operational") {
      router?.replace("/boot");
      return false;
    }

    const setup = await client.getSetupStatus();
    if (!setup.is_configured) {
      router?.replace("/setup");
      return false;
    }

    return true;
  } catch {
    router?.replace("/boot");
    return false;
  }
}

const serviceIdMap: Record<string, string> = {
  jellyfin: "video",
  video: "video",
  navidrome: "audio",
  audiobookshelf: "audio",
  audio: "audio",
  paperless: "dms",
  dms: "dms",
  "open-webui": "ai",
  "llama-server": "ai",
  ai: "ai",
};

const backendServiceMap: Record<string, string> = {
  video: "jellyfin",
  audio: "navidrome",
  dms: "paperless",
  ai: "llama-server",
};

function HomeScreenContent({
  apiClientOverride,
  router,
  initialTelemetry = null,
  eventSourceFactory,
  initialLanguage = "en",
  initialAuthenticated = false,
  initialReady = false,
  initialServices = {},
}: HomePageProps & { router?: { replace: (url: string) => void } }) {
  const client = apiClientOverride ?? defaultApiClient;
  const [isReady, setIsReady] = useState(initialReady);
  const [language, setLanguage] = useState<Language>(() => {
    if (typeof window !== "undefined") {
      const saved = localStorage.getItem("sovereign_lang") as Language;
      if (saved && ["en", "de", "es", "fr"].includes(saved)) {
        return saved;
      }
    }
    return initialLanguage;
  });
  const [isAuthenticated, setIsAuthenticated] = useState(initialAuthenticated);
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [enabledFeatures, setEnabledFeatures] = useState<string[] | null>(null);
  const [serviceStates, setServiceStates] = useState<
    Record<string, ServiceStateOverride>
  >(initialServices);
  const [restartError, setRestartError] = useState<{
    title: string;
    detail: string;
  } | null>(null);

  const handleLanguageChange = useCallback((lang: Language) => {
    setLanguage(lang);
    if (typeof window !== "undefined") {
      localStorage.setItem("sovereign_lang", lang);
    }
  }, []);

  // SSE event handlers for service lifecycle and health changes
  const handleServiceHealthChanged = useCallback(
    (evt: ServiceHealthChangedEvent) => {
      if (!evt?.service_id) return;
      const cardId = serviceIdMap[evt.service_id];
      if (!cardId) return;

      let mappedStatus: StatusLevel = "healthy";
      let reassuranceMsg: string | undefined = undefined;

      const statusStr = (evt.status || "").toLowerCase();
      const healthStr = (evt.health || "").toLowerCase();

      if (statusStr === "starting") {
        mappedStatus = "starting";
      } else if (
        statusStr === "degraded" ||
        healthStr === "degraded"
      ) {
        mappedStatus = "degraded";
        reassuranceMsg = evt.detail;
      } else if (
        statusStr === "error" ||
        statusStr === "stopped" ||
        healthStr === "unhealthy"
      ) {
        mappedStatus = "error";
        reassuranceMsg = evt.detail;
      } else if (
        statusStr === "running" &&
        (healthStr === "healthy" || !healthStr)
      ) {
        mappedStatus = "healthy";
      }

      setServiceStates((prev) => ({
        ...prev,
        [cardId]: {
          ...prev[cardId],
          status: mappedStatus,
          isRestarting:
            mappedStatus === "starting" ? prev[cardId]?.isRestarting : false,
          reassuranceMessage: reassuranceMsg,
        },
      }));
    },
    []
  );

  const handleServiceError = useCallback((evt: ServiceErrorEvent) => {
    if (!evt?.service_id) return;
    const cardId = serviceIdMap[evt.service_id];
    if (!cardId) return;

    setServiceStates((prev) => ({
      ...prev,
      [cardId]: {
        ...prev[cardId],
        status: "error",
        reassuranceMessage: evt.message,
        isRestarting: false,
      },
    }));
  }, []);

  // Subscribe to real-time SSE telemetry & service lifecycle streams
  const { telemetry } = useTelemetry({
    client,
    initialTelemetry,
    eventSourceFactory,
    onServiceHealthChanged: handleServiceHealthChanged,
    onServiceStatusChanged: handleServiceHealthChanged,
    onServiceError: handleServiceError,
  });

  // Verify boot status and setup state on initial mount
  useEffect(() => {
    let isMounted = true;

    void checkBootAndSetup(client, router).then((ready) => {
      if (isMounted && ready) {
        setIsReady(true);
      }
    });

    // Check if session is already active
    void client
      .getSession()
      .then((res) => {
        if (isMounted && res.authenticated) {
          setIsAuthenticated(true);
        }
      })
      .catch(() => {
        // Unauthenticated LAN caller is completely valid
      });

    // Attempt to load current service statuses
    void client
      .getServices()
      .then((res) => {
        if (!isMounted || !res?.services) return;
        const updates: Record<string, ServiceStateOverride> = {};
        for (const s of res.services) {
          const cardId = serviceIdMap[s.id];
          if (!cardId) continue;
          let status: StatusLevel = "healthy";
          if (s.status === "starting") status = "starting";
          else if (s.status === "degraded" || s.health === "degraded")
            status = "degraded";
          else if (
            s.status === "error" ||
            s.status === "stopped" ||
            s.health === "unhealthy"
          )
            status = "error";
          else if (s.status === "running") status = "healthy";
          updates[cardId] = { status };
        }
        setServiceStates((prev) => ({ ...prev, ...updates }));
      })
      .catch(() => {
        // Fallback to telemetry/default state
      });

    return () => {
      isMounted = false;
    };
  }, [client, router]);

  // Load configured features if authenticated
  useEffect(() => {
    if (!isAuthenticated) return;
    let isMounted = true;
    void client
      .getSettings()
      .then((settings) => {
        if (isMounted && settings?.features) {
          const list = settings.features
            .split(",")
            .map((s) => s.trim().toLowerCase());
          setEnabledFeatures(list);
        }
      })
      .catch(() => {
        // Unauthenticated or settings error
      });
    return () => {
      isMounted = false;
    };
  }, [client, isAuthenticated]);

  const executeRestart = useCallback(
    async (serviceId: string) => {
      const backendId = backendServiceMap[serviceId] || serviceId;
      setServiceStates((prev) => ({
        ...prev,
        [serviceId]: {
          ...prev[serviceId],
          status: "starting",
          isRestarting: true,
        },
      }));

      try {
        await client.restartService(backendId);
      } catch (err: unknown) {
        let title = "Failed to restart service";
        let detail = "An unexpected error occurred while restarting the service.";
        if (err instanceof ProblemDetailsError) {
          title = err.title || title;
          detail = err.detail || detail;
        } else if (err instanceof Error) {
          detail = err.message;
        }
        setRestartError({ title, detail });
        setServiceStates((prev) => ({
          ...prev,
          [serviceId]: {
            ...prev[serviceId],
            isRestarting: false,
          },
        }));
      }
    },
    [client]
  );

  const handleRestart = useCallback(
    async (serviceId: string) => {
      if (!isAuthenticated) {
        setPendingAction(`restart:${serviceId}`);
        setIsLoginModalOpen(true);
        return;
      }
      await executeRestart(serviceId);
    },
    [isAuthenticated, executeRestart]
  );

  const handleSettingsClick = useCallback(() => {
    if (isAuthenticated) {
      router?.replace("/settings");
    } else {
      setPendingAction("settings");
      setIsLoginModalOpen(true);
    }
  }, [isAuthenticated, router]);

  const handleRequireAdmin = useCallback((actionName: string) => {
    setPendingAction(actionName);
    setIsLoginModalOpen(true);
  }, []);

  const handleLoginSuccess = useCallback(() => {
    setIsAuthenticated(true);
    setIsLoginModalOpen(false);

    if (pendingAction === "settings") {
      setPendingAction(null);
      router?.replace("/settings");
    } else if (pendingAction?.startsWith("restart:")) {
      const serviceId = pendingAction.slice("restart:".length);
      setPendingAction(null);
      void executeRestart(serviceId);
    } else if (pendingAction === "restart") {
      setPendingAction(null);
      void executeRestart("video");
    } else {
      setPendingAction(null);
    }
  }, [pendingAction, router, executeRestart]);

  if (!isReady) {
    return null;
  }

  const t = translations[language];

  // Aggregate health derived from telemetry or default to operational
  const rawStatus = telemetry?.overall_status ?? "operational";
  let aggregateStatus: StatusLevel = "operational";
  if (rawStatus === "degraded") {
    aggregateStatus = "degraded";
  } else if (rawStatus === "error") {
    aggregateStatus = "error";
  }

  // Determine individual service states based on subsystem telemetry and live SSE state
  const nasIsDegraded =
    telemetry?.nas_status === "degraded" ||
    telemetry?.nas_status === "disconnected";

  const services = [
    {
      id: "video",
      name: t.videoTitle,
      techSubtitle: t.videoSubtitle,
      icon: "🎬",
      status: (serviceStates.video?.status ??
        (nasIsDegraded ? "degraded" : "healthy")) as StatusLevel,
      metric: t.videoMetric,
      secondaryDescriptor: t.videoDesc,
      actionUrl: "/media",
      actionLabel: t.videoAction,
      isRestarting: Boolean(serviceStates.video?.isRestarting),
      reassuranceMessage: serviceStates.video?.reassuranceMessage,
    },
    {
      id: "audio",
      name: t.audioTitle,
      techSubtitle: t.audioSubtitle,
      icon: "🎵",
      status: (serviceStates.audio?.status ??
        (nasIsDegraded ? "degraded" : "healthy")) as StatusLevel,
      metric: t.audioMetric,
      secondaryDescriptor: t.audioDesc,
      actionUrl: "/music",
      actionLabel: t.audioAction,
      isRestarting: Boolean(serviceStates.audio?.isRestarting),
      reassuranceMessage: serviceStates.audio?.reassuranceMessage,
    },
    {
      id: "dms",
      name: t.dmsTitle,
      techSubtitle: t.dmsSubtitle,
      icon: "📑",
      status: (serviceStates.dms?.status ?? "healthy") as StatusLevel,
      metric: t.dmsMetric,
      secondaryDescriptor: t.dmsDesc,
      actionUrl: "/docs",
      actionLabel: t.dmsAction,
      isRestarting: Boolean(serviceStates.dms?.isRestarting),
      reassuranceMessage: serviceStates.dms?.reassuranceMessage,
    },
    {
      id: "ai",
      name: t.aiTitle,
      techSubtitle: t.aiSubtitle,
      icon: "⚡",
      status: (serviceStates.ai?.status ?? "healthy") as StatusLevel,
      metric: t.aiMetric,
      secondaryDescriptor: t.aiDesc,
      actionUrl: "/chat",
      actionLabel: t.aiAction,
      isRestarting: Boolean(serviceStates.ai?.isRestarting),
      reassuranceMessage: serviceStates.ai?.reassuranceMessage,
    },
  ];

  const visibleServices = services.filter((svc) => {
    if (!enabledFeatures || enabledFeatures.length === 0) return true;
    return enabledFeatures.includes(svc.id);
  });

  return (
    <div className="flex min-h-screen flex-col bg-[#F8F9FA] text-[#0F172A] lg:h-screen lg:overflow-hidden">
      {/* Persistent 64px Header */}
      <AppHeader
        currentLanguage={language}
        onLanguageChange={handleLanguageChange}
        aggregateStatus={aggregateStatus}
        onSettingsClick={handleSettingsClick}
      />

      {/* Main Single-Viewport / Scrollable Service Grid Container */}
      <main className="flex-1 overflow-y-auto px-4 py-6 sm:px-8 sm:py-8 pb-20 lg:pb-20">
        <div className="mx-auto max-w-[1440px]">
          {/* Services Grid (Single-col mobile, 2-col tablet, 4-col desktop) */}
          <section aria-label="Services Overview">
            <div
              className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 sm:gap-6"
              data-testid="services-grid"
            >
              {visibleServices.map((svc) => (
                <ServiceCard
                  key={svc.id}
                  id={svc.id}
                  name={svc.name}
                  techSubtitle={svc.techSubtitle}
                  icon={svc.icon}
                  status={svc.status}
                  metric={svc.metric}
                  secondaryDescriptor={svc.secondaryDescriptor}
                  actionUrl={svc.actionUrl}
                  actionLabel={svc.actionLabel}
                  currentLanguage={language}
                  onRequireAdmin={handleRequireAdmin}
                  isAuthenticated={isAuthenticated}
                  onRestart={handleRestart}
                  isRestarting={svc.isRestarting}
                  reassuranceMessage={svc.reassuranceMessage}
                />
              ))}
            </div>
          </section>
        </div>
      </main>

      {/* Pinned 56px Telemetry Bar */}
      <TelemetryBar telemetry={telemetry} currentLanguage={language} />

      {/* Accessible Administrator Login Modal */}
      <AdminLoginModal
        isOpen={isLoginModalOpen}
        onClose={() => {
          setIsLoginModalOpen(false);
          setPendingAction(null);
        }}
        onSuccess={handleLoginSuccess}
        apiClient={client}
        currentLanguage={language}
      />

      {/* RFC 7807 Problem Details Modal on Restart Failure */}
      {restartError && (
        <div
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="restart-error-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          data-testid="problem-details-modal"
        >
          <div className="w-full max-w-md rounded-xl border border-red-200 bg-white p-6 shadow-xl text-[#0F172A]">
            <div className="flex items-center gap-3">
              <span className="text-2xl text-red-600" aria-hidden="true">
                ⚠️
              </span>
              <h3
                id="restart-error-title"
                className="text-base font-semibold text-[#0F172A]"
              >
                {restartError.title}
              </h3>
            </div>
            <p className="mt-3 text-xs sm:text-sm text-[#475569] leading-relaxed">
              {restartError.detail}
            </p>
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                onClick={() => setRestartError(null)}
                className="h-11 rounded-lg bg-[#0F172A] px-4 text-xs font-medium text-white hover:bg-[#1E293B] focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
                data-testid="close-problem-details"
              >
                {t.close}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default function HomePage() {
  return <HomeScreen />;
}

