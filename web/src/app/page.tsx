"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { apiClient } from "@/lib/api";

export default function HomePage() {
  const router = useRouter();
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const checkBootMode = async () => {
      try {
        const status = await apiClient.getBootStatus();
        if (!isMounted) return;

        if (status.mode === "diagnostics") {
          router.replace("/boot");
          return;
        }
      } catch {
        // If boot-status probe fails, proceed to render dashboard fallback
      }
      if (isMounted) {
        setIsReady(true);
      }
    };

    void checkBootMode();

    return () => {
      isMounted = false;
    };
  }, [router]);

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
