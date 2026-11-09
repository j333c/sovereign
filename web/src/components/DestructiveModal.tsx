import React, { useState, useEffect, useRef } from "react";
import { translations, type Language } from "@/lib/i18n";
import { useModalFocusTrap } from "@/lib/hooks";

export type DestructiveActionType = "reboot" | "factory-reset" | "deactivate";

export const LOCALIZED_RESET_TOKENS: Record<Language, string[]> = {
  en: ["reset", "delete"],
  de: ["reset", "delete", "löschen", "loeschen", "zurücksetzen", "zuruecksetzen"],
  es: ["reset", "delete", "eliminar", "restablecer"],
  fr: ["reset", "delete", "supprimer", "réinitialiser", "reinitialiser"],
};

export function isValidConfirmationToken(input: string | null | undefined, lang: Language): boolean {
  if (!input || typeof input !== "string") return false;
  const normalized = input.normalize("NFC").trim().toLowerCase();
  if (!normalized) return false;
  const allowed = LOCALIZED_RESET_TOKENS[lang] || LOCALIZED_RESET_TOKENS.en;
  return allowed.some((token) => token.normalize("NFC") === normalized);
}

export interface DestructiveModalProps {
  isOpen: boolean;
  actionType: DestructiveActionType;
  serviceName?: string;
  isProcessing?: boolean;
  errorMessage?: string | null;
  onConfirm: () => Promise<void> | void;
  onClose: () => void;
  currentLanguage: Language;
  initialResetInput?: string;
}

export function DestructiveModal({
  isOpen,
  actionType,
  serviceName,
  isProcessing = false,
  errorMessage = null,
  onConfirm,
  onClose,
  currentLanguage,
  initialResetInput = "",
}: DestructiveModalProps) {
  const t = translations[currentLanguage];
  const [resetInput, setResetInput] = useState(initialResetInput);
  const dialogRef = useRef<HTMLDivElement>(null);
  const cancelButtonRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const previousActiveElement = useRef<HTMLElement | null>(null);

  useModalFocusTrap(dialogRef, isOpen, onClose, { disableEscape: isProcessing });

  // Reset state when opening/closing
  useEffect(() => {
    if (isOpen) {
      setResetInput(initialResetInput);
    }
  }, [isOpen, actionType, initialResetInput]);

  // Focus management: focus on open and restore on unmount/close
  useEffect(() => {
    if (!isOpen) return;

    if (typeof document !== "undefined") {
      previousActiveElement.current = document.activeElement as HTMLElement | null;
    }

    const timer = setTimeout(() => {
      if (actionType === "factory-reset") {
        inputRef.current?.focus();
      } else {
        cancelButtonRef.current?.focus();
      }
    }, 50);

    return () => {
      clearTimeout(timer);
      previousActiveElement.current?.focus();
    };
  }, [isOpen, actionType]);

  if (!isOpen) {
    return null;
  }

  const isResetValid = actionType !== "factory-reset" || isValidConfirmationToken(resetInput, currentLanguage);

  // Derive title, description, and button text based on actionType
  let title = t.rebootTitle;
  let description = t.rebootDesc;
  let confirmText = isProcessing ? t.rebootingState : t.rebootConfirmButton;
  let icon = "🔄";
  let buttonColor = "bg-amber-600 hover:bg-amber-700 text-white focus-visible:ring-amber-500";

  if (actionType === "factory-reset") {
    title = t.factoryResetTitle;
    description = t.factoryResetDesc;
    confirmText = isProcessing ? t.factoryResettingState : t.factoryResetConfirmButton;
    icon = "⚠️";
    buttonColor = "bg-rose-600 hover:bg-rose-700 text-white focus-visible:ring-rose-500 disabled:opacity-40 disabled:cursor-not-allowed";
  } else if (actionType === "deactivate") {
    title = t.deactivateTitle;
    description = serviceName
      ? `${serviceName}: ${t.deactivateDesc}`
      : t.deactivateDesc;
    confirmText = isProcessing ? t.deactivatingState : t.deactivateConfirmButton;
    icon = "⏸️";
    buttonColor = "bg-rose-600 hover:bg-rose-700 text-white focus-visible:ring-rose-500";
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 transition-opacity"
      role="dialog"
      aria-modal="true"
      aria-labelledby="destructive-modal-title"
      aria-describedby="destructive-modal-desc"
      ref={dialogRef}
      onClick={(e) => {
        if (e.target === e.currentTarget && !isProcessing) {
          onClose();
        }
      }}
    >
      <div className="w-full max-w-lg rounded-2xl border border-[#E2E8F0] bg-white p-6 shadow-2xl transition-all">
        {/* Modal Header */}
        <div className="flex items-start gap-4 pb-4 border-b border-[#F1F5F9]">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-amber-50 text-2xl border border-amber-200" aria-hidden="true">
            {icon}
          </div>
          <div className="flex-1">
            <h2 id="destructive-modal-title" className="text-lg font-bold text-[#0F172A]">
              {title}
            </h2>
            <p id="destructive-modal-desc" className="mt-1 text-xs text-[#64748B] leading-relaxed">
              {description}
            </p>
          </div>
        </div>

        {/* Factory Reset Validation Input */}
        {actionType === "factory-reset" && (
          <div className="mt-5 space-y-2">
            <label htmlFor="factory-reset-confirmation-input" className="block text-xs font-semibold text-[#334155]">
              {t.typeResetToConfirm}
            </label>
            <input
              id="factory-reset-confirmation-input"
              ref={inputRef}
              type="text"
              value={resetInput}
              onChange={(e) => setResetInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && isResetValid && !isProcessing) {
                  e.preventDefault();
                  void onConfirm();
                }
              }}
              placeholder={t.resetInputPlaceholder}
              disabled={isProcessing}
              aria-invalid={actionType === "factory-reset" && resetInput.length > 0 && !isResetValid}
              className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] px-3.5 py-2 text-sm font-mono tracking-wider text-[#0F172A] placeholder-[#94A3B8] focus:border-rose-500 focus:outline-none focus:ring-2 focus:ring-rose-200"
              autoComplete="off"
              data-testid="factory-reset-input"
            />
          </div>
        )}

        {/* Reassurance note for Service Deactivation */}
        {actionType === "deactivate" && (
          <div className="mt-4 rounded-xl border border-blue-100 bg-blue-50/60 p-3.5 flex items-center gap-3 text-xs text-blue-900">
            <span className="text-lg" aria-hidden="true">🛡️</span>
            <span>{t.rollbackPreservedNote}</span>
          </div>
        )}

        {/* Error Notice */}
        {errorMessage && (
          <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-800" role="alert">
            {errorMessage}
          </div>
        )}

        {/* Modal Actions */}
        <div className="mt-6 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button
            type="button"
            ref={cancelButtonRef}
            onClick={onClose}
            disabled={isProcessing}
            className="flex min-h-[44px] items-center justify-center rounded-xl border border-[#CBD5E1] bg-white px-5 py-2.5 text-sm font-semibold text-[#475569] shadow-xs hover:bg-[#F8FAFC] hover:text-[#0F172A] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer disabled:opacity-50"
            data-testid="destructive-cancel-button"
          >
            {t.cancel}
          </button>

          <button
            type="button"
            onClick={() => {
              if (isResetValid && !isProcessing) {
                void onConfirm();
              }
            }}
            disabled={!isResetValid || isProcessing}
            className={`flex min-h-[44px] items-center justify-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-2 cursor-pointer ${buttonColor}`}
            data-testid="destructive-confirm-button"
          >
            {isProcessing && (
              <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" aria-hidden="true" />
            )}
            <span>{confirmText}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
