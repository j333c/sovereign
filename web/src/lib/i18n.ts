import { Language, DashboardTranslations, Translations } from "./i18n/types";
import { en } from "./i18n/en";
import { de } from "./i18n/de";
import { es } from "./i18n/es";
import { fr } from "./i18n/fr";

export type { Language, DashboardTranslations, Translations };

export const translations: Record<Language, DashboardTranslations> = {
  en,
  de,
  es,
  fr,
};

/**
 * Automatically detects the preferred language based on browser navigator settings,
 * Accept-Language header strings, or provided language tags, prioritizing German (DE),
 * Spanish (ES), French (FR), and defaulting to English (EN).
 */
export function detectBrowserLanguage(preferredLanguages?: readonly string[] | string): Language {
  const candidates: string[] = [];

  if (typeof preferredLanguages === "string") {
    // Parse Accept-Language header (e.g. "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")
    const parts = preferredLanguages.split(",").map((p) => {
      const [lang, qPart] = p.trim().split(";");
      const q = qPart && qPart.startsWith("q=") ? parseFloat(qPart.slice(2)) : 1.0;
      return { lang: lang.trim(), q: isNaN(q) ? 1.0 : q };
    });
    parts.sort((a, b) => b.q - a.q);
    candidates.push(...parts.map((p) => p.lang));
  } else if (Array.isArray(preferredLanguages)) {
    candidates.push(...preferredLanguages);
  } else if (typeof window !== "undefined" && typeof navigator !== "undefined") {
    if (navigator.languages && navigator.languages.length > 0) {
      candidates.push(...navigator.languages);
    } else if (navigator.language) {
      candidates.push(navigator.language);
    }
  }

  for (const cand of candidates) {
    if (!cand || typeof cand !== "string") continue;
    const clean = cand.toLowerCase().trim();
    const primary = clean.split(/[-_]/)[0];
    if (primary === "de") return "de";
    if (primary === "es") return "es";
    if (primary === "fr") return "fr";
    if (primary === "en") return "en";
  }

  return "en";
}

/**
 * Persists the selected language preference to localStorage, cookies, and document element.
 */
export function persistLanguage(lang: Language): void {
  if (typeof window !== "undefined") {
    try {
      localStorage.setItem("sovereign_lang", lang);
    } catch {
      // ignore
    }
    try {
      document.cookie = `sovereign_lang=${lang}; path=/; max-age=31536000; SameSite=Lax`;
    } catch {
      // ignore
    }
    try {
      if (typeof document !== "undefined" && document.documentElement) {
        document.documentElement.lang = lang;
      }
    } catch {
      // ignore
    }
  }
}

/**
 * Helper to normalize and match a raw language code into a supported Language.
 */
export function normalizeLanguageTag(val?: string | null): Language | null {
  if (!val || typeof val !== "string") return null;
  const primary = val.toLowerCase().trim().split(/[-_]/)[0];
  if (primary === "en" || primary === "de" || primary === "es" || primary === "fr") {
    return primary as Language;
  }
  return null;
}

/**
 * Retrieves the active language preference:
 * 1. Explicit argument / override (normalized)
 * 2. Persisted value in localStorage
 * 3. Persisted value in cookies
 * 4. Browser autodetection via navigator.languages / navigator.language (in browser context)
 * 5. Default fallback to "en"
 */
export function getInitialLanguage(savedLanguage?: string | null): Language {
  const explicit = normalizeLanguageTag(savedLanguage);
  if (explicit) {
    return explicit;
  }
  if (typeof window !== "undefined") {
    try {
      const stored = localStorage.getItem("sovereign_lang");
      const matched = normalizeLanguageTag(stored);
      if (matched) return matched;
    } catch {
      // ignore
    }
    try {
      if (typeof document !== "undefined" && document.cookie) {
        const match = document.cookie.match(/(?:^|;\s*)sovereign_lang=([^;]+)/);
        if (match && match[1]) {
          const matched = normalizeLanguageTag(match[1]);
          if (matched) return matched;
        }
      }
    } catch {
      // ignore
    }
    return detectBrowserLanguage();
  }
  return "en";
}


