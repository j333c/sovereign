"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { StatusBadge, StatusLevel } from "./StatusBadge";
import { Language, translations } from "@/lib/i18n";

export interface AppHeaderProps {
  currentLanguage: Language;
  onLanguageChange: (lang: Language) => void;
  aggregateStatus: StatusLevel;
  onSettingsClick?: () => void;
  statusMessage?: string;
}

export function AppHeader({
  currentLanguage,
  onLanguageChange,
  aggregateStatus,
  onSettingsClick,
  statusMessage,
}: AppHeaderProps) {
  const t = translations[currentLanguage];
  const [showAlertPopover, setShowAlertPopover] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);

  const isGreen = aggregateStatus === "healthy" || aggregateStatus === "operational";
  const isAmber = aggregateStatus === "amber" || aggregateStatus === "degraded";
  const healthLabel = isGreen
    ? t.allSystemsOperational
    : isAmber
    ? t.attentionRequired
    : t.systemError;

  useEffect(() => {
    if (!showAlertPopover) return;
    function handleClickOutside(e: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setShowAlertPopover(false);
      }
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setShowAlertPopover(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [showAlertPopover]);

  return (
    <header
      className="sticky top-0 z-30 flex h-16 w-full items-center justify-between border-b border-[#E5E7EB] bg-white px-4 sm:px-8 shadow-xs"
      data-testid="app-header"
    >
      {/* Brand & Aggregate Health */}
      <div className="flex items-center gap-4 sm:gap-6">
        <Link
          href="/"
          className="flex items-center gap-3 rounded-md focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
          aria-label="Sovereign Home"
        >
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[#0F172A] text-white font-bold text-sm tracking-wide shadow-xs">
            S
          </div>
          <div className="flex flex-col">
            <span className="text-base sm:text-lg font-semibold tracking-tight text-[#0F172A] leading-tight">
              {t.crestTitle}
            </span>
            <span className="text-[11px] text-[#64748B] hidden sm:inline leading-none">
              {t.crestSubtitle}
            </span>
          </div>
        </Link>

        {/* Aggregate Health Pill with Popover */}
        <div className="relative" ref={popoverRef}>
          <button
            type="button"
            onClick={() => setShowAlertPopover(!showAlertPopover)}
            className="flex items-center justify-center min-h-[44px] min-w-[44px] rounded-full focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
            aria-label={`System status: ${healthLabel}. Click to view details`}
            aria-expanded={showAlertPopover}
          >
            <StatusBadge status={aggregateStatus} label={healthLabel} />
          </button>

          {showAlertPopover && (
            <div
              className="absolute left-0 mt-2 w-72 rounded-lg border border-[#E5E7EB] bg-white p-3 shadow-lg z-50 text-xs text-[#0F172A]"
              role="dialog"
              aria-label={t.systemHealthDetails}
            >
              <div className="flex items-center justify-between pb-2 border-b border-[#E5E7EB] font-semibold">
                <span>{healthLabel}</span>
                <button
                  type="button"
                  onClick={() => setShowAlertPopover(false)}
                  className="text-[#64748B] hover:text-[#0F172A] p-1 rounded-sm focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
                  aria-label="Close status popover"
                >
                  ✕
                </button>
              </div>
              <p className="mt-2 text-[#64748B] leading-relaxed">
                {statusMessage ||
                  (isGreen
                    ? t.healthPopoverHealthy
                    : t.healthPopoverAttention)}
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Utility Actions */}
      <div className="flex items-center gap-2 sm:gap-3">
        {/* Drop Media link */}
        <Link
          href="/drop"
          className="flex h-11 items-center justify-center rounded-lg px-3 sm:px-4 text-xs sm:text-sm font-medium text-[#0F172A] bg-[#F8F9FA] hover:bg-[#F1F5F9] border border-[#E5E7EB] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
          aria-label={t.dropMedia}
        >
          <span className="mr-1.5" aria-hidden="true">
            📥
          </span>
          <span>{t.dropMedia}</span>
        </Link>

        {/* Guides link */}
        <Link
          href="/guides"
          className="flex h-11 items-center justify-center rounded-lg px-3 sm:px-4 text-xs sm:text-sm font-medium text-[#0F172A] bg-[#F8F9FA] hover:bg-[#F1F5F9] border border-[#E5E7EB] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
          aria-label={t.guidesTitle}
          title={t.guidesTitle}
          data-testid="guides-nav-link"
        >
          <span className="mr-1.5" aria-hidden="true">
            📖
          </span>
          <span className="hidden md:inline">{t.guidesTitle}</span>
        </Link>

        {/* Settings button */}
        {onSettingsClick ? (
          <button
            type="button"
            onClick={onSettingsClick}
            className="flex h-11 w-11 items-center justify-center rounded-lg text-[#0F172A] bg-[#F8F9FA] hover:bg-[#F1F5F9] border border-[#E5E7EB] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
            aria-label={t.settings}
            title={t.settings}
          >
            <span aria-hidden="true" className="text-base">
              ⚙️
            </span>
          </button>
        ) : (
          <Link
            href="/settings"
            className="flex h-11 w-11 items-center justify-center rounded-lg text-[#0F172A] bg-[#F8F9FA] hover:bg-[#F1F5F9] border border-[#E5E7EB] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
            aria-label={t.settings}
            title={t.settings}
          >
            <span aria-hidden="true" className="text-base">
              ⚙️
            </span>
          </Link>
        )}

        {/* Language Switcher */}
        <div className="relative">
          <label htmlFor="language-select" className="sr-only">
            {t.languageLabel}
          </label>
          <select
            id="language-select"
            value={currentLanguage}
            onChange={(e) => onLanguageChange(e.target.value as Language)}
            className="h-11 appearance-none rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] px-3 pr-7 text-xs font-medium text-[#0F172A] hover:bg-[#F1F5F9] focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
            aria-label={t.languageLabel}
          >
            <option value="en">English (EN)</option>
            <option value="de">Deutsch (DE)</option>
            <option value="es">Español (ES)</option>
            <option value="fr">Français (FR)</option>
          </select>
          <span
            aria-hidden="true"
            className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-[#64748B]"
          >
            ▼
          </span>
        </div>
      </div>
    </header>
  );
}
