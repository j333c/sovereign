"use client";

import React, { useState, useEffect, useRef } from "react";
import { SovereignApiClient, ProblemDetailsError } from "@/lib/api";
import { Language, translations } from "@/lib/i18n";

export interface AdminLoginModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  apiClient: SovereignApiClient;
  currentLanguage: Language;
}

export function AdminLoginModal({
  isOpen,
  onClose,
  onSuccess,
  apiClient,
  currentLanguage,
}: AdminLoginModalProps) {
  const t = translations[currentLanguage];
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setPassword("");
      setError(null);
      setIsSubmitting(false);
      setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
    }
  }, [isOpen]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape" && isOpen) {
        onClose();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!password) {
      setError(t.invalidPassword);
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await apiClient.login({
        username: "admin",
        password,
      });
      setIsSubmitting(false);
      onSuccess();
    } catch (err) {
      setIsSubmitting(false);
      if (err instanceof ProblemDetailsError && err.status === 401) {
        setError(t.invalidPassword);
      } else {
        setError(t.invalidPassword);
      }
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-xs"
      role="dialog"
      aria-modal="true"
      aria-labelledby="admin-login-title"
    >
      <div className="w-full max-w-md rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xl animate-in fade-in zoom-in-95 duration-150">
        <div className="flex items-center justify-between pb-4 border-b border-[#E5E7EB]">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[#0F172A] text-white text-sm">
              🔒
            </div>
            <h2
              id="admin-login-title"
              className="text-base sm:text-lg font-semibold text-[#0F172A]"
            >
              {t.adminAuthTitle}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="flex h-9 w-9 items-center justify-center rounded-lg text-[#64748B] hover:text-[#0F172A] hover:bg-slate-100 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
            aria-label={t.cancel}
          >
            ✕
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-4 space-y-4">
          <p className="text-xs sm:text-sm text-[#64748B] leading-relaxed">
            {t.adminAuthDesc}
          </p>

          <div>
            <label
              htmlFor="admin-password-input"
              className="block text-xs font-medium text-[#0F172A] mb-1.5"
            >
              {t.adminPasswordLabel}
            </label>
            <input
              id="admin-password-input"
              ref={inputRef}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t.adminPasswordPlaceholder}
              autoComplete="current-password"
              className="w-full h-11 rounded-lg border border-[#E5E7EB] px-3.5 text-sm text-[#0F172A] placeholder:text-[#94A3B8] focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
            />
          </div>

          {error && (
            <div
              className="rounded-lg bg-red-50 p-2.5 text-xs text-red-700 border border-red-200"
              role="alert"
            >
              {error}
            </div>
          )}

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="flex h-11 min-w-[80px] items-center justify-center rounded-lg border border-[#E5E7EB] bg-white px-4 text-xs sm:text-sm font-medium text-[#0F172A] hover:bg-slate-50 transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none disabled:opacity-50 cursor-pointer"
            >
              {t.cancel}
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex h-11 min-w-[90px] items-center justify-center rounded-lg bg-[#0F172A] px-5 text-xs sm:text-sm font-medium text-white hover:bg-[#1E293B] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none disabled:opacity-50 cursor-pointer"
            >
              {isSubmitting ? "..." : t.unlock}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
