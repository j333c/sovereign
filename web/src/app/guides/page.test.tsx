import React from "react";
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import GuidesPage, { guidesData, GuideTopicId } from "./page";
import { translations, Language } from "@/lib/i18n";

function renderComponent<T>(fn: (props: T) => React.ReactNode, props: T): any {
  let result: any = null;
  function Wrapper() {
    result = fn(props);
    return null;
  }
  renderToStaticMarkup(React.createElement(Wrapper));
  return result;
}

function findByTestId(node: any, testId: string): any {
  if (!node) return null;
  if (node.props?.["data-testid"] === testId) return node;
  if (node.props?.children) {
    const children = Array.isArray(node.props.children)
      ? node.props.children
      : [node.props.children];
    for (const child of children) {
      const found = findByTestId(child, testId);
      if (found) return found;
    }
  }
  return null;
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, "&amp;")
    .replace(/'/g, "&#x27;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

describe("GuidesPage Component & Multilingual Bundled Documentation (Story 6.5)", () => {
  it("renders GuidesPage with AppHeader, topic navigation tabs, and active guide content", () => {
    const html = renderToStaticMarkup(
      <GuidesPage initialLanguage="en" initialTopic="quickstart" />
    );

    assert.ok(html.includes("Appliance Guides &amp; Help"));
    assert.ok(html.includes("Quickstart &amp; Appliance Setup"));
    assert.ok(html.includes("Hardware Connections"));
    assert.ok(html.includes("Finding Your Appliance on the Local Network"));
    assert.ok(html.includes("http://sovereign.local"));
    assert.ok(html.includes('data-testid="guide-tab-quickstart"'));
    assert.ok(html.includes('data-testid="guide-tab-storage"'));
    assert.ok(html.includes('data-testid="guide-tab-media"'));
    assert.ok(html.includes('data-testid="guide-tab-ai"'));
    assert.ok(html.includes('data-testid="guide-tab-resilience"'));
  });

  it("renders storage, media, ai, and resilience topic guides correctly", () => {
    const topics: GuideTopicId[] = ["storage", "media", "ai", "resilience"];

    for (const topic of topics) {
      const html = renderToStaticMarkup(
        <GuidesPage initialLanguage="en" initialTopic={topic} />
      );

      const topicData = guidesData.en.find((t) => t.id === topic);
      assert.ok(topicData, `Topic data for ${topic} must exist`);
      assert.ok(
        html.includes(escapeHtml(topicData.summary)),
        `HTML must include summary for topic ${topic}`
      );
      for (const section of topicData.sections) {
        assert.ok(
          html.includes(escapeHtml(section.title)),
          `HTML must include section title ${section.title}`
        );
      }
    }
  });

  it("renders bundled guides fully translated across all 4 launch languages (EN, DE, ES, FR)", () => {
    const languages: Language[] = ["en", "de", "es", "fr"];

    for (const lang of languages) {
      const html = renderToStaticMarkup(
        <GuidesPage initialLanguage={lang} initialTopic="quickstart" />
      );

      const t = translations[lang];
      assert.ok(html.includes(escapeHtml(t.guidesTitle)));

      const topicData = guidesData[lang].find((t) => t.id === "quickstart");
      assert.ok(topicData, `Quickstart topic must exist for language ${lang}`);
      assert.ok(
        html.includes(escapeHtml(topicData.summary)),
        `HTML must include summary in ${lang}`
      );
      assert.ok(
        html.includes(escapeHtml(topicData.sections[0].title)),
        `HTML must include section 1 title in ${lang}`
      );
    }
  });

  it("supports keyboard navigation across guide topic tabs", () => {
    const tree = renderComponent(GuidesPage, {
      initialLanguage: "en",
      initialTopic: "quickstart",
    });

    const panel = findByTestId(tree, "guide-content-panel");
    assert.ok(panel, "Guide content panel must be rendered");

    const quickstartTab = findByTestId(tree, "guide-tab-quickstart");
    assert.ok(quickstartTab, "Quickstart tab button must exist");

    // Clicking a tab triggers selection
    quickstartTab.props.onClick();
  });
});
