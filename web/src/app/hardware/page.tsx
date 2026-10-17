"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { Language, translations, getInitialLanguage, persistLanguage } from "@/lib/i18n";

export interface HardwarePageProps {
  initialLanguage?: Language;
  initialPreorderModalOpen?: boolean;
  initialSelectedModel?: "mini" | "home" | "studio";
  initialReservationToken?: string | null;
}

export type HardwareTier = "light" | "media" | "ai";

export interface HardwareCalculatorState {
  dms: boolean;
  audio: boolean;
  video: boolean;
  disc: boolean;
  ai: boolean;
}

export function calculateHardwareTier(features?: HardwareCalculatorState | null): HardwareTier {
  if (!features) {
    return "light";
  }
  if (features.ai) {
    return "ai";
  }
  if (features.video || features.disc) {
    return "media";
  }
  return "light";
}

export default function HardwarePage({
  initialLanguage,
  initialPreorderModalOpen = false,
  initialSelectedModel = "mini",
  initialReservationToken = null,
}: HardwarePageProps = {}) {
  const [lang, setLang] = useState<Language>(() => getInitialLanguage(initialLanguage));
  const t = translations[lang];

  const handleLanguageChange = (newLang: Language) => {
    setLang(newLang);
    persistLanguage(newLang);
  };

  const [features, setFeatures] = useState<HardwareCalculatorState>({
    dms: true,
    audio: true,
    video: false,
    disc: false,
    ai: false,
  });

  const recommendedTier = calculateHardwareTier(features);

  // Turnkey preorder reservation modal state
  const [isPreorderModalOpen, setIsPreorderModalOpen] = useState(initialPreorderModalOpen);
  const [preorderEmail, setPreorderEmail] = useState("");
  const [selectedModel, setSelectedModel] = useState<"mini" | "home" | "studio">(initialSelectedModel);
  const [reservationToken, setReservationToken] = useState<string | null>(initialReservationToken);
  const [tokenCopied, setTokenCopied] = useState(false);
  const modalRef = useRef<HTMLDivElement>(null);
  const emailInputRef = useRef<HTMLInputElement>(null);

  const toggleFeature = (key: keyof HardwareCalculatorState) => {
    setFeatures((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleOpenPreorder = (model: "mini" | "home" | "studio") => {
    setSelectedModel(model);
    setIsPreorderModalOpen(true);
    setReservationToken(null);
    setTokenCopied(false);
  };

  const handleConfirmPreorder = (e: React.FormEvent) => {
    e.preventDefault();
    if (!preorderEmail || !preorderEmail.includes("@")) return;
    const token = `SOV-HW-${Math.random().toString(36).substring(2, 8).padEnd(6, "0").toUpperCase()}`;
    setReservationToken(token);
    try {
      localStorage.setItem("sovereign_hardware_token", token);
      localStorage.setItem("sovereign_hardware_model", selectedModel);
    } catch {
      // safe fallback in restricted testing/iframe environments
    }
  };

  // Keyboard navigation & accessibility focus trap for modal
  useEffect(() => {
    if (!isPreorderModalOpen) return;
    emailInputRef.current?.focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsPreorderModalOpen(false);
        return;
      }

      if (e.key === "Tab" && modalRef.current) {
        const focusable = modalRef.current.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];

        if (e.shiftKey) {
          if (document.activeElement === first) {
            e.preventDefault();
            last.focus();
          }
        } else {
          if (document.activeElement === last) {
            e.preventDefault();
            first.focus();
          }
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isPreorderModalOpen]);

  return (
    <div className="min-h-screen bg-[#F8F9FA] text-[#0F172A] flex flex-col font-sans">
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={handleLanguageChange}
        aggregateStatus="healthy"
      />

      <main className="flex-1 w-full max-w-6xl mx-auto px-4 py-8 sm:py-12 flex flex-col gap-10">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Link
            href="/"
            className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-[#0F172A] shadow-xs hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
          >
            <span aria-hidden="true">←</span>
            <span>{t.backToDashboard}</span>
          </Link>
          <Link
            href="/landing"
            className="inline-flex min-h-[44px] items-center gap-1.5 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-blue-600 shadow-xs hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
            data-testid="hardware-showcase-link"
          >
            <span aria-hidden="true">✨</span>
            <span>{t.landingNavLanding}</span>
          </Link>
        </div>

        {/* Header Title & Subtitle */}
        <div className="flex flex-col items-center text-center gap-3">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-[#EFF6FF] text-[#1D4ED8] text-xs font-semibold border border-[#BFDBFE]">
            <span>⚙️</span>
            <span>{t.hardwareTitle}</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-bold tracking-tight text-[#0F172A]">
            {t.hardwareTitle}
          </h1>
          <p className="text-sm sm:text-base text-[#64748B] max-w-2xl leading-relaxed">
            {t.hardwareSubtitle}
          </p>
        </div>

        {/* SECTION 1: Hardware Profiles Overview */}
        <section role="region" aria-labelledby="hardware-tiers-heading" className="space-y-4">
          <h2 id="hardware-tiers-heading" className="text-xl font-bold text-[#0F172A]">
            Three Standard Hardware Profiles
          </h2>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* Light Tier Card */}
            <div
              className={`rounded-2xl border p-6 flex flex-col justify-between transition-all bg-white shadow-xs ${
                recommendedTier === "light"
                  ? "border-blue-600 ring-2 ring-blue-600/20"
                  : "border-[#E5E7EB]"
              }`}
              data-testid="tier-card-light"
            >
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-2xl" aria-hidden="true">🌱</span>
                  {recommendedTier === "light" && (
                    <span className="rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-semibold text-blue-700">
                      {t.recommendedTierLabel}
                    </span>
                  )}
                </div>
                <div>
                  <h3 className="text-lg font-bold text-[#0F172A]">{t.tierLightTitle}</h3>
                  <p className="text-xs text-[#64748B] mt-1">{t.tierLightDesc}</p>
                </div>
                <div className="border-t border-[#F1F5F9] pt-4 space-y-2 text-xs">
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">CPU</span>
                    <span className="font-semibold text-right text-[#0F172A]">{t.tierLightCpu}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">RAM</span>
                    <span className="font-semibold text-[#0F172A]">{t.tierLightRam}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">Storage</span>
                    <span className="font-semibold text-[#0F172A]">{t.tierLightStorage}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">Idle Power</span>
                    <span className="font-semibold text-emerald-600">{t.tierLightPower}</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Media Tier Card */}
            <div
              className={`rounded-2xl border p-6 flex flex-col justify-between transition-all bg-white shadow-xs ${
                recommendedTier === "media"
                  ? "border-blue-600 ring-2 ring-blue-600/20"
                  : "border-[#E5E7EB]"
              }`}
              data-testid="tier-card-media"
            >
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-2xl" aria-hidden="true">🎬</span>
                  {recommendedTier === "media" && (
                    <span className="rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-semibold text-blue-700">
                      {t.recommendedTierLabel}
                    </span>
                  )}
                </div>
                <div>
                  <h3 className="text-lg font-bold text-[#0F172A]">{t.tierMediaTitle}</h3>
                  <p className="text-xs text-[#64748B] mt-1">{t.tierMediaDesc}</p>
                </div>
                <div className="border-t border-[#F1F5F9] pt-4 space-y-2 text-xs">
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">CPU</span>
                    <span className="font-semibold text-right text-[#0F172A]">{t.tierMediaCpu}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">RAM</span>
                    <span className="font-semibold text-[#0F172A]">{t.tierMediaRam}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">Storage</span>
                    <span className="font-semibold text-[#0F172A]">{t.tierMediaStorage}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">Idle Power</span>
                    <span className="font-semibold text-emerald-600">{t.tierMediaPower}</span>
                  </div>
                </div>
              </div>
            </div>

            {/* AI & Power Tier Card */}
            <div
              className={`rounded-2xl border p-6 flex flex-col justify-between transition-all bg-white shadow-xs ${
                recommendedTier === "ai"
                  ? "border-blue-600 ring-2 ring-blue-600/20"
                  : "border-[#E5E7EB]"
              }`}
              data-testid="tier-card-ai"
            >
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-2xl" aria-hidden="true">⚡</span>
                  {recommendedTier === "ai" && (
                    <span className="rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-semibold text-blue-700">
                      {t.recommendedTierLabel}
                    </span>
                  )}
                </div>
                <div>
                  <h3 className="text-lg font-bold text-[#0F172A]">{t.tierAITitle}</h3>
                  <p className="text-xs text-[#64748B] mt-1">{t.tierAIDesc}</p>
                </div>
                <div className="border-t border-[#F1F5F9] pt-4 space-y-2 text-xs">
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">CPU</span>
                    <span className="font-semibold text-right text-[#0F172A]">{t.tierAICpu}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">RAM + VRAM</span>
                    <span className="font-semibold text-[#0F172A]">{t.tierAIRam}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">Storage</span>
                    <span className="font-semibold text-[#0F172A]">{t.tierAIStorage}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-[#64748B]">Idle Power</span>
                    <span className="font-semibold text-emerald-600">{t.tierAIPower}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* SECTION 2: Interactive Hardware Tier Calculator */}
        <section
          role="region"
          aria-labelledby="calculator-heading"
          className="rounded-2xl border border-[#E5E7EB] bg-white p-6 sm:p-8 shadow-xs space-y-6"
        >
          <div>
            <h2 id="calculator-heading" className="text-xl font-bold text-[#0F172A]">
              {t.calcTitle}
            </h2>
            <p className="text-xs sm:text-sm text-[#64748B] mt-1">
              {t.calcSubtitle}
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4" role="group" aria-label="Feature selection">
            {/* Feature 1: DMS */}
            <label
              className={`flex items-center gap-3 p-4 rounded-xl border cursor-pointer min-h-[44px] transition-all focus-within:ring-2 focus-within:ring-blue-600 focus-within:outline-hidden ${
                features.dms
                  ? "border-blue-600 bg-blue-50/40 text-[#0F172A]"
                  : "border-[#E5E7EB] hover:bg-[#F8F9FA] text-[#475569]"
              }`}
            >
              <input
                type="checkbox"
                checked={features.dms}
                onChange={() => toggleFeature("dms")}
                className="h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                data-testid="calc-check-dms"
              />
              <span className="text-xs sm:text-sm font-medium">{t.featureDMSLabel}</span>
            </label>

            {/* Feature 2: Audio */}
            <label
              className={`flex items-center gap-3 p-4 rounded-xl border cursor-pointer min-h-[44px] transition-all focus-within:ring-2 focus-within:ring-blue-600 focus-within:outline-hidden ${
                features.audio
                  ? "border-blue-600 bg-blue-50/40 text-[#0F172A]"
                  : "border-[#E5E7EB] hover:bg-[#F8F9FA] text-[#475569]"
              }`}
            >
              <input
                type="checkbox"
                checked={features.audio}
                onChange={() => toggleFeature("audio")}
                className="h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                data-testid="calc-check-audio"
              />
              <span className="text-xs sm:text-sm font-medium">{t.featureAudioLabel}</span>
            </label>

            {/* Feature 3: Video */}
            <label
              className={`flex items-center gap-3 p-4 rounded-xl border cursor-pointer min-h-[44px] transition-all focus-within:ring-2 focus-within:ring-blue-600 focus-within:outline-hidden ${
                features.video
                  ? "border-blue-600 bg-blue-50/40 text-[#0F172A]"
                  : "border-[#E5E7EB] hover:bg-[#F8F9FA] text-[#475569]"
              }`}
            >
              <input
                type="checkbox"
                checked={features.video}
                onChange={() => toggleFeature("video")}
                className="h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                data-testid="calc-check-video"
              />
              <span className="text-xs sm:text-sm font-medium">{t.featureVideoLabel}</span>
            </label>

            {/* Feature 4: Disc Ingestion */}
            <label
              className={`flex items-center gap-3 p-4 rounded-xl border cursor-pointer min-h-[44px] transition-all focus-within:ring-2 focus-within:ring-blue-600 focus-within:outline-hidden ${
                features.disc
                  ? "border-blue-600 bg-blue-50/40 text-[#0F172A]"
                  : "border-[#E5E7EB] hover:bg-[#F8F9FA] text-[#475569]"
              }`}
            >
              <input
                type="checkbox"
                checked={features.disc}
                onChange={() => toggleFeature("disc")}
                className="h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                data-testid="calc-check-disc"
              />
              <span className="text-xs sm:text-sm font-medium">{t.featureDiscLabel}</span>
            </label>

            {/* Feature 5: AI & RAG */}
            <label
              className={`flex items-center gap-3 p-4 rounded-xl border cursor-pointer min-h-[44px] transition-all sm:col-span-2 lg:col-span-1 focus-within:ring-2 focus-within:ring-blue-600 focus-within:outline-hidden ${
                features.ai
                  ? "border-blue-600 bg-blue-50/40 text-[#0F172A]"
                  : "border-[#E5E7EB] hover:bg-[#F8F9FA] text-[#475569]"
              }`}
            >
              <input
                type="checkbox"
                checked={features.ai}
                onChange={() => toggleFeature("ai")}
                className="h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                data-testid="calc-check-ai"
              />
              <span className="text-xs sm:text-sm font-medium">{t.featureAILabel}</span>
            </label>
          </div>

          {/* Calculator Result Box */}
          <div
            className="rounded-xl border border-blue-200 bg-blue-50/50 p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4"
            data-testid="calc-result"
          >
            <div className="space-y-1.5">
              <span className="text-xs font-semibold uppercase tracking-wider text-blue-700">
                {t.recommendedTierLabel}
              </span>
              <h3 className="text-lg font-bold text-[#0F172A]">
                {recommendedTier === "light"
                  ? t.tierLightTitle
                  : recommendedTier === "media"
                  ? t.tierMediaTitle
                  : t.tierAITitle}
              </h3>
              <p className="text-xs text-[#64748B]">
                {recommendedTier === "light"
                  ? t.tierLightDesc
                  : recommendedTier === "media"
                  ? t.tierMediaDesc
                  : t.tierAIDesc}
              </p>
              <div className="pt-2 flex flex-wrap gap-2 text-xs">
                <span className="rounded-md bg-white border border-blue-200 px-2.5 py-1 text-[#334155]" data-testid="calc-target-cpu">
                  <strong className="text-[#0F172A]">CPU:</strong>{" "}
                  {recommendedTier === "light" ? t.tierLightCpu : recommendedTier === "media" ? t.tierMediaCpu : t.tierAICpu}
                </span>
                <span className="rounded-md bg-white border border-blue-200 px-2.5 py-1 text-[#334155]" data-testid="calc-target-ram">
                  <strong className="text-[#0F172A]">RAM:</strong>{" "}
                  {recommendedTier === "light" ? t.tierLightRam : recommendedTier === "media" ? t.tierMediaRam : t.tierAIRam}
                </span>
                <span className="rounded-md bg-white border border-blue-200 px-2.5 py-1 text-[#334155]" data-testid="calc-target-storage">
                  <strong className="text-[#0F172A]">Storage:</strong>{" "}
                  {recommendedTier === "light" ? t.tierLightStorage : recommendedTier === "media" ? t.tierMediaStorage : t.tierAIStorage}
                </span>
              </div>
            </div>

            <div className="flex items-center gap-2 self-start sm:self-auto">
              <span className="text-xs font-semibold px-3 py-1.5 rounded-lg bg-white border border-blue-200 text-blue-800" data-testid="calc-target-power">
                {recommendedTier === "light"
                  ? t.tierLightPower
                  : recommendedTier === "media"
                  ? t.tierMediaPower
                  : t.tierAIPower}
              </span>
            </div>
          </div>
        </section>

        {/* SECTION 3: Turnkey Hardware Appliance Roadmap & Architectural Preorder Hooks */}
        <section
          role="region"
          aria-labelledby="turnkey-heading"
          className="rounded-2xl border border-[#E5E7EB] bg-gradient-to-br from-white to-[#F8FAFC] p-6 sm:p-8 shadow-xs space-y-6"
        >
          <div className="max-w-2xl">
            <h2 id="turnkey-heading" className="text-xl font-bold text-[#0F172A]">
              {t.turnkeyTitle}
            </h2>
            <p className="text-xs sm:text-sm text-[#64748B] mt-1">
              {t.turnkeySubtitle}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Edition 1: Sovereign Mini */}
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-5 flex flex-col justify-between gap-4">
              <div>
                <span className="text-xs font-bold uppercase tracking-wider text-emerald-600">
                  Phase 1 · Q2 2027
                </span>
                <h3 className="text-base font-bold text-[#0F172A] mt-1">Sovereign Mini</h3>
                <p className="text-xs text-[#64748B] mt-1">
                  0.6L CNC aluminum fanless cube. 8 GB RAM, 512 GB NVMe, sub-8W idle. Zero noise.
                </p>
              </div>
              <button
                type="button"
                onClick={() => handleOpenPreorder("mini")}
                className="w-full min-h-[44px] rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] hover:bg-[#F1F5F9] text-xs font-semibold text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                data-testid="reserve-btn-mini"
              >
                {t.turnkeyPreorderBtn}
              </button>
            </div>

            {/* Edition 2: Sovereign Home */}
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-5 flex flex-col justify-between gap-4">
              <div>
                <span className="text-xs font-bold uppercase tracking-wider text-blue-600">
                  Phase 2 · Q4 2027
                </span>
                <h3 className="text-base font-bold text-[#0F172A] mt-1">Sovereign Home</h3>
                <p className="text-xs text-[#64748B] mt-1">
                  4-bay hot-swap mini tower with integrated slot-loading Blu-ray ingestion drive and dual 2.5 GbE.
                </p>
              </div>
              <button
                type="button"
                onClick={() => handleOpenPreorder("home")}
                className="w-full min-h-[44px] rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] hover:bg-[#F1F5F9] text-xs font-semibold text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                data-testid="reserve-btn-home"
              >
                {t.turnkeyPreorderBtn}
              </button>
            </div>

            {/* Edition 3: Sovereign Pro Studio */}
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-5 flex flex-col justify-between gap-4">
              <div>
                <span className="text-xs font-bold uppercase tracking-wider text-purple-600">
                  Phase 3 · 2028
                </span>
                <h3 className="text-base font-bold text-[#0F172A] mt-1">Sovereign Pro Studio</h3>
                <p className="text-xs text-[#64748B] mt-1">
                  Chambered whisper-quiet workstation with 16 GB VRAM AI tensor accelerator and integrated UPS battery backup.
                </p>
              </div>
              <button
                type="button"
                onClick={() => handleOpenPreorder("studio")}
                className="w-full min-h-[44px] rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] hover:bg-[#F1F5F9] text-xs font-semibold text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                data-testid="reserve-btn-studio"
              >
                {t.turnkeyPreorderBtn}
              </button>
            </div>
          </div>
        </section>

        {/* Back Link */}
        <div className="flex justify-between items-center border-t border-[#E5E7EB] pt-6">
          <Link
            href="/guides"
            className="inline-flex min-h-[44px] items-center gap-2 text-xs sm:text-sm font-semibold text-blue-600 hover:text-blue-800 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
          >
            <span>←</span>
            <span>{t.guidesTitle}</span>
          </Link>

          <Link
            href="/settings"
            className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-white px-4 py-2 text-xs sm:text-sm font-medium text-[#0F172A] hover:bg-[#F8F9FA] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
          >
            <span>⚙️</span>
            <span>{t.settings}</span>
          </Link>
        </div>
      </main>

      {/* Pre-order Reservation Modal */}
      {isPreorderModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby="modal-preorder-title"
        >
          <div
            ref={modalRef}
            className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-[#E5E7EB] space-y-5"
          >
            <div>
              <div className="flex items-center justify-between">
                <h3 id="modal-preorder-title" className="text-base font-bold text-[#0F172A]">
                  {t.turnkeyModalTitle}
                </h3>
                <button
                  type="button"
                  onClick={() => setIsPreorderModalOpen(false)}
                  className="min-h-[44px] min-w-[44px] flex items-center justify-center text-[#64748B] hover:text-[#0F172A] rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                  aria-label={t.close}
                >
                  ✕
                </button>
              </div>
              <div className="mt-1">
                <span
                  className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-blue-50 border border-blue-200 text-blue-700 text-xs font-semibold"
                  data-testid="modal-selected-model"
                >
                  <span>🏷️</span>
                  <span>
                    {selectedModel === "mini"
                      ? "Sovereign Mini (Phase 1)"
                      : selectedModel === "home"
                      ? "Sovereign Home (Phase 2)"
                      : "Sovereign Pro Studio (Phase 3)"}
                  </span>
                </span>
              </div>
            </div>

            {reservationToken ? (
              <div className="space-y-4" data-testid="reservation-success">
                <div className="rounded-xl bg-emerald-50 border border-emerald-200 p-4 text-emerald-800 text-xs sm:text-sm space-y-2">
                  <p className="font-semibold">{t.turnkeyReservedNotice}</p>
                  <p className="font-mono text-emerald-950 font-bold text-sm tracking-wider">
                    {reservationToken}
                  </p>
                  <button
                    type="button"
                    onClick={() => {
                      if (typeof navigator !== "undefined" && navigator.clipboard) {
                        navigator.clipboard.writeText(reservationToken);
                        setTokenCopied(true);
                        setTimeout(() => setTokenCopied(false), 2000);
                      }
                    }}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-emerald-300 bg-white text-emerald-800 text-xs font-semibold hover:bg-emerald-100 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-600"
                    data-testid="copy-token-btn"
                  >
                    <span>📋</span>
                    <span>{tokenCopied ? t.copied : t.copyPath || "Copy Token"}</span>
                  </button>
                </div>
                <button
                  type="button"
                  onClick={() => setIsPreorderModalOpen(false)}
                  className="w-full min-h-[44px] rounded-xl bg-blue-600 text-white text-xs sm:text-sm font-semibold hover:bg-blue-700 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                >
                  {t.close}
                </button>
              </div>
            ) : (
              <form onSubmit={handleConfirmPreorder} className="space-y-4">
                <div>
                  <label htmlFor="preorder-email" className="block text-xs font-semibold text-[#0F172A] mb-1.5">
                    {t.turnkeyEmailLabel}
                  </label>
                  <input
                    ref={emailInputRef}
                    id="preorder-email"
                    type="email"
                    required
                    value={preorderEmail}
                    onChange={(e) => setPreorderEmail(e.target.value)}
                    placeholder="family@household.local"
                    className="w-full min-h-[44px] rounded-lg border border-[#E5E7EB] px-3.5 text-xs sm:text-sm text-[#0F172A] focus:border-blue-600 focus:outline-none focus:ring-1 focus:ring-blue-600"
                    data-testid="preorder-email-input"
                  />
                </div>

                <div className="pt-2 flex justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setIsPreorderModalOpen(false)}
                    className="min-h-[44px] px-4 rounded-xl border border-[#E5E7EB] bg-[#F8F9FA] text-xs font-semibold text-[#475569] hover:bg-[#F1F5F9] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                  >
                    {t.cancel}
                  </button>
                  <button
                    type="submit"
                    className="min-h-[44px] px-5 rounded-xl bg-blue-600 text-white text-xs font-semibold hover:bg-blue-700 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                    data-testid="preorder-submit-btn"
                  >
                    {t.turnkeySubmitBtn}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
