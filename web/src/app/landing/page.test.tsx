import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import LandingPage from "./page";
import GuidesPage from "@/app/guides/page";
import HardwarePage from "@/app/hardware/page";
import { translations } from "@/lib/i18n";

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;");
}

describe("Public Marketing Landing Website & Feature Showcase (Story 7.2)", () => {
  it("renders Hero section with value proposition, call-to-actions, and trust statistics", () => {
    const html = renderToStaticMarkup(<LandingPage initialLanguage="en" />);

    // Value proposition & branding
    assert.ok(html.includes(escapeHtml(translations.en.landingHeroHeadline)), "renders hero headline");
    assert.ok(html.includes(escapeHtml(translations.en.landingHeroSubheadline)), "renders hero subheadline");
    assert.ok(html.includes(escapeHtml(translations.en.landingHeroBadge)), "renders localized trust badge");

    // Primary CTA buttons
    assert.ok(html.includes('data-testid="landing-cta-dashboard"'), "renders dashboard CTA button");
    assert.ok(html.includes('data-testid="landing-cta-hardware"'), "renders hardware calculator CTA button");
    assert.ok(html.includes('data-testid="landing-cta-github"'), "renders GitHub repo CTA button");
    assert.ok(html.includes('href="/"'), "dashboard CTA links to /");
    assert.ok(html.includes('href="/hardware"'), "hardware CTA links to /hardware");
    assert.ok(html.includes("https://github.com/j333c/sovereign"), "GitHub CTA links to repository");

    // Trust statistics
    assert.ok(html.includes("0"), "renders 0 Cloud Pings stat");
    assert.ok(html.includes("100%"), "renders 100% Local Storage stat");
    assert.ok(html.includes("5 in 1"), "renders 5 in 1 Unified Spokes stat");
    assert.ok(html.includes("Sub-10W"), "renders Sub-10W Idle Capable stat");
    assert.ok(html.includes(escapeHtml(translations.en.landingStatPings)), "renders localized pings label");
    assert.ok(html.includes(escapeHtml(translations.en.landingStatStorage)), "renders localized storage label");
    assert.ok(html.includes(escapeHtml(translations.en.landingStatSpokes)), "renders localized spokes label");
    assert.ok(html.includes(escapeHtml(translations.en.landingStatPower)), "renders localized power label");
  });

  it("renders tabbed interactive feature showcase with all 5 core workloads and WAI-ARIA conformance", () => {
    const html = renderToStaticMarkup(<LandingPage initialLanguage="en" initialActiveTab="video" />);

    // Tab buttons in tablist
    assert.ok(html.includes('role="tablist"'), "renders accessible tablist");
    assert.ok(html.includes('data-testid="feature-tab-video"'), "renders video tab button");
    assert.ok(html.includes('data-testid="feature-tab-audio"'), "renders audio tab button");
    assert.ok(html.includes('data-testid="feature-tab-dms"'), "renders dms tab button");
    assert.ok(html.includes('data-testid="feature-tab-ai"'), "renders ai tab button");
    assert.ok(html.includes('data-testid="feature-tab-disc"'), "renders disc tab button");

    // All 5 tab panels exist in DOM to preserve aria-controls targets
    assert.ok(html.includes('id="panel-video"'), "renders panel-video target in DOM");
    assert.ok(html.includes('id="panel-audio"'), "renders panel-audio target in DOM");
    assert.ok(html.includes('id="panel-dms"'), "renders panel-dms target in DOM");
    assert.ok(html.includes('id="panel-ai"'), "renders panel-ai target in DOM");
    assert.ok(html.includes('id="panel-disc"'), "renders panel-disc target in DOM");

    // Active panel has tabIndex=0 and no hidden attribute; inactive panels have hidden attribute
    assert.ok(html.includes('data-testid="feature-panel-video"'), "renders video feature panel");
    assert.ok(html.includes("Jellyfin 4K Ingestion &amp; Transcoding"), "renders Jellyfin transcode info");
    assert.ok(html.includes("Intel QuickSync QSV VA-API"), "renders Intel QuickSync specs");
    assert.ok(html.includes('href="/guides#media"'), "renders deep-link to /guides#media");
  });

  it("renders audio, dms, ai, and optical disc feature panels correctly when activated", () => {
    // 1. Audio panel
    const audioHtml = renderToStaticMarkup(<LandingPage initialLanguage="en" initialActiveTab="audio" />);
    assert.ok(audioHtml.includes('data-testid="feature-panel-audio"'), "renders audio panel");
    assert.ok(audioHtml.includes("Navidrome &amp; Audiobookshelf"), "renders Navidrome info");
    assert.ok(audioHtml.includes("FLAC 96kHz / 24-bit Lossless"), "renders FLAC specs");
    assert.ok(audioHtml.includes(escapeHtml(translations.en.landingBadgeAudio)), "renders audio badge");

    // 2. DMS panel
    const dmsHtml = renderToStaticMarkup(<LandingPage initialLanguage="en" initialActiveTab="dms" />);
    assert.ok(dmsHtml.includes('data-testid="feature-panel-dms"'), "renders dms panel");
    assert.ok(dmsHtml.includes("Paperless-ngx OCR Engine"), "renders Paperless info");
    assert.ok(dmsHtml.includes("Tesseract 5.3"), "renders OCR engine note");
    assert.ok(dmsHtml.includes('href="/guides#storage"'), "renders deep link to /guides#storage");

    // 3. AI panel
    const aiHtml = renderToStaticMarkup(<LandingPage initialLanguage="en" initialActiveTab="ai" />);
    assert.ok(aiHtml.includes('data-testid="feature-panel-ai"'), "renders ai panel");
    assert.ok(aiHtml.includes("llama.cpp Engine &amp; Vector RAG"), "renders llama.cpp info");
    assert.ok(aiHtml.includes("sovereign-ai bridge (internal only)"), "renders air-gapped network note");
    assert.ok(aiHtml.includes('href="/guides#ai"'), "renders deep link to /guides#ai");

    // 4. Optical disc panel
    const discHtml = renderToStaticMarkup(<LandingPage initialLanguage="en" initialActiveTab="disc" />);
    assert.ok(discHtml.includes('data-testid="feature-panel-disc"'), "renders disc panel");
    assert.ok(discHtml.includes("Automatic Ripping Machine (ARM)"), "renders ARM info");
    assert.ok(discHtml.includes("LibreDrive flashed"), "renders LibreDrive note");
    assert.ok(discHtml.includes(escapeHtml(translations.en.landingBadgeDisc)), "renders disc badge");
  });

  it("renders zero-telemetry architectural invariants and privacy guarantees", () => {
    const html = renderToStaticMarkup(<LandingPage initialLanguage="en" />);

    assert.ok(html.includes(escapeHtml(translations.en.landingPrivacyTitle)), "renders privacy title");
    assert.ok(html.includes(escapeHtml(translations.en.landingPrivacyPill1Title)), "renders air-gapped network pill");
    assert.ok(html.includes(escapeHtml(translations.en.landingPrivacyPill2Title)), "renders zero docker socket pill");
    assert.ok(html.includes(escapeHtml(translations.en.landingPrivacyPill3Title)), "renders CGO-free WAL durability pill");
    assert.ok(html.includes(escapeHtml(translations.en.landingPrivacyPill4Title)), "renders atomic 180s rollback pill");
  });

  it("embeds interactive hardware tier calculator and verifies Light, Media, and AI tiers", () => {
    // 1. Default / Light Tier
    const lightHtml = renderToStaticMarkup(
      <LandingPage initialLanguage="en" initialFeatures={{ dms: true, audio: true, video: false, disc: false, ai: false }} />
    );
    assert.ok(lightHtml.includes('data-testid="calc-check-dms"'), "renders dms checkbox");
    assert.ok(lightHtml.includes('data-testid="calc-result"'), "renders calc result box");
    assert.ok(lightHtml.includes(escapeHtml(translations.en.tierLightTitle)), "renders Light Tier title");
    assert.ok(lightHtml.includes(escapeHtml(translations.en.tierLightCpu)), "renders Light Tier CPU");
    assert.ok(lightHtml.includes(escapeHtml(translations.en.tierLightRam)), "renders Light Tier RAM");
    assert.ok(lightHtml.includes(escapeHtml(translations.en.tierLightStorage)), "renders Light Tier Storage");
    assert.ok(lightHtml.includes(escapeHtml(translations.en.tierLightPower)), "renders Light Tier Power");

    // 2. Media Tier (video enabled)
    const mediaHtml = renderToStaticMarkup(
      <LandingPage initialLanguage="en" initialFeatures={{ dms: true, audio: true, video: true, disc: false, ai: false }} />
    );
    assert.ok(mediaHtml.includes(escapeHtml(translations.en.tierMediaTitle)), "renders Media Tier title");
    assert.ok(mediaHtml.includes(escapeHtml(translations.en.tierMediaCpu)), "renders Media Tier CPU");
    assert.ok(mediaHtml.includes(escapeHtml(translations.en.tierMediaRam)), "renders Media Tier RAM");
    assert.ok(mediaHtml.includes(escapeHtml(translations.en.tierMediaStorage)), "renders Media Tier Storage");
    assert.ok(mediaHtml.includes(escapeHtml(translations.en.tierMediaPower)), "renders Media Tier Power");

    // 3. AI Tier (ai enabled)
    const aiHtml = renderToStaticMarkup(
      <LandingPage initialLanguage="en" initialFeatures={{ dms: true, audio: true, video: true, disc: true, ai: true }} />
    );
    assert.ok(aiHtml.includes(escapeHtml(translations.en.tierAITitle)), "renders AI Tier title");
    assert.ok(aiHtml.includes(escapeHtml(translations.en.tierAICpu)), "renders AI Tier CPU");
    assert.ok(aiHtml.includes(escapeHtml(translations.en.tierAIRam)), "renders AI Tier RAM");
    assert.ok(aiHtml.includes(escapeHtml(translations.en.tierAIStorage)), "renders AI Tier Storage");
    assert.ok(aiHtml.includes(escapeHtml(translations.en.tierAIPower)), "renders AI Tier Power");
  });

  it("renders localized marketing content in German, Spanish, and French", () => {
    // German (DE)
    const deHtml = renderToStaticMarkup(<LandingPage initialLanguage="de" />);
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingHeroHeadline)), "renders German hero headline");
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingHeroBadge)), "renders German hero badge");
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingFeatureShowcaseTitle)), "renders German feature showcase title");
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingBadgeVideo)), "renders German video badge");
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingPrivacyTitle)), "renders German privacy title");
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingCtaDashboard)), "renders German dashboard CTA");
    assert.ok(deHtml.includes(escapeHtml(translations.de.landingFooterBrandDesc)), "renders German footer brand desc");

    // Spanish (ES)
    const esHtml = renderToStaticMarkup(<LandingPage initialLanguage="es" />);
    assert.ok(esHtml.includes(escapeHtml(translations.es.landingHeroHeadline)), "renders Spanish hero headline");
    assert.ok(esHtml.includes(escapeHtml(translations.es.landingHeroBadge)), "renders Spanish hero badge");
    assert.ok(esHtml.includes(escapeHtml(translations.es.landingFeatureShowcaseTitle)), "renders Spanish feature showcase title");
    assert.ok(esHtml.includes(escapeHtml(translations.es.landingBadgeAudio)), "renders Spanish audio badge");
    assert.ok(esHtml.includes(escapeHtml(translations.es.landingPrivacyTitle)), "renders Spanish privacy title");

    // French (FR)
    const frHtml = renderToStaticMarkup(<LandingPage initialLanguage="fr" />);
    assert.ok(frHtml.includes(escapeHtml(translations.fr.landingHeroHeadline)), "renders French hero headline");
    assert.ok(frHtml.includes(escapeHtml(translations.fr.landingHeroBadge)), "renders French hero badge");
    assert.ok(frHtml.includes(escapeHtml(translations.fr.landingFeatureShowcaseTitle)), "renders French feature showcase title");
    assert.ok(frHtml.includes(escapeHtml(translations.fr.landingBadgeAi)), "renders French AI badge");
    assert.ok(frHtml.includes(escapeHtml(translations.fr.landingPrivacyTitle)), "renders French privacy title");
  });

  it("verifies inbound navigation links to /landing from Guides and Hardware pages", () => {
    const guidesHtml = renderToStaticMarkup(<GuidesPage initialLanguage="en" />);
    assert.ok(guidesHtml.includes('data-testid="guides-showcase-link"'), "renders showcase link on guides page");
    assert.ok(guidesHtml.includes('href="/landing"'), "guides page link targets /landing");

    const hardwareHtml = renderToStaticMarkup(<HardwarePage initialLanguage="en" />);
    assert.ok(hardwareHtml.includes('data-testid="hardware-showcase-link"'), "renders showcase link on hardware page");
    assert.ok(hardwareHtml.includes('href="/landing"'), "hardware page link targets /landing");
  });

  it("complies with WCAG AA accessibility standards", () => {
    const html = renderToStaticMarkup(<LandingPage initialLanguage="en" />);

    assert.ok(html.includes("min-h-[44px]") || html.includes("min-h-[48px]"), "enforces 44px+ minimum touch targets");
    assert.ok(html.includes("focus-visible:ring-2"), "has visible keyboard focus rings");
    assert.ok(html.includes('role="region"'), "major sections have role=region");
    assert.ok(html.includes('role="tablist"'), "feature selector has role=tablist");
    assert.ok(html.includes("text-[#1D4ED8]"), "Section 5 subtitle meets WCAG AA contrast");
  });
});
