import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import HardwarePage, { calculateHardwareTier, HardwareCalculatorState } from "./page";
import { translations } from "@/lib/i18n";

describe("Hardware Tier Calculator & Profiles (Story 7.1)", () => {
  it("calculateHardwareTier correctly classifies feature sets into Light, Media, and AI Tiers", () => {
    // 1. Light Tier: DMS and/or Audio only
    assert.equal(
      calculateHardwareTier({ dms: true, audio: true, video: false, disc: false, ai: false }),
      "light"
    );
    assert.equal(
      calculateHardwareTier({ dms: true, audio: false, video: false, disc: false, ai: false }),
      "light"
    );
    assert.equal(
      calculateHardwareTier({ dms: false, audio: true, video: false, disc: false, ai: false }),
      "light"
    );

    // 2. Media Tier: Video streaming and/or optical disc ripping enabled, without AI
    assert.equal(
      calculateHardwareTier({ dms: true, audio: true, video: true, disc: false, ai: false }),
      "media"
    );
    assert.equal(
      calculateHardwareTier({ dms: false, audio: false, video: false, disc: true, ai: false }),
      "media"
    );
    assert.equal(
      calculateHardwareTier({ dms: true, audio: true, video: true, disc: true, ai: false }),
      "media"
    );

    // 3. AI & Power Tier: AI enabled (highest tier requirement)
    assert.equal(
      calculateHardwareTier({ dms: true, audio: true, video: true, disc: true, ai: true }),
      "ai"
    );
    assert.equal(
      calculateHardwareTier({ dms: false, audio: false, video: false, disc: false, ai: true }),
      "ai"
    );
    assert.equal(
      calculateHardwareTier({ dms: true, audio: false, video: false, disc: false, ai: true }),
      "ai"
    );
    // Null and undefined safety guards
    assert.equal(calculateHardwareTier(null), "light");
    assert.equal(calculateHardwareTier(undefined), "light");
  });

  it("renders 3 distinct hardware tier profiles with CPU, RAM, storage, and idle power specs", () => {
    const html = renderToStaticMarkup(<HardwarePage initialLanguage="en" />);

    // Check all 3 tier cards are rendered
    assert.ok(html.includes('data-testid="tier-card-light"'), "renders light tier card");
    assert.ok(html.includes('data-testid="tier-card-media"'), "renders media tier card");
    assert.ok(html.includes('data-testid="tier-card-ai"'), "renders ai tier card");

    // Light Tier specs
    assert.ok(html.includes("Light Tier"), "renders Light Tier title");
    assert.ok(html.includes("Dual-core x86 or 64-bit ARM"), "renders Light CPU spec");
    assert.ok(html.includes("4 GB DDR4 / DDR5"), "renders Light RAM spec");
    assert.ok(html.includes("5W – 10W idle"), "renders Light idle power spec");

    // Media Tier specs
    assert.ok(html.includes("Media Tier"), "renders Media Tier title");
    assert.ok(html.includes("Intel QuickSync iGPU"), "renders Media QuickSync spec");
    assert.ok(html.includes("8 GB DDR4 / DDR5"), "renders Media RAM spec");
    assert.ok(html.includes("10W – 25W idle"), "renders Media idle power spec");

    // AI Tier specs
    assert.ok(html.includes("AI &amp; Power Tier"), "renders AI Tier title");
    assert.ok(html.includes("6+ core modern x86 CPU"), "renders AI CPU spec");
    assert.ok(html.includes("16 GB – 32 GB RAM + 8–16 GB VRAM GPU"), "renders AI RAM+VRAM spec");
    assert.ok(html.includes("25W – 45W idle"), "renders AI idle power spec");
  });

  it("renders interactive feature calculator with five options and dynamic target badges", () => {
    const html = renderToStaticMarkup(<HardwarePage initialLanguage="en" />);

    assert.ok(html.includes("Interactive Hardware Tier Calculator"), "renders calculator heading");
    assert.ok(html.includes('data-testid="calc-check-dms"'), "renders DMS checkbox");
    assert.ok(html.includes('data-testid="calc-check-audio"'), "renders Audio checkbox");
    assert.ok(html.includes('data-testid="calc-check-video"'), "renders Video checkbox");
    assert.ok(html.includes('data-testid="calc-check-disc"'), "renders Disc checkbox");
    assert.ok(html.includes('data-testid="calc-check-ai"'), "renders AI checkbox");

    // Target recommendations inside result box
    assert.ok(html.includes('data-testid="calc-result"'), "renders calculator result recommendation box");
    assert.ok(html.includes('data-testid="calc-target-cpu"'), "renders recommended CPU spec badge");
    assert.ok(html.includes('data-testid="calc-target-ram"'), "renders recommended RAM spec badge");
    assert.ok(html.includes('data-testid="calc-target-storage"'), "renders recommended Storage spec badge");
    assert.ok(html.includes('data-testid="calc-target-power"'), "renders recommended Power spec badge");
  });

  it("renders turnkey pre-assembled hardware roadmap and reservation buttons", () => {
    const html = renderToStaticMarkup(<HardwarePage initialLanguage="en" />);

    assert.ok(html.includes("Turnkey Pre-Assembled Sovereign Appliances"), "renders turnkey section title");
    assert.ok(html.includes("Sovereign Mini"), "renders Phase 1 Sovereign Mini");
    assert.ok(html.includes("Sovereign Home"), "renders Phase 2 Sovereign Home");
    assert.ok(html.includes("Sovereign Pro Studio"), "renders Phase 3 Sovereign Pro Studio");

    assert.ok(html.includes('data-testid="reserve-btn-mini"'), "renders mini reserve button");
    assert.ok(html.includes('data-testid="reserve-btn-home"'), "renders home reserve button");
    assert.ok(html.includes('data-testid="reserve-btn-studio"'), "renders studio reserve button");
  });

  it("renders preorder reservation dialog with model indicator, email input, and submit button", () => {
    const html = renderToStaticMarkup(
      <HardwarePage
        initialLanguage="en"
        initialPreorderModalOpen={true}
        initialSelectedModel="home"
      />
    );

    assert.ok(html.includes('role="dialog"'), "renders accessible modal dialog");
    assert.ok(html.includes('aria-modal="true"'), "modal dialog is aria-modal=true");
    assert.ok(html.includes('data-testid="modal-selected-model"'), "renders selected model badge");
    assert.ok(html.includes("Sovereign Home (Phase 2)"), "displays Sovereign Home model name");
    assert.ok(html.includes('data-testid="preorder-email-input"'), "renders email input");
    assert.ok(html.includes('data-testid="preorder-submit-btn"'), "renders submit reservation button");
  });

  it("renders reservation success screen with token and copy button when confirmed", () => {
    const html = renderToStaticMarkup(
      <HardwarePage
        initialLanguage="en"
        initialPreorderModalOpen={true}
        initialSelectedModel="mini"
        initialReservationToken="SOV-HW-TEST42"
      />
    );

    assert.ok(html.includes('data-testid="reservation-success"'), "renders reservation success view");
    assert.ok(html.includes("SOV-HW-TEST42"), "displays generated hardware identity token");
    assert.ok(html.includes('data-testid="copy-token-btn"'), "renders copy token button");
  });

  it("renders localized hardware guidance in German, Spanish, and French", () => {
    const escape = (s: string) => s.replace(/&/g, "&amp;");

    // German (DE)
    const deHtml = renderToStaticMarkup(<HardwarePage initialLanguage="de" />);
    assert.ok(deHtml.includes(escape(translations.de.hardwareTitle)), "renders German title");
    assert.ok(deHtml.includes(escape(translations.de.tierLightTitle)), "renders German Light Tier title");
    assert.ok(deHtml.includes(escape(translations.de.tierMediaTitle)), "renders German Media Tier title");
    assert.ok(deHtml.includes(escape(translations.de.tierAITitle)), "renders German AI Tier title");
    assert.ok(deHtml.includes(escape(translations.de.calcTitle)), "renders German calculator title");

    // Spanish (ES)
    const esHtml = renderToStaticMarkup(<HardwarePage initialLanguage="es" />);
    assert.ok(esHtml.includes(escape(translations.es.hardwareTitle)), "renders Spanish title");
    assert.ok(esHtml.includes(escape(translations.es.tierLightTitle)), "renders Spanish Light Tier title");
    assert.ok(esHtml.includes(escape(translations.es.calcTitle)), "renders Spanish calculator title");

    // French (FR)
    const frHtml = renderToStaticMarkup(<HardwarePage initialLanguage="fr" />);
    assert.ok(frHtml.includes(escape(translations.fr.hardwareTitle)), "renders French title");
    assert.ok(frHtml.includes(escape(translations.fr.tierLightTitle)), "renders French Light Tier title");
    assert.ok(frHtml.includes(escape(translations.fr.calcTitle)), "renders French calculator title");
  });

  it("complies with WCAG AA accessibility with 44px minimum touch targets and focus rings", () => {
    const html = renderToStaticMarkup(<HardwarePage initialLanguage="en" />);

    assert.ok(html.includes("min-h-[44px]"), "enforces 44px minimum touch targets on buttons and inputs");
    assert.ok(html.includes("focus-visible:ring-2"), "has visible keyboard focus rings");
    assert.ok(html.includes('role="group"'), "feature selection has role=group");
    assert.ok(html.includes('role="region"'), "major sections have role=region");
  });
});
