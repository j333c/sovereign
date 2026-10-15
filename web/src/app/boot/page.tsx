"use client";

import React, { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { apiClient as defaultApiClient, SovereignApiClient, type BootStatusResponse, ProblemDetailsError } from "@/lib/api";

export interface BootScreenProps {
  initialStatus?: BootStatusResponse | null;
  apiClientOverride?: SovereignApiClient;
  routerOverride?: { push: (url: string) => void };
  onRedirect?: () => void;
  redirectDelayMs?: number;
  pollingIntervalMs?: number;
}

export function BootScreen(props: BootScreenProps) {
  if (props.routerOverride || props.onRedirect) {
    return <BootScreenContent {...props} router={props.routerOverride} />;
  }
  return <BootScreenWithRouter {...props} />;
}

function BootScreenWithRouter(props: BootScreenProps) {
  const router = useRouter();
  return <BootScreenContent {...props} router={router} />;
}

function BootScreenContent({
  initialStatus = null,
  apiClientOverride,
  router,
  onRedirect,
  redirectDelayMs = 1200,
  pollingIntervalMs = 1000,
}: BootScreenProps & { router?: { push: (url: string) => void } }) {
  const client = apiClientOverride ?? defaultApiClient;

  const [status, setStatus] = useState<BootStatusResponse | null>(initialStatus);
  const [isRedirecting, setIsRedirecting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isStorageDegraded, setIsStorageDegraded] = useState(false);
  const hasTriggeredRedirect = useRef(false);

  useEffect(() => {
    let isMounted = true;
    let pollTimer: ReturnType<typeof setTimeout> | null = null;

    const checkBootStatus = async () => {
      try {
        const res = await client.getBootStatus();
        if (!isMounted) return;

        setStatus(res);
        setIsStorageDegraded(false);
        setErrorMessage(null);

        if (res.mode === "operational" && res.diagnostics_completed) {
          if (!hasTriggeredRedirect.current) {
            hasTriggeredRedirect.current = true;
            setIsRedirecting(true);

            setTimeout(() => {
              if (onRedirect) {
                onRedirect();
              } else if (router) {
                router.push("/");
              }
            }, redirectDelayMs);
          }
          return;
        }

        if (res.mode === "failed" || res.mode === "recovery") {
          setErrorMessage("System verification detected an issue. Your files are protected in safety mode. Automated restore recommended.");
          return;
        }

        // Still in diagnostics, poll again
        pollTimer = setTimeout(checkBootStatus, pollingIntervalMs);
      } catch (err: unknown) {
        if (!isMounted) return;

        if (err instanceof ProblemDetailsError && err.status === 503) {
          setIsStorageDegraded(true);
          setErrorMessage("Hardware attention needed: A hard drive is reporting errors. Your existing files are locked in read-only safety mode.");
        } else {
          // Re-try after interval without showing intimidating stack traces
          pollTimer = setTimeout(checkBootStatus, pollingIntervalMs);
        }
      }
    };

    void checkBootStatus();

    return () => {
      isMounted = false;
      if (pollTimer) clearTimeout(pollTimer);
    };
  }, [client, onRedirect, redirectDelayMs, pollingIntervalMs, router]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-6 bg-[var(--canvas)]">
      <div className="w-full max-w-lg rounded-xl border border-[var(--border)] bg-[var(--surface)] p-8 shadow-sm text-center">
        {/* Brand Crest */}
        <div className="mx-auto mb-6 flex h-10 w-10 items-center justify-center rounded-lg bg-[var(--action-primary)] text-white font-semibold text-lg tracking-wider select-none">
          S
        </div>

        {/* Circular Progress or Status Icon */}
        <div className="mx-auto my-6 flex items-center justify-center" aria-live="polite">
          {errorMessage || isStorageDegraded ? (
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-[var(--color-status-amber-bg)] text-[var(--color-status-amber)] border border-[var(--color-status-amber-border)]">
              <span className="text-2xl" aria-hidden="true">&#9670;</span>
            </div>
          ) : isRedirecting ? (
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-[var(--color-status-green-bg)] text-[var(--color-status-green)] border border-[var(--color-status-green-border)] animate-pulse">
              <span className="text-2xl" aria-hidden="true">&#9679;</span>
            </div>
          ) : (
            <svg
              className="animate-spin text-[#2563EB]"
              width="56"
              height="56"
              viewBox="0 0 56 56"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
              role="progressbar"
              aria-label="Checking system safety"
            >
              <circle
                cx="28"
                cy="28"
                r="23"
                stroke="#E5E7EB"
                strokeWidth="4"
              />
              <circle
                cx="28"
                cy="28"
                r="23"
                stroke="#2563EB"
                strokeWidth="4"
                strokeDasharray="145"
                strokeDashoffset="90"
                strokeLinecap="round"
              />
            </svg>
          )}
        </div>

        {/* Reassurance Microcopy */}
        <div className="space-y-3">
          <h1 className="text-lg font-semibold tracking-tight text-[var(--text-primary)] leading-snug">
            {errorMessage ? (
              errorMessage
            ) : isRedirecting ? (
              "System safety verified. Redirecting to dashboard..."
            ) : (
              "Checking system safety after power restoration... your files are secure."
            )}
          </h1>

          <p className="text-sm text-[var(--text-muted)] leading-relaxed">
            {errorMessage
              ? "Your data remains safe and intact on local storage. No action is required while the system stabilizes."
              : isRedirecting
              ? "All storage volumes and database integrity checks completed successfully."
              : "Verifying system integrity and storage safety before launching services."}
          </p>
        </div>

        {/* Appliance status badge */}
        <div className="mt-8 pt-6 border-t border-[var(--border)] flex items-center justify-center gap-2">
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
              errorMessage
                ? "bg-[var(--color-status-amber-bg)] text-[var(--color-status-amber-fg)] border border-[var(--color-status-amber-border)]"
                : isRedirecting
                ? "bg-[var(--color-status-green-bg)] text-[var(--color-status-green-fg)] border border-[var(--color-status-green-border)]"
                : "bg-blue-50 text-blue-700 border border-blue-200"
            }`}
          >
            <span
              className={`h-2 w-2 rounded-full ${
                errorMessage
                  ? "bg-[var(--color-status-amber)]"
                  : isRedirecting
                  ? "bg-[var(--color-status-green)]"
                  : "bg-[#2563EB] animate-pulse"
              }`}
            />
            {errorMessage
              ? "Attention Required"
              : isRedirecting
              ? "System Operational"
              : "Verifying Storage"}
          </span>
        </div>
      </div>
    </main>
  );
}

export default function BootPage() {
  return <BootScreen />;
}
