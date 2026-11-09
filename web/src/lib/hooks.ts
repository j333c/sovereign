import { useState, useEffect, RefObject } from "react";
import { Language, translations, getInitialLanguage, persistLanguage, Translations } from "./i18n";

export interface UseLanguageResult {
  lang: Language;
  t: Translations;
  handleLanguageChange: (newLang: Language) => void;
}

export function useLanguage(initialLanguage?: Language): UseLanguageResult {
  const [lang, setLang] = useState<Language>(() => getInitialLanguage(initialLanguage));
  const t = translations[lang];

  const handleLanguageChange = (newLang: Language) => {
    setLang(newLang);
    persistLanguage(newLang);
  };

  return { lang, t, handleLanguageChange };
}

export interface UseModalFocusTrapOptions {
  disableEscape?: boolean;
  focusSelector?: string;
}

export function useModalFocusTrap(
  containerRef: RefObject<HTMLElement | null>,
  isOpen: boolean,
  onClose: () => void,
  options?: UseModalFocusTrapOptions
) {
  const disableEscape = options?.disableEscape ?? false;
  const focusSelector =
    options?.focusSelector ??
    'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  useEffect(() => {
    if (!isOpen) return;

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !disableEscape) {
        onClose();
        return;
      }

      if (event.key === "Tab") {
        const container = containerRef.current;
        if (!container) return;
        const focusables = container.querySelectorAll<HTMLElement>(focusSelector);
        if (!focusables || focusables.length === 0) return;

        const first = focusables[0];
        const last = focusables[focusables.length - 1];

        if (event.shiftKey) {
          if (document.activeElement === first) {
            event.preventDefault();
            last.focus();
          }
        } else {
          if (document.activeElement === last) {
            event.preventDefault();
            first.focus();
          }
        }
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [containerRef, isOpen, onClose, disableEscape, focusSelector]);
}
