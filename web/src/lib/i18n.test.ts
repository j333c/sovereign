import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  detectBrowserLanguage,
  getInitialLanguage,
  persistLanguage,
  normalizeLanguageTag,
  translations,
  Language,
} from "./i18n";

describe("i18n Language Detection, Parity & Persistence (Story 6.5)", () => {
  it("detects language tags matching German, Spanish, French, and English", () => {
    assert.equal(detectBrowserLanguage(["de-DE"]), "de");
    assert.equal(detectBrowserLanguage(["de"]), "de");
    assert.equal(detectBrowserLanguage(["es-ES"]), "es");
    assert.equal(detectBrowserLanguage(["es-MX"]), "es");
    assert.equal(detectBrowserLanguage(["es"]), "es");
    assert.equal(detectBrowserLanguage(["fr-FR"]), "fr");
    assert.equal(detectBrowserLanguage(["fr-CA"]), "fr");
    assert.equal(detectBrowserLanguage(["fr"]), "fr");
    assert.equal(detectBrowserLanguage(["en-US"]), "en");
    assert.equal(detectBrowserLanguage(["en-GB"]), "en");
    assert.equal(detectBrowserLanguage(["en"]), "en");
  });

  it("prioritizes the first supported match in multi-language preference arrays", () => {
    assert.equal(detectBrowserLanguage(["ja-JP", "de-DE", "en"]), "de");
    assert.equal(detectBrowserLanguage(["zh-CN", "fr-FR", "es"]), "fr");
    assert.equal(detectBrowserLanguage(["pt-BR", "es-ES", "en"]), "es");
  });

  it("defaults to English (en) when given unsupported language tags or empty array", () => {
    assert.equal(detectBrowserLanguage(["ja-JP"]), "en");
    assert.equal(detectBrowserLanguage(["zh-CN"]), "en");
    assert.equal(detectBrowserLanguage(["ru-RU"]), "en");
    assert.equal(detectBrowserLanguage([]), "en");
    assert.equal(detectBrowserLanguage([""]), "en");
  });

  it("avoids false positive prefix matching on language tags like Estonian or Greek", () => {
    // 'et' / 'est' (Estonian) should NOT match 'es'
    assert.equal(detectBrowserLanguage(["et-EE"]), "en");
    assert.equal(detectBrowserLanguage(["et"]), "en");
    // 'el' (Greek) should NOT match 'en' or 'es'
    assert.equal(detectBrowserLanguage(["el-GR"]), "en");
    // 'eo' (Esperanto) should NOT match 'es'
    assert.equal(detectBrowserLanguage(["eo"]), "en");
    // 'fry' (Western Frisian) should NOT match 'fr'
    assert.equal(detectBrowserLanguage(["fry"]), "en");
  });

  it("supports Accept-Language header string with quality factors", () => {
    assert.equal(detectBrowserLanguage("de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7"), "de");
    assert.equal(detectBrowserLanguage("fr-FR,fr;q=0.9,es;q=0.8"), "fr");
    assert.equal(detectBrowserLanguage("es-ES,es;q=0.9,en;q=0.8"), "es");
    assert.equal(detectBrowserLanguage("ja-JP,zh;q=0.9,en;q=0.5"), "en");
  });

  it("normalizes regional language tags in normalizeLanguageTag and getInitialLanguage", () => {
    assert.equal(normalizeLanguageTag("de-DE"), "de");
    assert.equal(normalizeLanguageTag("es_MX"), "es");
    assert.equal(normalizeLanguageTag("fr-CA"), "fr");
    assert.equal(normalizeLanguageTag("en-US"), "en");
    assert.equal(normalizeLanguageTag("unsupported"), null);

    assert.equal(getInitialLanguage("de-DE"), "de");
    assert.equal(getInitialLanguage("es-MX"), "es");
    assert.equal(getInitialLanguage("fr-FR"), "fr");
    assert.equal(getInitialLanguage("en-GB"), "en");
    assert.equal(getInitialLanguage("invalid"), "en");
  });

  it("verifies 100% key parity between en and all other locales (de, es, fr)", () => {
    const enKeys = Object.keys(translations.en) as (keyof typeof translations.en)[];
    const locales: Language[] = ["de", "es", "fr"];

    for (const locale of locales) {
      const locDict = translations[locale];
      const locKeys = new Set(Object.keys(locDict));

      for (const key of enKeys) {
        assert.ok(
          locKeys.has(key),
          `Missing key "${key}" in locale "${locale}" dictionary`
        );
        const val = locDict[key];
        assert.ok(
          typeof val === "string" && val.trim().length > 0,
          `Empty translation for key "${key}" in locale "${locale}"`
        );
      }
    }
  });

  it("persistLanguage updates localStorage, cookies, and document.documentElement.lang", () => {
    const storageMock: Record<string, string> = {};
    const docMock = {
      cookie: "",
      documentElement: { lang: "en" },
    };

    const originalWindow = (globalThis as unknown as { window?: unknown }).window;
    const originalDocument = (globalThis as unknown as { document?: unknown }).document;
    const originalLocalStorage = (globalThis as unknown as { localStorage?: unknown }).localStorage;

    try {
      (globalThis as unknown as { window: unknown }).window = {};
      (globalThis as unknown as { document: unknown }).document = docMock;
      (globalThis as unknown as { localStorage: unknown }).localStorage = {
        setItem: (k: string, v: string) => {
          storageMock[k] = v;
        },
        getItem: (k: string) => storageMock[k] || null,
      };

      persistLanguage("de");
      assert.equal(storageMock["sovereign_lang"], "de");
      assert.ok(docMock.cookie.includes("sovereign_lang=de"));
      assert.equal(docMock.documentElement.lang, "de");

      persistLanguage("fr");
      assert.equal(storageMock["sovereign_lang"], "fr");
      assert.ok(docMock.cookie.includes("sovereign_lang=fr"));
      assert.equal(docMock.documentElement.lang, "fr");
    } finally {
      if (originalWindow !== undefined) {
        (globalThis as unknown as { window: unknown }).window = originalWindow;
      } else {
        delete (globalThis as unknown as { window?: unknown }).window;
      }
      if (originalDocument !== undefined) {
        (globalThis as unknown as { document: unknown }).document = originalDocument;
      } else {
        delete (globalThis as unknown as { document?: unknown }).document;
      }
      if (originalLocalStorage !== undefined) {
        (globalThis as unknown as { localStorage: unknown }).localStorage = originalLocalStorage;
      } else {
        delete (globalThis as unknown as { localStorage?: unknown }).localStorage;
      }
    }
  });

  it("getInitialLanguage tests full priority cascade: storage -> cookie -> navigator -> default en", () => {
    const storageMock: Record<string, string> = {};
    const docMock = { cookie: "" };
    const navMock = { languages: ["de-DE"] };

    const originalWindow = (globalThis as unknown as { window?: unknown }).window;
    const originalDocument = (globalThis as unknown as { document?: unknown }).document;
    const originalLocalStorage = (globalThis as unknown as { localStorage?: unknown }).localStorage;
    const originalNavigatorDesc = Object.getOwnPropertyDescriptor(globalThis, "navigator");

    try {
      (globalThis as unknown as { window: unknown }).window = {};
      (globalThis as unknown as { document: unknown }).document = docMock;
      (globalThis as unknown as { localStorage: unknown }).localStorage = {
        getItem: (k: string) => storageMock[k] || null,
      };
      Object.defineProperty(globalThis, "navigator", {
        value: navMock,
        configurable: true,
        writable: true,
      });

      // 1. When localStorage has a saved language, it wins
      storageMock["sovereign_lang"] = "es";
      docMock.cookie = "sovereign_lang=fr";
      navMock.languages = ["de-DE"];
      assert.equal(getInitialLanguage(), "es");

      // 2. When localStorage is empty, cookie is checked
      delete storageMock["sovereign_lang"];
      assert.equal(getInitialLanguage(), "fr");

      // 3. When cookie is empty, browser detection is checked
      docMock.cookie = "";
      assert.equal(getInitialLanguage(), "de");

      // 4. When browser returns unsupported, fallback to en
      navMock.languages = ["ja-JP"];
      assert.equal(getInitialLanguage(), "en");
    } finally {
      if (originalWindow !== undefined) {
        (globalThis as unknown as { window: unknown }).window = originalWindow;
      } else {
        delete (globalThis as unknown as { window?: unknown }).window;
      }
      if (originalDocument !== undefined) {
        (globalThis as unknown as { document: unknown }).document = originalDocument;
      } else {
        delete (globalThis as unknown as { document?: unknown }).document;
      }
      if (originalLocalStorage !== undefined) {
        (globalThis as unknown as { localStorage: unknown }).localStorage = originalLocalStorage;
      } else {
        delete (globalThis as unknown as { localStorage?: unknown }).localStorage;
      }
      if (originalNavigatorDesc) {
        Object.defineProperty(globalThis, "navigator", originalNavigatorDesc);
      } else {
        delete (globalThis as unknown as { navigator?: unknown }).navigator;
      }
    }
  });
});
