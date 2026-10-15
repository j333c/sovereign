"use client";

import React, { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  apiClient as defaultApiClient,
  SovereignApiClient,
  TelemetryUpdate,
} from "@/lib/api";
import { Language, translations } from "@/lib/i18n";
import { useTelemetry } from "@/lib/useTelemetry";
import { AppHeader } from "@/components/AppHeader";
import { TelemetryBar } from "@/components/TelemetryBar";
import { ServiceCard } from "@/components/ServiceCard";
import { AdminLoginModal } from "@/components/AdminLoginModal";
import { StatusLevel } from "@/components/StatusBadge";

export interface HomePageProps {
  apiClientOverride?: SovereignApiClient;
  routerOverride?: { replace: (url: string) => void };
  initialTelemetry?: TelemetryUpdate | null;
  eventSourceFactory?: (url: string) => EventSource;
  initialLanguage?: Language;
  initialAuthenticated?: boolean;
  initialReady?: boolean;
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

function HomeScreenContent({
  apiClientOverride,
  router,
  initialTelemetry = null,
  eventSourceFactory,
  initialLanguage = "en",
  initialAuthenticated = false,
  initialReady = false,
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

  const handleLanguageChange = useCallback((lang: Language) => {
    setLanguage(lang);
    if (typeof window !== "undefined") {
      localStorage.setItem("sovereign_lang", lang);
    }
  }, []);

  // Subscribe to real-time SSE telemetry
  const { telemetry } = useTelemetry({
    client,
    initialTelemetry,
    eventSourceFactory,
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
          const list = settings.features.split(",").map((s) => s.trim().toLowerCase());
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
    } else if (pendingAction) {
      // Pending action executed on successful authentication
      setPendingAction(null);
    }
  }, [pendingAction, router]);

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

  // Determine individual service states based on subsystem telemetry
  const nasIsDegraded = telemetry?.nas_status === "degraded" || telemetry?.nas_status === "disconnected";

  const services = [
    {
      id: "video",
      name: t.videoTitle,
      techSubtitle: t.videoSubtitle,
      icon: "🎬",
      status: (nasIsDegraded ? "degraded" : "healthy") as StatusLevel,
      metric: t.videoMetric,
      secondaryDescriptor: t.videoDesc,
      actionUrl: "/media",
      actionLabel: t.videoAction,
    },
    {
      id: "audio",
      name: t.audioTitle,
      techSubtitle: t.audioSubtitle,
      icon: "🎵",
      status: (nasIsDegraded ? "degraded" : "healthy") as StatusLevel,
      metric: t.audioMetric,
      secondaryDescriptor: t.audioDesc,
      actionUrl: "/music",
      actionLabel: t.audioAction,
    },
    {
      id: "dms",
      name: t.dmsTitle,
      techSubtitle: t.dmsSubtitle,
      icon: "📑",
      status: "healthy" as StatusLevel,
      metric: t.dmsMetric,
      secondaryDescriptor: t.dmsDesc,
      actionUrl: "/docs",
      actionLabel: t.dmsAction,
    },
    {
      id: "ai",
      name: t.aiTitle,
      techSubtitle: t.aiSubtitle,
      icon: "⚡",
      status: "healthy" as StatusLevel,
      metric: t.aiMetric,
      secondaryDescriptor: t.aiDesc,
      actionUrl: "/chat",
      actionLabel: t.aiAction,
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
    </div>
  );
}

export default function HomePage() {
  return <HomeScreen />;
}
