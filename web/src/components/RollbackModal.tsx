import React, { useEffect, useRef } from "react";
import { translations, type Language } from "@/lib/i18n";

export interface RollbackModalProps {
  isOpen: boolean;
  serviceId: string;
  serviceName: string;
  serviceIcon?: string;
  currentVersion?: string;
  previousVersion?: string;
  snapshotTimestamp?: string;
  isRollingBack?: boolean;
  onConfirm: () => Promise<void> | void;
  onClose: () => void;
  currentLanguage: Language;
}

export function RollbackModal({
  isOpen,
  serviceId,
  serviceName,
  serviceIcon = "⏪",
  currentVersion,
  previousVersion,
  snapshotTimestamp,
  isRollingBack = false,
  onConfirm,
  onClose,
  currentLanguage,
}: RollbackModalProps) {
  const t = translations[currentLanguage];
  const dialogRef = useRef<HTMLDivElement>(null);
  const cancelButtonRef = useRef<HTMLButtonElement>(null);
  const previousActiveElement = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!isOpen) return;

    if (typeof document !== "undefined") {
      previousActiveElement.current = document.activeElement as HTMLElement | null;
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !isRollingBack) {
        onClose();
        return;
      }

      if (event.key === "Tab") {
        const container = dialogRef.current;
        if (!container) return;
        const focusables = container.querySelectorAll<HTMLElement>(
          'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        if (!focusables || focusables.length === 0) return;

        const first = focusables[0];
        const last = focusables[focusables.length - 1];

        if (event.shiftKey) {
          if (document.activeElement === first) {
            event.preventDefault();
            last.focus();
          }
        } else {
          if (document.activeElement === last) {
            event.preventDefault();
            first.focus();
          }
        }
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    // Auto-focus cancel button on open for keyboard accessibility
    const timer = setTimeout(() => {
      cancelButtonRef.current?.focus();
    }, 50);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      clearTimeout(timer);
      previousActiveElement.current?.focus();
    };
  }, [isOpen, isRollingBack, onClose]);

  if (!isOpen) {
    return null;
  }

  // Helper to format ISO timestamp into localized consumer date/time
  function formatTimestamp(ts?: string): string {
    if (!ts) return "—";
    try {
      const d = new Date(ts);
      if (isNaN(d.getTime())) return ts;
      return new Intl.DateTimeFormat(
        currentLanguage === "de"
          ? "de-DE"
          : currentLanguage === "es"
          ? "es-ES"
          : currentLanguage === "fr"
          ? "fr-FR"
          : "en-US",
        {
          dateStyle: "medium",
          timeStyle: "short",
        }
      ).format(d);
    } catch {
      return ts;
    }
  }

  // Format version to strip unnecessary image registry prefixes for consumer clarity
  function formatImageTag(tag?: string, isPrev?: boolean): string {
    if (!tag) return isPrev ? t.noPriorRevisions : "—";
    const parts = tag.split("/");
    const last = parts[parts.length - 1];
    return last;
  }

  const formattedCurrent = formatImageTag(currentVersion, false);
  const formattedPrevious = formatImageTag(previousVersion, true);
  const formattedSnapshot = formatTimestamp(snapshotTimestamp);
  const isConfirmDisabled = isRollingBack || !previousVersion || previousVersion === currentVersion;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby={`rollback-title-${serviceId}`}
      aria-describedby={`rollback-desc-${serviceId}`}
      data-testid={`rollback-modal-${serviceId}`}
      onClick={(e) => {
        if (e.target === e.currentTarget && !isRollingBack) {
          onClose();
        }
      }}
    >
      <div
        ref={dialogRef}
        className="w-full max-w-lg rounded-2xl border border-[#E2E8F0] bg-white p-6 sm:p-7 shadow-2xl text-[#0F172A] animate-in fade-in zoom-in-95 duration-150"
      >
        {/* Header */}
        <div className="flex items-start gap-4">
          <div
            className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-amber-50 text-2xl border border-amber-200 text-amber-700"
            aria-hidden="true"
          >
            {serviceIcon}
          </div>
          <div className="flex-1 min-w-0">
            <h3
              id={`rollback-title-${serviceId}`}
              className="text-lg font-semibold text-[#0F172A] tracking-tight leading-snug"
            >
              {t.rollbackVersion}: {serviceName}
            </h3>
            <p className="mt-1 text-xs sm:text-sm text-[#64748B]">
              {serviceName}
            </p>
          </div>
        </div>

        {/* Reassurance Message Notice */}
        <div
          id={`rollback-desc-${serviceId}`}
          className="mt-5 rounded-xl border border-emerald-200 bg-emerald-50/80 p-3.5 text-xs sm:text-sm text-emerald-900 leading-relaxed flex items-start gap-2.5"
          data-testid="rollback-reassurance-note"
        >
          <span className="shrink-0 text-emerald-700 font-bold" aria-hidden="true">
            🛡️
          </span>
          <p className="font-normal">{t.rollbackPreservedNote}</p>
        </div>

        {/* Version & Snapshot Comparison Grid */}
        <div className="mt-5 grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-3.5">
            <span className="block text-[11px] font-medium text-[#64748B] uppercase tracking-wider">
              {t.currentVersionLabel}
            </span>
            <span
              className="mt-1 block text-sm font-semibold text-[#0F172A] truncate"
              data-testid="current-version-tag"
              title={currentVersion}
            >
              {formattedCurrent}
            </span>
          </div>

          <div className="rounded-xl border border-blue-200 bg-blue-50/70 p-3.5">
            <span className="block text-[11px] font-medium text-blue-700 uppercase tracking-wider">
              {t.previousVersionLabel}
            </span>
            <span
              className="mt-1 block text-sm font-semibold text-blue-900 truncate"
              data-testid="previous-version-tag"
              title={previousVersion}
            >
              {formattedPrevious}
            </span>
          </div>

          <div className="sm:col-span-2 rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-3.5 flex items-center justify-between">
            <div>
              <span className="block text-[11px] font-medium text-[#64748B] uppercase tracking-wider">
                {t.snapshotTimestampLabel}
              </span>
              <span
                className="mt-0.5 block text-xs sm:text-sm font-medium text-[#334155]"
                data-testid="snapshot-timestamp"
              >
                {formattedSnapshot}
              </span>
            </div>
            <span className="text-xs font-semibold px-2.5 py-1 rounded-full bg-slate-200 text-slate-700">
              {t.walAndConfig}
            </span>
          </div>
        </div>

        {/* Actions Footer */}
        <div className="mt-6 sm:mt-7 flex flex-col-reverse sm:flex-row justify-end gap-3">
          <button
            ref={cancelButtonRef}
            type="button"
            disabled={isRollingBack}
            onClick={onClose}
            className="flex h-11 min-h-[44px] items-center justify-center rounded-xl border border-[#CBD5E1] bg-white px-5 text-sm font-medium text-[#475569] hover:bg-[#F1F5F9] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer disabled:opacity-50"
            data-testid={`cancel-rollback-${serviceId}`}
          >
            {t.cancel}
          </button>
          <button
            type="button"
            disabled={isConfirmDisabled}
            onClick={async () => {
              if (isConfirmDisabled) return;
              await onConfirm();
            }}
            className="flex h-11 min-h-[44px] items-center justify-center gap-2 rounded-xl bg-[#0F172A] px-5 text-sm font-medium text-white hover:bg-[#1E293B] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
            data-testid={`confirm-rollback-${serviceId}`}
          >
            {isRollingBack ? (
              <>
                <span
                  className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"
                  aria-hidden="true"
                />
                <span>{t.rollingBack}</span>
              </>
            ) : (
              <span>{t.rollbackVersion}</span>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
