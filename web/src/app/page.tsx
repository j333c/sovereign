"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { apiClient as defaultApiClient, SovereignApiClient, ProblemDetailsError } from "@/lib/api";

export interface HomePageProps {
  apiClientOverride?: SovereignApiClient;
  routerOverride?: { replace: (url: string) => void };
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

function HomeScreenContent({
  apiClientOverride,
  router,
}: HomePageProps & { router?: { replace: (url: string) => void } }) {
  const client = apiClientOverride ?? defaultApiClient;
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const checkBootMode = async () => {
      try {
        const status = await client.getBootStatus();
        if (!isMounted) return;

        if (status.mode !== "operational") {
          if (router) {
            router.replace("/boot");
          }
          return;
        }

        const setup = await client.getSetupStatus();
        if (!isMounted) return;

        if (!setup.is_configured) {
          if (router) {
            router.replace("/setup");
          }
          return;
        }
      } catch (err: unknown) {
        if (!isMounted) return;

        if (err instanceof ProblemDetailsError && err.status === 503) {
          if (router) {
            router.replace("/boot");
          }
          return;
        }
      }

      if (isMounted) {
        setIsReady(true);
      }
    };

    void checkBootMode();

    return () => {
      isMounted = false;
    };
  }, [client, router]);

  if (!isReady) {
    return null;
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-8 bg-[var(--canvas)]">
      <div className="w-full max-w-xl rounded-lg border border-[var(--border)] bg-[var(--surface)] p-8 shadow-sm">
        <div className="flex items-center justify-between pb-6 border-b border-[var(--border)]">
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-[var(--text-primary)]">
              Sovereign Appliance
            </h1>
            <p className="text-sm text-[var(--text-muted)] mt-1">
              Core supervisor control plane
            </p>
          </div>
          <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-800 border border-emerald-200">
            <span className="h-2 w-2 rounded-full bg-emerald-500" />
            Operational
          </span>
        </div>

        <div className="mt-6 space-y-4">
          <div className="rounded-md bg-[var(--border-subtle)] p-4 text-xs font-mono text-[var(--text-faint)]">
            GET /api/v1/health &bull; GET /api/v1/system/boot-status
          </div>
          <p className="text-sm text-[var(--text-muted)] leading-relaxed">
            Appliance supervisor daemon and decoupled presentation frontend are operational with SQLite WAL durability and OpenAPI contract boundaries.
          </p>
        </div>
      </div>
    </main>
  );
}

export default function HomePage() {
  return <HomeScreen />;
}
