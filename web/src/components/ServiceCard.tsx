"use client";

import React, { useState, useEffect, useRef } from "react";
import { StatusBadge, StatusLevel } from "./StatusBadge";
import { Language, translations } from "@/lib/i18n";

export interface ServiceCardProps {
  id: string;
  name: string;
  techSubtitle: string;
  icon: string;
  status: StatusLevel;
  metric: string;
  secondaryDescriptor: string;
  actionUrl: string;
  actionLabel: string;
  currentLanguage: Language;
  onRequireAdmin: (actionName: string) => void;
  isAuthenticated: boolean;
}

export function ServiceCard({
  id,
  name,
  techSubtitle,
  icon,
  status,
  metric,
  secondaryDescriptor,
  actionUrl,
  actionLabel,
  currentLanguage,
  onRequireAdmin,
  isAuthenticated,
}: ServiceCardProps) {
  const t = translations[currentLanguage];
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        menuRef.current &&
        !menuRef.current.contains(event.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(event.target as Node)
      ) {
        setIsMenuOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && isMenuOpen) {
        setIsMenuOpen(false);
        buttonRef.current?.focus();
      }
    }

    if (isMenuOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      window.addEventListener("keydown", handleKeyDown);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [isMenuOpen]);

  function handlePrivilegedAction(actionName: string) {
    setIsMenuOpen(false);
    if (!isAuthenticated) {
      onRequireAdmin(actionName);
    } else {
      // Admin action already authorized
      alert(`Action: ${actionName} on service ${name}`);
    }
  }

  function handleDownloadLogs() {
    setIsMenuOpen(false);
    // Non-destructive action - simulated download
    const dummyLog = `=== Diagnostic Logs for ${name} (${id}) ===\nTimestamp: ${new Date().toISOString()}\nStatus: ${status}\nAll healthchecks passing.\nZero personal user content included.`;
    const blob = new Blob([dummyLog], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${id}-diagnostics.txt`;
    a.click();
    URL.revokeObjectURL(url);
  }

  const isGreen = status === "healthy" || status === "operational";
  const isAmber = status === "amber" || status === "degraded";
  const statusLabel = isGreen
    ? t.runningHealthy
    : isAmber
    ? t.attentionRequired
    : t.stoppedError;

  return (
    <article
      className="flex flex-col justify-between rounded-lg border border-[#E5E7EB] bg-white p-5 shadow-xs transition-shadow hover:shadow-sm hover:border-[#CBD5E1]"
      data-testid={`service-card-${id}`}
    >
      <div>
        {/* Card Header: Icon + Title + Overflow Menu */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            <div
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#F8F9FA] border border-[#E5E7EB] text-xl"
              aria-hidden="true"
            >
              {icon}
            </div>
            <div>
              <h2 className="text-[15px] font-semibold text-[#0F172A] leading-tight">
                {name}
              </h2>
              <p className="text-xs text-[#64748B] mt-0.5">{techSubtitle}</p>
            </div>
          </div>

          {/* Accessible ••• Overflow Menu */}
          <div className="relative">
            <button
              ref={buttonRef}
              type="button"
              onClick={() => setIsMenuOpen(!isMenuOpen)}
              className="flex h-11 w-11 items-center justify-center rounded-md text-[#64748B] hover:bg-[#F1F5F9] hover:text-[#0F172A] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
              aria-label={`${t.optionsFor} ${name}`}
              aria-expanded={isMenuOpen}
              aria-haspopup="true"
              data-testid={`overflow-menu-btn-${id}`}
            >
              <span className="text-lg leading-none font-bold">•••</span>
            </button>

            {isMenuOpen && (
              <div
                ref={menuRef}
                className="absolute right-0 mt-1 w-56 rounded-lg border border-[#E5E7EB] bg-white py-1 shadow-lg z-40 text-xs text-[#0F172A] animate-in fade-in zoom-in-95 duration-100"
                role="menu"
                aria-label={`${t.optionsFor} ${name}`}
              >
                <button
                  type="button"
                  onClick={() => handlePrivilegedAction("restart")}
                  className="flex w-full items-center px-4 py-2.5 text-left text-[#0F172A] hover:bg-[#F1F5F9] focus-visible:bg-[#F1F5F9] focus-visible:outline-none cursor-pointer"
                  role="menuitem"
                  data-testid={`overflow-restart-${id}`}
                >
                  <span className="mr-2" aria-hidden="true">
                    🔄
                  </span>
                  <span>{t.restartService}</span>
                </button>
                <button
                  type="button"
                  onClick={() => handlePrivilegedAction("rollback")}
                  className="flex w-full items-center px-4 py-2.5 text-left text-[#0F172A] hover:bg-[#F1F5F9] focus-visible:bg-[#F1F5F9] focus-visible:outline-none cursor-pointer"
                  role="menuitem"
                  data-testid={`overflow-rollback-${id}`}
                >
                  <span className="mr-2" aria-hidden="true">
                    ⏪
                  </span>
                  <span>{t.rollbackVersion}</span>
                </button>
                <div className="my-1 border-t border-[#E5E7EB]" />
                <button
                  type="button"
                  onClick={handleDownloadLogs}
                  className="flex w-full items-center px-4 py-2.5 text-left text-[#64748B] hover:text-[#0F172A] hover:bg-[#F1F5F9] focus-visible:bg-[#F1F5F9] focus-visible:outline-none cursor-pointer"
                  role="menuitem"
                  data-testid={`overflow-logs-${id}`}
                >
                  <span className="mr-2" aria-hidden="true">
                    📄
                  </span>
                  <span>{t.downloadLogs}</span>
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Status Badge + Metrics */}
        <div className="mt-4 pt-3 border-t border-[#F1F5F9]">
          <div className="flex items-center justify-between">
            <StatusBadge status={status} label={statusLabel} />
          </div>

          <div className="mt-3">
            <div
              className="text-lg font-semibold text-[#0F172A] tracking-tight"
              data-testid={`service-metric-${id}`}
            >
              {metric}
            </div>
            <p className="text-xs text-[#64748B] mt-0.5">{secondaryDescriptor}</p>
          </div>
        </div>
      </div>

      {/* Primary Action Button ("Open Service UI") */}
      <div className="mt-5">
        <a
          href={actionUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="flex h-11 md:h-10 w-full items-center justify-center rounded-lg bg-[#0F172A] px-4 text-xs sm:text-sm font-medium text-white hover:bg-[#1E293B] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
          data-testid={`service-action-${id}`}
        >
          {actionLabel}
        </a>
      </div>
    </article>
  );
}
