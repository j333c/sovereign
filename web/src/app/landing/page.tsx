"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { Language, translations, getInitialLanguage, persistLanguage } from "@/lib/i18n";
import {
  calculateHardwareTier,
  HardwareCalculatorState,
  HardwareTier,
} from "@/app/hardware/page";

export interface LandingPageProps {
  initialLanguage?: Language;
  initialActiveTab?: "video" | "audio" | "dms" | "ai" | "disc";
  initialFeatures?: HardwareCalculatorState;
  aggregateStatus?: "operational" | "healthy" | "degraded" | "unhealthy";
}

type FeatureTabKey = "video" | "audio" | "dms" | "ai" | "disc";

const REPO_URL = process.env.NEXT_PUBLIC_REPO_URL || "https://github.com/jens/sovereign";

export default function LandingPage({
  initialLanguage,
  initialActiveTab = "video",
  initialFeatures,
  aggregateStatus = "healthy",
}: LandingPageProps = {}) {
  const [lang, setLang] = useState<Language>(() => getInitialLanguage(initialLanguage));
  const t = translations[lang];

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.title = t.landingTitle;
    }
  }, [t.landingTitle]);

  const handleLanguageChange = (newLang: Language) => {
    setLang(newLang);
    persistLanguage(newLang);
  };

  const [activeTab, setActiveTab] = useState<FeatureTabKey>(initialActiveTab);

  // Keyboard navigation for WAI-ARIA tablist
  const tabKeys: FeatureTabKey[] = ["video", "audio", "dms", "ai", "disc"];

  const handleTabKeyDown = (e: React.KeyboardEvent<HTMLElement>, currentIndex: number) => {
    if (e.key === "ArrowRight") {
      e.preventDefault();
      const nextIndex = (currentIndex + 1) % tabKeys.length;
      setActiveTab(tabKeys[nextIndex]);
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      const prevIndex = (currentIndex - 1 + tabKeys.length) % tabKeys.length;
      setActiveTab(tabKeys[prevIndex]);
    } else if (e.key === "Home") {
      e.preventDefault();
      setActiveTab(tabKeys[0]);
    } else if (e.key === "End") {
      e.preventDefault();
      setActiveTab(tabKeys[tabKeys.length - 1]);
    }
  };

  // Calculator state
  const [features, setFeatures] = useState<HardwareCalculatorState>(
    () =>
      initialFeatures ?? {
        dms: true,
        audio: true,
        video: false,
        disc: false,
        ai: false,
      }
  );

  const recommendedTier = calculateHardwareTier(features);

  const toggleFeature = (key: keyof HardwareCalculatorState) => {
    setFeatures((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  return (
    <div className="min-h-screen bg-[#F8F9FA] text-[#0F172A] flex flex-col font-sans">
      {/* 64px Persistent Header */}
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={handleLanguageChange}
        aggregateStatus={aggregateStatus}
      />

      <main className="flex-1 w-full max-w-6xl mx-auto px-4 py-8 sm:py-14 flex flex-col gap-16 sm:gap-24">
        {/* HERO SECTION */}
        <section
          role="region"
          aria-labelledby="hero-heading"
          className="flex flex-col items-center text-center gap-6 pt-4 sm:pt-8"
        >
          <div className="inline-flex items-center gap-2 px-3.5 py-1 rounded-full bg-[#EFF6FF] text-[#1D4ED8] text-xs font-semibold border border-[#BFDBFE]">
            <span>🛡️</span>
            <span>{t.landingHeroBadge}</span>
          </div>

          <h1
            id="hero-heading"
            className="text-4xl sm:text-6xl font-extrabold tracking-tight text-[#0F172A] max-w-4xl leading-[1.15]"
          >
            {t.landingHeroHeadline}
          </h1>

          <p className="text-base sm:text-lg text-[#64748B] max-w-2xl leading-relaxed">
            {t.landingHeroSubheadline}
          </p>

          {/* Call-to-Action Buttons */}
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <Link
              href="/"
              className="inline-flex min-h-[48px] items-center justify-center rounded-xl bg-blue-600 px-6 text-sm font-semibold text-white shadow-xs hover:bg-blue-700 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2"
              data-testid="landing-cta-dashboard"
            >
              <span>{t.landingCtaDashboard}</span>
              <span className="ml-2" aria-hidden="true">→</span>
            </Link>

            <Link
              href="/hardware"
              className="inline-flex min-h-[48px] items-center justify-center rounded-xl border border-[#CBD5E1] bg-white px-5 text-sm font-semibold text-[#0F172A] hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
              data-testid="landing-cta-hardware"
            >
              <span className="mr-2" aria-hidden="true">💻</span>
              <span>{t.landingCtaHardware}</span>
            </Link>

            <a
              href={REPO_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-[48px] items-center justify-center rounded-xl border border-[#E2E8F0] bg-[#F1F5F9] px-5 text-sm font-semibold text-[#334155] hover:bg-[#E2E8F0] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
              data-testid="landing-cta-github"
            >
              <span className="mr-2" aria-hidden="true">🐙</span>
              <span>{t.landingCtaGitHub}</span>
            </a>
          </div>

          {/* Trust Stat Pills */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-6 w-full max-w-3xl">
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-3.5 flex flex-col items-center">
              <span className="text-xl sm:text-2xl font-bold text-blue-600">0</span>
              <span className="text-xs text-[#64748B] mt-0.5 font-medium">{t.landingStatPings}</span>
            </div>
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-3.5 flex flex-col items-center">
              <span className="text-xl sm:text-2xl font-bold text-emerald-600">100%</span>
              <span className="text-xs text-[#64748B] mt-0.5 font-medium">{t.landingStatStorage}</span>
            </div>
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-3.5 flex flex-col items-center">
              <span className="text-xl sm:text-2xl font-bold text-purple-600">5 in 1</span>
              <span className="text-xs text-[#64748B] mt-0.5 font-medium">{t.landingStatSpokes}</span>
            </div>
            <div className="rounded-xl border border-[#E5E7EB] bg-white p-3.5 flex flex-col items-center">
              <span className="text-xl sm:text-2xl font-bold text-amber-600">Sub-10W</span>
              <span className="text-xs text-[#64748B] mt-0.5 font-medium">{t.landingStatPower}</span>
            </div>
          </div>
        </section>

        {/* SECTION 2: INTERACTIVE FEATURE SHOWCASE */}
        <section
          role="region"
          aria-labelledby="features-heading"
          className="space-y-6"
        >
          <div className="text-center max-w-2xl mx-auto space-y-2">
            <h2 id="features-heading" className="text-2xl sm:text-3xl font-bold text-[#0F172A]">
              {t.landingFeatureShowcaseTitle}
            </h2>
            <p className="text-xs sm:text-sm text-[#64748B]">
              {t.landingFeatureShowcaseSubtitle}
            </p>
          </div>

          {/* Feature Tabs */}
          <div
            className="flex flex-wrap items-center justify-center gap-2 border-b border-[#E5E7EB] pb-4"
            role="tablist"
            aria-label="Core appliance features"
          >
            {[
              { id: "video", label: t.landingFeatureVideoTitle, icon: "🎬" },
              { id: "audio", label: t.landingFeatureAudioTitle, icon: "🎵" },
              { id: "dms", label: t.landingFeatureDMSTitle, icon: "📑" },
              { id: "ai", label: t.landingFeatureAITitle, icon: "🤖" },
              { id: "disc", label: t.landingFeatureDiscTitle, icon: "💿" },
            ].map((tab, idx) => {
              const isSelected = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  id={`tab-${tab.id}`}
                  tabIndex={isSelected ? 0 : -1}
                  aria-selected={isSelected}
                  aria-controls={`panel-${tab.id}`}
                  onClick={() => setActiveTab(tab.id as FeatureTabKey)}
                  onKeyDown={(e) => handleTabKeyDown(e, idx)}
                  className={`flex min-h-[44px] items-center gap-2 rounded-xl px-4 py-2 text-xs sm:text-sm font-semibold transition-all cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                    isSelected
                      ? "bg-blue-600 text-white shadow-xs"
                      : "bg-white text-[#475569] border border-[#E5E7EB] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
                  }`}
                  data-testid={`feature-tab-${tab.id}`}
                >
                  <span aria-hidden="true">{tab.icon}</span>
                  <span>{tab.label}</span>
                </button>
              );
            })}
          </div>

          {/* Tab Panels (all panels in DOM to preserve aria-controls targets) */}
          <div className="relative">
            {/* Video Panel */}
            <div
              id="panel-video"
              role="tabpanel"
              aria-labelledby="tab-video"
              tabIndex={0}
              hidden={activeTab !== "video"}
              className={
                activeTab === "video"
                  ? "rounded-2xl border border-[#E5E7EB] bg-white p-6 sm:p-10 shadow-xs space-y-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                  : "hidden"
              }
              data-testid="feature-panel-video"
            >
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-center">
                <div className="space-y-4">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-50 text-emerald-800 text-xs font-semibold border border-emerald-200">
                    <span>{t.landingBadgeVideo}</span>
                  </div>
                  <h3 className="text-xl sm:text-2xl font-bold text-[#0F172A]">
                    {t.landingFeatureVideoTitle}
                  </h3>
                  <p className="text-sm text-[#475569] leading-relaxed">
                    {t.landingFeatureVideoDesc}
                  </p>
                  <ul className="space-y-2 text-xs sm:text-sm text-[#334155]">
                    <li className="flex items-center gap-2">
                      <span className="text-emerald-600">✓</span>
                      <span>Full 4K HEVC / AV1 real-time transcoding with tone mapping</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-emerald-600">✓</span>
                      <span>Zero subscription fees; stream to Smart TVs, phones, and tablets</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-emerald-600">✓</span>
                      <span>Isolated LAN access via Caddy reverse proxy without port forwarding</span>
                    </li>
                  </ul>
                  <div className="pt-2">
                    <Link
                      href="/guides#media"
                      className="inline-flex min-h-[44px] items-center gap-1.5 px-3 py-1.5 rounded-lg border border-[#E5E7EB] text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                    >
                      <span>{t.landingReadGuideMedia}</span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </div>
                </div>
                <div className="rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-6 space-y-3 font-mono text-xs">
                  <div className="flex items-center justify-between border-b border-[#E2E8F0] pb-2 text-[#64748B]">
                    <span>Jellyfin 4K Ingestion & Transcoding</span>
                    <span className="text-emerald-600">● 60 FPS</span>
                  </div>
                  <div className="space-y-1.5 text-[#334155]">
                    <p>Codec: HEVC Main 10 (4K HDR10)</p>
                    <p>Encoder: Intel QuickSync QSV VA-API</p>
                    <p>Tone Mapping: BT.2020 to BT.709 OpenCL</p>
                    <p>Bitrate: 45.2 Mbps → 18.0 Mbps Adaptive</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Audio Panel */}
            <div
              id="panel-audio"
              role="tabpanel"
              aria-labelledby="tab-audio"
              tabIndex={0}
              hidden={activeTab !== "audio"}
              className={
                activeTab === "audio"
                  ? "rounded-2xl border border-[#E5E7EB] bg-white p-6 sm:p-10 shadow-xs space-y-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                  : "hidden"
              }
              data-testid="feature-panel-audio"
            >
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-center">
                <div className="space-y-4">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-50 text-blue-800 text-xs font-semibold border border-blue-200">
                    <span>{t.landingBadgeAudio}</span>
                  </div>
                  <h3 className="text-xl sm:text-2xl font-bold text-[#0F172A]">
                    {t.landingFeatureAudioTitle}
                  </h3>
                  <p className="text-sm text-[#475569] leading-relaxed">
                    {t.landingFeatureAudioDesc}
                  </p>
                  <ul className="space-y-2 text-xs sm:text-sm text-[#334155]">
                    <li className="flex items-center gap-2">
                      <span className="text-blue-600">✓</span>
                      <span>Subsonic API compatibility for DSub, Symfonium, and Amperfy</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-blue-600">✓</span>
                      <span>Chapter bookmarking, playback speed control, and sleep timers</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-blue-600">✓</span>
                      <span>Bit-perfect lossless playback without re-encoding</span>
                    </li>
                  </ul>
                  <div className="pt-2">
                    <Link
                      href="/guides#media"
                      className="inline-flex min-h-[44px] items-center gap-1.5 px-3 py-1.5 rounded-lg border border-[#E5E7EB] text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                    >
                      <span>{t.landingReadGuideAudio}</span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </div>
                </div>
                <div className="rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-6 space-y-3 font-mono text-xs">
                  <div className="flex items-center justify-between border-b border-[#E2E8F0] pb-2 text-[#64748B]">
                    <span>Navidrome & Audiobookshelf</span>
                    <span className="text-blue-600">● Bit-Perfect</span>
                  </div>
                  <div className="space-y-1.5 text-[#334155]">
                    <p>Format: FLAC 96kHz / 24-bit Lossless</p>
                    <p>Subsonic API: v1.16.1 Active</p>
                    <p>Audiobook Sync: Multi-device Progress</p>
                    <p>Storage: Local /data/media/audio</p>
                  </div>
                </div>
              </div>
            </div>

            {/* DMS Panel */}
            <div
              id="panel-dms"
              role="tabpanel"
              aria-labelledby="tab-dms"
              tabIndex={0}
              hidden={activeTab !== "dms"}
              className={
                activeTab === "dms"
                  ? "rounded-2xl border border-[#E5E7EB] bg-white p-6 sm:p-10 shadow-xs space-y-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                  : "hidden"
              }
              data-testid="feature-panel-dms"
            >
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-center">
                <div className="space-y-4">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-amber-50 text-amber-800 text-xs font-semibold border border-amber-200">
                    <span>{t.landingBadgeDms}</span>
                  </div>
                  <h3 className="text-xl sm:text-2xl font-bold text-[#0F172A]">
                    {t.landingFeatureDMSTitle}
                  </h3>
                  <p className="text-sm text-[#475569] leading-relaxed">
                    {t.landingFeatureDMSDesc}
                  </p>
                  <ul className="space-y-2 text-xs sm:text-sm text-[#334155]">
                    <li className="flex items-center gap-2">
                      <span className="text-amber-600">✓</span>
                      <span>Paperless-ngx with Tesseract OCR multi-language optical indexing</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-amber-600">✓</span>
                      <span>Automatic contract deadline detection & cancellation calendar reminders</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-amber-600">✓</span>
                      <span>Encrypted IMAP email polling and drag-and-drop web file ingestion</span>
                    </li>
                  </ul>
                  <div className="pt-2">
                    <Link
                      href="/guides#storage"
                      className="inline-flex min-h-[44px] items-center gap-1.5 px-3 py-1.5 rounded-lg border border-[#E5E7EB] text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                    >
                      <span>{t.landingReadGuideDms}</span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </div>
                </div>
                <div className="rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-6 space-y-3 font-mono text-xs">
                  <div className="flex items-center justify-between border-b border-[#E2E8F0] pb-2 text-[#64748B]">
                    <span>Paperless-ngx OCR Engine</span>
                    <span className="text-amber-600">● Tesseract 5.3</span>
                  </div>
                  <div className="space-y-1.5 text-[#334155]">
                    <p>Languages: deu, eng, spa, fra</p>
                    <p>Auto-tag: Invoices, Contracts, Taxes</p>
                    <p>Deadline Extractor: Active (7-day lead)</p>
                    <p>Archive: PDF/A-2b Long-Term Standard</p>
                  </div>
                </div>
              </div>
            </div>

            {/* AI Panel */}
            <div
              id="panel-ai"
              role="tabpanel"
              aria-labelledby="tab-ai"
              tabIndex={0}
              hidden={activeTab !== "ai"}
              className={
                activeTab === "ai"
                  ? "rounded-2xl border border-[#E5E7EB] bg-white p-6 sm:p-10 shadow-xs space-y-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                  : "hidden"
              }
              data-testid="feature-panel-ai"
            >
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-center">
                <div className="space-y-4">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-purple-50 text-purple-800 text-xs font-semibold border border-purple-200">
                    <span>{t.landingBadgeAi}</span>
                  </div>
                  <h3 className="text-xl sm:text-2xl font-bold text-[#0F172A]">
                    {t.landingFeatureAITitle}
                  </h3>
                  <p className="text-sm text-[#475569] leading-relaxed">
                    {t.landingFeatureAIDesc}
                  </p>
                  <ul className="space-y-2 text-xs sm:text-sm text-[#334155]">
                    <li className="flex items-center gap-2">
                      <span className="text-purple-600">✓</span>
                      <span>Air-gapped llama.cpp inference without internet routing</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-purple-600">✓</span>
                      <span>Vector-embedded document RAG across your household paperwork</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-purple-600">✓</span>
                      <span>Smart GPU arbitration prioritizing AI during daytime, media at night</span>
                    </li>
                  </ul>
                  <div className="pt-2">
                    <Link
                      href="/guides#ai"
                      className="inline-flex min-h-[44px] items-center gap-1.5 px-3 py-1.5 rounded-lg border border-[#E5E7EB] text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                    >
                      <span>{t.landingReadGuideAi}</span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </div>
                </div>
                <div className="rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-6 space-y-3 font-mono text-xs">
                  <div className="flex items-center justify-between border-b border-[#E2E8F0] pb-2 text-[#64748B]">
                    <span>llama.cpp Engine & Vector RAG</span>
                    <span className="text-purple-600">● Air-Gapped</span>
                  </div>
                  <div className="space-y-1.5 text-[#334155]">
                    <p>Network: sovereign-ai bridge (internal only)</p>
                    <p>Model: Qwen2.5 / Llama 3 8B (Q4_K_M)</p>
                    <p>Context: 8,192 tokens FlashAttention</p>
                    <p>VRAM Arbitration: Smart Governor Active</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Disc Panel */}
            <div
              id="panel-disc"
              role="tabpanel"
              aria-labelledby="tab-disc"
              tabIndex={0}
              hidden={activeTab !== "disc"}
              className={
                activeTab === "disc"
                  ? "rounded-2xl border border-[#E5E7EB] bg-white p-6 sm:p-10 shadow-xs space-y-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                  : "hidden"
              }
              data-testid="feature-panel-disc"
            >
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-center">
                <div className="space-y-4">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-slate-100 text-slate-800 text-xs font-semibold border border-slate-300">
                    <span>{t.landingBadgeDisc}</span>
                  </div>
                  <h3 className="text-xl sm:text-2xl font-bold text-[#0F172A]">
                    {t.landingFeatureDiscTitle}
                  </h3>
                  <p className="text-sm text-[#475569] leading-relaxed">
                    {t.landingFeatureDiscDesc}
                  </p>
                  <ul className="space-y-2 text-xs sm:text-sm text-[#334155]">
                    <li className="flex items-center gap-2">
                      <span className="text-slate-700">✓</span>
                      <span>Automatic Ripping Machine (ARM) headless udev detection</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-slate-700">✓</span>
                      <span>Lossless ripping of Audio CDs (FLAC/MusicBrainz), DVDs & Blu-rays</span>
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-slate-700">✓</span>
                      <span>Automatic tray ejection upon completion; zero screen required</span>
                    </li>
                  </ul>
                  <div className="pt-2">
                    <Link
                      href="/guides#media"
                      className="inline-flex min-h-[44px] items-center gap-1.5 px-3 py-1.5 rounded-lg border border-[#E5E7EB] text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                    >
                      <span>{t.landingReadGuideDisc}</span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </div>
                </div>
                <div className="rounded-xl border border-[#E2E8F0] bg-[#F8FAFC] p-6 space-y-3 font-mono text-xs">
                  <div className="flex items-center justify-between border-b border-[#E2E8F0] pb-2 text-[#64748B]">
                    <span>Automatic Ripping Machine (ARM)</span>
                    <span className="text-slate-700">● udev Ready</span>
                  </div>
                  <div className="space-y-1.5 text-[#334155]">
                    <p>Optical Drive: /dev/sr0 (LibreDrive flashed)</p>
                    <p>Audio CD: abcde / flac + MusicBrainz</p>
                    <p>Video Disc: MakeMKV + HandBrake</p>
                    <p>Workflow: Auto-rip → Transcode → Eject</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* SECTION 3: PRIVACY & ARCHITECTURAL INVARIANTS */}
        <section
          role="region"
          aria-labelledby="privacy-heading"
          className="rounded-2xl border border-[#E2E8F0] bg-white p-6 sm:p-10 shadow-xs space-y-8"
        >
          <div className="text-center max-w-2xl mx-auto space-y-2">
            <h2 id="privacy-heading" className="text-2xl sm:text-3xl font-bold text-[#0F172A]">
              {t.landingPrivacyTitle}
            </h2>
            <p className="text-xs sm:text-sm text-[#64748B]">
              {t.landingPrivacySubtitle}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Invariant 1: Air-Gapped AI */}
            <div className="rounded-xl border border-[#E5E7EB] bg-[#F8FAFC] p-5 space-y-2">
              <div className="flex items-center gap-2">
                <span className="text-xl" aria-hidden="true">🔒</span>
                <h3 className="text-sm font-bold text-[#0F172A]">
                  {t.landingPrivacyPill1Title}
                </h3>
              </div>
              <p className="text-xs text-[#475569] leading-relaxed">
                {t.landingPrivacyPill1Desc}
              </p>
            </div>

            {/* Invariant 2: Zero Docker Socket Mounts */}
            <div className="rounded-xl border border-[#E5E7EB] bg-[#F8FAFC] p-5 space-y-2">
              <div className="flex items-center gap-2">
                <span className="text-xl" aria-hidden="true">🛡️</span>
                <h3 className="text-sm font-bold text-[#0F172A]">
                  {t.landingPrivacyPill2Title}
                </h3>
              </div>
              <p className="text-xs text-[#475569] leading-relaxed">
                {t.landingPrivacyPill2Desc}
              </p>
            </div>

            {/* Invariant 3: CGO-Free WAL Durability */}
            <div className="rounded-xl border border-[#E5E7EB] bg-[#F8FAFC] p-5 space-y-2">
              <div className="flex items-center gap-2">
                <span className="text-xl" aria-hidden="true">💾</span>
                <h3 className="text-sm font-bold text-[#0F172A]">
                  {t.landingPrivacyPill3Title}
                </h3>
              </div>
              <p className="text-xs text-[#475569] leading-relaxed">
                {t.landingPrivacyPill3Desc}
              </p>
            </div>

            {/* Invariant 4: 180s Atomic Rollback Safety */}
            <div className="rounded-xl border border-[#E5E7EB] bg-[#F8FAFC] p-5 space-y-2">
              <div className="flex items-center gap-2">
                <span className="text-xl" aria-hidden="true">⏱️</span>
                <h3 className="text-sm font-bold text-[#0F172A]">
                  {t.landingPrivacyPill4Title}
                </h3>
              </div>
              <p className="text-xs text-[#475569] leading-relaxed">
                {t.landingPrivacyPill4Desc}
              </p>
            </div>
          </div>
        </section>

        {/* SECTION 4: HARDWARE TIER CALCULATOR */}
        <section
          role="region"
          aria-labelledby="calculator-heading"
          className="rounded-2xl border border-[#E2E8F0] bg-white p-6 sm:p-10 shadow-xs space-y-8"
        >
          <div className="text-center max-w-2xl mx-auto space-y-2">
            <h2 id="calculator-heading" className="text-2xl sm:text-3xl font-bold text-[#0F172A]">
              {t.calcTitle}
            </h2>
            <p className="text-xs sm:text-sm text-[#64748B]">
              {t.calcSubtitle}
            </p>
          </div>

          {/* Feature Checkboxes */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
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
              <h3 className="text-lg font-bold text-[#0F172A]" data-testid="calc-tier-title">
                {recommendedTier === "light"
                  ? t.tierLightTitle
                  : recommendedTier === "media"
                  ? t.tierMediaTitle
                  : t.tierAITitle}
              </h3>
              <p className="text-xs text-[#64748B]" data-testid="calc-tier-desc">
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

            <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3">
              <span className="text-xs font-semibold px-3 py-1.5 rounded-lg bg-white border border-blue-200 text-blue-800" data-testid="calc-target-power">
                {recommendedTier === "light"
                  ? t.tierLightPower
                  : recommendedTier === "media"
                  ? t.tierMediaPower
                  : t.tierAIPower}
              </span>
              <Link
                href="/hardware"
                className="inline-flex min-h-[44px] items-center gap-1.5 rounded-lg border border-[#CBD5E1] bg-white px-3.5 text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
              >
                <span>{t.landingFullHardwareGuides}</span>
                <span aria-hidden="true">→</span>
              </Link>
            </div>
          </div>
        </section>

        {/* SECTION 5: OPEN-SOURCE CALL TO ACTION */}
        <section
          role="region"
          aria-labelledby="cta-heading"
          className="rounded-2xl border border-[#BFDBFE] bg-[#EFF6FF] p-8 sm:p-12 text-center space-y-6 shadow-xs"
        >
          <div className="max-w-2xl mx-auto space-y-2">
            <h2 id="cta-heading" className="text-2xl sm:text-3xl font-bold text-[#1E3A8A]">
              {t.landingOpenSourceTitle}
            </h2>
            <p className="text-xs sm:text-sm text-[#1D4ED8] font-medium">
              {t.landingOpenSourceSubtitle}
            </p>
          </div>

          <div className="flex flex-wrap items-center justify-center gap-4 pt-2">
            <Link
              href="/"
              className="inline-flex min-h-[44px] items-center justify-center rounded-xl bg-blue-600 px-5 text-xs sm:text-sm font-semibold text-white hover:bg-blue-700 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
            >
              <span>{t.landingCtaDashboard}</span>
            </Link>

            <Link
              href="/guides"
              className="inline-flex min-h-[44px] items-center justify-center rounded-xl border border-blue-300 bg-white px-5 text-xs sm:text-sm font-semibold text-blue-900 hover:bg-blue-50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
            >
              <span>{t.guidesTitle}</span>
            </Link>

            <Link
              href="/hardware"
              className="inline-flex min-h-[44px] items-center justify-center rounded-xl border border-blue-300 bg-white px-5 text-xs sm:text-sm font-semibold text-blue-900 hover:bg-blue-50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
            >
              <span>{t.hardwareTitle}</span>
            </Link>

            <a
              href={REPO_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-[44px] items-center justify-center rounded-xl border border-blue-300 bg-white px-5 text-xs sm:text-sm font-semibold text-blue-900 hover:bg-blue-50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
            >
              <span>{t.landingCtaGitHub}</span>
            </a>
          </div>
        </section>
      </main>

      {/* Global Footer */}
      <footer className="w-full border-t border-[#E5E7EB] bg-white py-8 px-4 text-xs text-[#64748B]">
        <div className="max-w-6xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="font-bold text-[#0F172A]">Sovereign Home Appliance</span>
            <span>·</span>
            <span>{t.landingFooterBrandDesc}</span>
          </div>

          <div className="flex items-center gap-4">
            <Link
              href="/"
              className="inline-flex min-h-[44px] items-center px-1 hover:text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 rounded-md"
            >
              Dashboard
            </Link>
            <Link
              href="/guides"
              className="inline-flex min-h-[44px] items-center px-1 hover:text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 rounded-md"
            >
              {t.guidesTitle}
            </Link>
            <Link
              href="/hardware"
              className="inline-flex min-h-[44px] items-center px-1 hover:text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 rounded-md"
            >
              {t.hardwareTitle}
            </Link>
            <Link
              href="/settings"
              className="inline-flex min-h-[44px] items-center px-1 hover:text-[#0F172A] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 rounded-md"
            >
              {t.settings}
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}
