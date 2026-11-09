"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { Language } from "@/lib/i18n";
import { useLanguage } from "@/lib/hooks";
import {
  GuideTopicId,
  GuideSection,
  GuideTopicContent,
  guidesData,
} from "@/lib/guides-data";

export { guidesData, type GuideTopicId, type GuideSection, type GuideTopicContent };

export interface GuidesPageProps {
  initialLanguage?: Language;
  initialTopic?: GuideTopicId;
}

export default function GuidesPage({ initialLanguage, initialTopic = "quickstart" }: GuidesPageProps = {}) {
  const { lang, t, handleLanguageChange } = useLanguage(initialLanguage);

  const [activeTopicId, setActiveTopicId] = useState<GuideTopicId>(initialTopic);
  const topics = guidesData[lang] || guidesData.en;
  const currentTopic = topics.find((top) => top.id === activeTopicId) || topics[0];
  const navRef = useRef<HTMLElement>(null);

  // Synchronize hash in URL (#storage, #media, #ai, #resilience, #quickstart)
  useEffect(() => {
    if (typeof window !== "undefined") {
      const handleHashChange = () => {
        const hash = window.location.hash.replace("#", "") as GuideTopicId;
        if (["quickstart", "storage", "media", "ai", "resilience"].includes(hash)) {
          setActiveTopicId(hash);
        }
      };

      if (!initialTopic && window.location.hash) {
        handleHashChange();
      }

      window.addEventListener("hashchange", handleHashChange);
      return () => window.removeEventListener("hashchange", handleHashChange);
    }
  }, [initialTopic]);

  const selectTopic = (id: GuideTopicId) => {
    setActiveTopicId(id);
    if (typeof window !== "undefined") {
      window.location.hash = id;
    }
  };

  // Keyboard navigation for sidebar topic buttons
  const handleNavKeyDown = (e: React.KeyboardEvent<HTMLElement>) => {
    const topicIds: GuideTopicId[] = ["quickstart", "storage", "media", "ai", "resilience"];
    const currentIndex = topicIds.indexOf(activeTopicId);
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIndex = (currentIndex + 1) % topicIds.length;
      selectTopic(topicIds[nextIndex]);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIndex = (currentIndex - 1 + topicIds.length) % topicIds.length;
      selectTopic(topicIds[prevIndex]);
    }
  };

  return (
    <div className="flex min-h-screen flex-col bg-[#F8F9FA] text-[#0F172A] font-sans">
      {/* 64px Header */}
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={handleLanguageChange}
        aggregateStatus="operational"
      />

      {/* Main Guides Layout */}
      <div className="mx-auto flex w-full max-w-[1200px] flex-1 flex-col gap-6 px-4 py-8 sm:px-6 sm:py-10 md:grid md:grid-cols-[280px_1fr] md:gap-8">
        {/* Sidebar Topics Navigation */}
        <aside>
          <div className="mb-4 flex flex-wrap gap-2">
            <Link
              href="/"
              className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-[#0F172A] shadow-xs hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              aria-label={t.backToDashboard}
            >
              <span aria-hidden="true">←</span>
              <span>{t.backToDashboard}</span>
            </Link>
            <Link
              href="/landing"
              className="inline-flex min-h-[44px] items-center gap-1.5 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-blue-600 shadow-xs hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              aria-label={t.landingNavLanding}
              data-testid="guides-showcase-link"
            >
              <span aria-hidden="true">✨</span>
              <span>{t.landingNavLanding}</span>
            </Link>
          </div>

          <div className="mb-3 px-1">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-[#64748B]">
              {t.guidesTitle}
            </h2>
          </div>

          <nav
            ref={navRef}
            onKeyDown={handleNavKeyDown}
            className="flex flex-col gap-1.5"
            aria-label="Guide topics"
            role="tablist"
          >
            {topics.map((top) => {
              const isSelected = top.id === activeTopicId;
              return (
                <button
                  key={top.id}
                  type="button"
                  role="tab"
                  id={`tab-${top.id}`}
                  aria-selected={isSelected}
                  aria-controls={`panel-${top.id}`}
                  onClick={() => selectTopic(top.id)}
                  className={`flex min-h-[44px] items-center gap-3 rounded-lg px-3.5 py-2.5 text-xs sm:text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                    isSelected
                      ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                      : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
                  }`}
                  data-testid={`guide-tab-${top.id}`}
                >
                  <span className="text-lg" aria-hidden="true">{top.icon}</span>
                  <span className="truncate">{top.title}</span>
                </button>
              );
            })}
          </nav>

          <div className="mt-6 border-t border-[#E5E7EB] pt-4 px-1">
            <Link
              href="/hardware"
              className="flex min-h-[44px] items-center gap-2.5 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-blue-600 hover:text-blue-800 hover:bg-[#F1F5F9] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 shadow-xs"
              data-testid="hardware-guide-link"
            >
              <span aria-hidden="true">💻</span>
              <span>{t.hardwareTitle}</span>
              <span className="ml-auto text-xs text-[#64748B]">→</span>
            </Link>
          </div>
        </aside>

        {/* Content Viewer Pane */}
        <main
          id={`panel-${currentTopic.id}`}
          role="tabpanel"
          aria-labelledby={`tab-${currentTopic.id}`}
          className="flex flex-col gap-6"
          data-testid="guide-content-panel"
        >
          <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 sm:p-8 shadow-xs">
            {/* Header */}
            <div className="flex items-start gap-4 pb-6 border-b border-[#F1F5F9]">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-blue-50 text-2xl border border-blue-100" aria-hidden="true">
                {currentTopic.icon}
              </div>
              <div className="flex-1">
                <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[#0F172A]">
                  {currentTopic.title}
                </h1>
                <p className="mt-1 text-sm text-[#64748B] leading-relaxed">
                  {currentTopic.summary}
                </p>
              </div>
            </div>

            {/* Guide Sections */}
            <div className="mt-8 space-y-8 divide-y divide-[#F1F5F9]">
              {currentTopic.sections.map((sec, idx) => (
                <div key={idx} className={idx > 0 ? "pt-8" : ""}>
                  <h3 className="text-base font-semibold text-[#0F172A] mb-3">
                    {sec.title}
                  </h3>
                  <div className="space-y-3">
                    {sec.content.map((p, pIdx) => (
                      <p key={pIdx} className="text-sm text-[#475569] leading-relaxed">
                        {p}
                      </p>
                    ))}
                  </div>
                </div>
              ))}
            </div>

            {/* Bottom Actions */}
            <div className="mt-10 flex flex-wrap items-center justify-between gap-4 border-t border-[#F1F5F9] pt-6">
              <Link
                href="/settings"
                className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] px-4 py-2 text-xs sm:text-sm font-medium text-[#0F172A] hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              >
                <span>⚙️</span>
                <span>{t.settings}</span>
              </Link>

              <Link
                href="/drop"
                className="inline-flex min-h-[44px] items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-xs sm:text-sm font-semibold text-white shadow-xs hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              >
                <span>📥</span>
                <span>{t.dropMedia}</span>
              </Link>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}
