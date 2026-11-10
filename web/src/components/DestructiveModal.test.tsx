import React from "react";
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { DestructiveModal, isValidConfirmationToken } from "./DestructiveModal";
import { translations, Language } from "@/lib/i18n";

describe("DestructiveModal Component (Story 6.4)", () => {
  it("renders null when isOpen is false", () => {
    const html = renderToStaticMarkup(
      <DestructiveModal
        isOpen={false}
        actionType="reboot"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );
    assert.equal(html, "");
  });

  it("renders accessible dialog attributes and reboot reassurance copy", () => {
    const html = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="reboot"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes('role="dialog"'));
    assert.ok(html.includes('aria-modal="true"'));
    assert.ok(html.includes('aria-labelledby="destructive-modal-title"'));
    assert.ok(html.includes('aria-describedby="destructive-modal-desc"'));
    assert.ok(html.includes("Reboot Appliance"));
    assert.ok(html.includes("60 seconds"));
    assert.ok(html.includes("data-testid=\"destructive-cancel-button\""));
    assert.ok(html.includes("data-testid=\"destructive-confirm-button\""));
  });

  it("renders factory-reset mode with RESET validation input and disabled confirm button initially", () => {
    const html = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes("Factory Data Reset"));
    assert.ok(html.includes("Type RESET to confirm"));
    assert.ok(html.includes('data-testid="factory-reset-input"'));
    assert.ok(html.includes("disabled"));
    assert.ok(html.includes("Perform Factory Reset"));
  });

  it("renders service deactivation mode with volume preservation reassurance note", () => {
    const html = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="deactivate"
        serviceName="Jellyfin"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes("Deactivate Service"));
    assert.ok(html.includes("Jellyfin"));
    assert.ok(html.includes("Your media, files, and documents are preserved and will not be touched."));
  });

  it("renders localized copy across all 4 languages (EN, DE, ES, FR)", () => {
    const languages: Language[] = ["en", "de", "es", "fr"];

    const escapeHtml = (str: string) => str.replace(/'/g, "&#x27;");

    for (const lang of languages) {
      const t = translations[lang];
      const htmlReboot = renderToStaticMarkup(
        <DestructiveModal
          isOpen={true}
          actionType="reboot"
          onConfirm={() => {}}
          onClose={() => {}}
          currentLanguage={lang}
        />
      );
      assert.ok(htmlReboot.includes(escapeHtml(t.rebootTitle)));
      assert.ok(htmlReboot.includes(escapeHtml(t.rebootConfirmButton)));

      const htmlReset = renderToStaticMarkup(
        <DestructiveModal
          isOpen={true}
          actionType="factory-reset"
          onConfirm={() => {}}
          onClose={() => {}}
          currentLanguage={lang}
        />
      );
      assert.ok(htmlReset.includes(escapeHtml(t.factoryResetTitle)));
      assert.ok(htmlReset.includes(escapeHtml(t.factoryResetConfirmButton)));
      assert.ok(htmlReset.includes(escapeHtml(t.typeResetToConfirm)));

      const htmlDeact = renderToStaticMarkup(
        <DestructiveModal
          isOpen={true}
          actionType="deactivate"
          serviceName="Navidrome"
          onConfirm={() => {}}
          onClose={() => {}}
          currentLanguage={lang}
        />
      );
      assert.ok(htmlDeact.includes(escapeHtml(t.deactivateTitle)));
      assert.ok(htmlDeact.includes(escapeHtml(t.deactivateConfirmButton)));
      assert.ok(htmlDeact.includes(escapeHtml(t.rollbackPreservedNote)));
    }
  });

  it("shows spinner and disabled state when isProcessing is true", () => {
    const html = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="reboot"
        isProcessing={true}
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes("animate-spin"));
    assert.ok(html.includes("Rebooting..."));
  });

  it("renders error message banner when errorMessage is passed", () => {
    const html = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="reboot"
        errorMessage="Network connection timed out."
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes('role="alert"'));
    assert.ok(html.includes("Network connection timed out."));
  });

  it("invokes onClose when cancel button is clicked", () => {
    let closed = false;
    let result: any = null;
    function Wrapper() {
      result = DestructiveModal({
        isOpen: true,
        actionType: "reboot",
        onConfirm: () => {},
        onClose: () => { closed = true; },
        currentLanguage: "en",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(Wrapper));

    function findBtn(node: any, testId: string): any {
      if (!node) return null;
      if (node.props?.["data-testid"] === testId) return node;
      if (node.props?.children) {
        const children = Array.isArray(node.props.children) ? node.props.children : [node.props.children];
        for (const child of children) {
          const found = findBtn(child, testId);
          if (found) return found;
        }
      }
      return null;
    }

    const cancelBtn = findBtn(result, "destructive-cancel-button");
    assert.ok(cancelBtn, "Cancel button must exist");
    cancelBtn.props.onClick();
    assert.equal(closed, true, "onClose should be called on cancel click");
  });

  it("invokes onConfirm when confirm button is clicked in reboot mode", () => {
    let confirmed = false;
    let result: any = null;
    function Wrapper() {
      result = DestructiveModal({
        isOpen: true,
        actionType: "reboot",
        onConfirm: () => { confirmed = true; },
        onClose: () => {},
        currentLanguage: "en",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(Wrapper));

    function findBtn(node: any, testId: string): any {
      if (!node) return null;
      if (node.props?.["data-testid"] === testId) return node;
      if (node.props?.children) {
        const children = Array.isArray(node.props.children) ? node.props.children : [node.props.children];
        for (const child of children) {
          const found = findBtn(child, testId);
          if (found) return found;
        }
      }
      return null;
    }

    const confirmBtn = findBtn(result, "destructive-confirm-button");
    assert.ok(confirmBtn, "Confirm button must exist");
    assert.equal(confirmBtn.props.disabled, false);
    confirmBtn.props.onClick();
    assert.equal(confirmed, true, "onConfirm should be called on confirm click");
  });

  it("blocks onConfirm when confirmation is invalid in factory-reset mode", () => {
    let confirmed = false;
    let result: any = null;
    function Wrapper() {
      result = DestructiveModal({
        isOpen: true,
        actionType: "factory-reset",
        onConfirm: () => { confirmed = true; },
        onClose: () => {},
        currentLanguage: "en",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(Wrapper));

    function findBtn(node: any, testId: string): any {
      if (!node) return null;
      if (node.props?.["data-testid"] === testId) return node;
      if (node.props?.children) {
        const children = Array.isArray(node.props.children) ? node.props.children : [node.props.children];
        for (const child of children) {
          const found = findBtn(child, testId);
          if (found) return found;
        }
      }
      return null;
    }

    const confirmBtn = findBtn(result, "destructive-confirm-button");
    assert.ok(confirmBtn, "Confirm button must exist");
    assert.equal(confirmBtn.props.disabled, true);
    confirmBtn.props.onClick();
    assert.equal(confirmed, false, "onConfirm must not be called when reset confirmation is invalid");
  });

  it("validates localized confirmation tokens case-insensitively across EN, DE, ES, and FR (Story 8.4)", () => {
    // 1. English (EN): RESET, DELETE
    assert.equal(isValidConfirmationToken("RESET", "en"), true);
    assert.equal(isValidConfirmationToken("reset", "en"), true);
    assert.equal(isValidConfirmationToken("Reset", "en"), true);
    assert.equal(isValidConfirmationToken("DELETE", "en"), true);
    assert.equal(isValidConfirmationToken("delete", "en"), true);
    assert.equal(isValidConfirmationToken("  reset  ", "en"), true);
    assert.equal(isValidConfirmationToken("invalid", "en"), false);
    assert.equal(isValidConfirmationToken("", "en"), false);

    // 2. German (DE): LÖSCHEN, ZURÜCKSETZEN, RESET, DELETE
    assert.equal(isValidConfirmationToken("LÖSCHEN", "de"), true);
    assert.equal(isValidConfirmationToken("löschen", "de"), true);
    assert.equal(isValidConfirmationToken("Löschen", "de"), true);
    assert.equal(isValidConfirmationToken("loeschen", "de"), true);
    assert.equal(isValidConfirmationToken("ZURÜCKSETZEN", "de"), true);
    assert.equal(isValidConfirmationToken("zurücksetzen", "de"), true);
    assert.equal(isValidConfirmationToken("zuruecksetzen", "de"), true);
    assert.equal(isValidConfirmationToken("RESET", "de"), true);
    assert.equal(isValidConfirmationToken("reset", "de"), true);
    assert.equal(isValidConfirmationToken("DELETE", "de"), true);
    assert.equal(isValidConfirmationToken("falsch", "de"), false);

    // 3. Spanish (ES): ELIMINAR, RESTABLECER, RESET, DELETE
    assert.equal(isValidConfirmationToken("ELIMINAR", "es"), true);
    assert.equal(isValidConfirmationToken("eliminar", "es"), true);
    assert.equal(isValidConfirmationToken("Eliminar", "es"), true);
    assert.equal(isValidConfirmationToken("RESTABLECER", "es"), true);
    assert.equal(isValidConfirmationToken("restablecer", "es"), true);
    assert.equal(isValidConfirmationToken("RESET", "es"), true);
    assert.equal(isValidConfirmationToken("DELETE", "es"), true);
    assert.equal(isValidConfirmationToken("incorrecto", "es"), false);

    // 4. French (FR): SUPPRIMER, RÉINITIALISER, REINITIALISER, RESET, DELETE
    assert.equal(isValidConfirmationToken("SUPPRIMER", "fr"), true);
    assert.equal(isValidConfirmationToken("supprimer", "fr"), true);
    assert.equal(isValidConfirmationToken("Supprimer", "fr"), true);
    assert.equal(isValidConfirmationToken("RÉINITIALISER", "fr"), true);
    assert.equal(isValidConfirmationToken("réinitialiser", "fr"), true);
    assert.equal(isValidConfirmationToken("reinitialiser", "fr"), true);
    assert.equal(isValidConfirmationToken("RESET", "fr"), true);
    assert.equal(isValidConfirmationToken("DELETE", "fr"), true);
    assert.equal(isValidConfirmationToken("faux", "fr"), false);

    // 5. Unicode NFD decomposed combining characters (macOS/iOS keyboards)
    assert.equal(isValidConfirmationToken("lo\u0308schen", "de"), true, "matches NFD decomposed German umlaut");
    assert.equal(isValidConfirmationToken("zuru\u0308cksetzen", "de"), true, "matches NFD decomposed German ü");
    assert.equal(isValidConfirmationToken("re\u0301initialiser", "fr"), true, "matches NFD decomposed French é");

    // 6. Runtime null/undefined/non-string type guards
    assert.equal(isValidConfirmationToken(null, "en"), false);
    assert.equal(isValidConfirmationToken(undefined, "de"), false);
    assert.equal(isValidConfirmationToken(123 as any, "es"), false);
  });

  it("renders localized placeholders and aria-invalid attributes in factory-reset input (Story 8.4)", () => {
    const htmlEn = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );
    assert.ok(htmlEn.includes('placeholder="RESET"'), "renders English placeholder");

    const htmlDe = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="de"
      />
    );
    assert.ok(htmlDe.includes('placeholder="RESET / LÖSCHEN"'), "renders German placeholder");

    const htmlEs = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="es"
      />
    );
    assert.ok(htmlEs.includes('placeholder="RESET / ELIMINAR"'), "renders Spanish placeholder");

    const htmlFr = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="fr"
      />
    );
    assert.ok(htmlFr.includes('placeholder="RESET / SUPPRIMER"'), "renders French placeholder");

    // aria-invalid validation test
    const htmlInvalid = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
        initialResetInput="wrong"
      />
    );
    assert.ok(htmlInvalid.includes('aria-invalid="true"'), "sets aria-invalid=true on invalid non-empty input");

    const htmlValid = renderToStaticMarkup(
      <DestructiveModal
        isOpen={true}
        actionType="factory-reset"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="de"
        initialResetInput="löschen"
      />
    );
    assert.ok(!htmlValid.includes('aria-invalid="true"'), "does not set aria-invalid on valid input");
  });

  it("enables confirm button and invokes onConfirm at component layer with localized tokens (Story 8.4)", () => {
    function findBtn(node: any, testId: string): any {
      if (!node) return null;
      if (node.props?.["data-testid"] === testId) return node;
      if (node.props?.children) {
        const children = Array.isArray(node.props.children) ? node.props.children : [node.props.children];
        for (const child of children) {
          const found = findBtn(child, testId);
          if (found) return found;
        }
      }
      return null;
    }

    // 1. German localized token "LÖSCHEN" enables confirm button
    let deConfirmed = false;
    let deResult: any = null;
    function DeWrapper() {
      deResult = DestructiveModal({
        isOpen: true,
        actionType: "factory-reset",
        onConfirm: () => { deConfirmed = true; },
        onClose: () => {},
        currentLanguage: "de",
        initialResetInput: "LÖSCHEN",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(DeWrapper));
    const deConfirmBtn = findBtn(deResult, "destructive-confirm-button");
    assert.ok(deConfirmBtn, "Confirm button must exist in DE mode");
    assert.equal(deConfirmBtn.props.disabled, false, "Confirm button must be enabled with LÖSCHEN");
    deConfirmBtn.props.onClick();
    assert.equal(deConfirmed, true, "onConfirm must be invoked on click with LÖSCHEN");

    // 2. Spanish localized token "eliminar" enables confirm button
    let esConfirmed = false;
    let esResult: any = null;
    function EsWrapper() {
      esResult = DestructiveModal({
        isOpen: true,
        actionType: "factory-reset",
        onConfirm: () => { esConfirmed = true; },
        onClose: () => {},
        currentLanguage: "es",
        initialResetInput: "eliminar",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(EsWrapper));
    const esConfirmBtn = findBtn(esResult, "destructive-confirm-button");
    assert.ok(esConfirmBtn, "Confirm button must exist in ES mode");
    assert.equal(esConfirmBtn.props.disabled, false, "Confirm button must be enabled with eliminar");
    esConfirmBtn.props.onClick();
    assert.equal(esConfirmed, true, "onConfirm must be invoked on click with eliminar");

    // 3. French localized token "supprimer" enables confirm button
    let frConfirmed = false;
    let frResult: any = null;
    function FrWrapper() {
      frResult = DestructiveModal({
        isOpen: true,
        actionType: "factory-reset",
        onConfirm: () => { frConfirmed = true; },
        onClose: () => {},
        currentLanguage: "fr",
        initialResetInput: "supprimer",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(FrWrapper));
    const frConfirmBtn = findBtn(frResult, "destructive-confirm-button");
    assert.ok(frConfirmBtn, "Confirm button must exist in FR mode");
    assert.equal(frConfirmBtn.props.disabled, false, "Confirm button must be enabled with supprimer");
    frConfirmBtn.props.onClick();
    assert.equal(frConfirmed, true, "onConfirm must be invoked on click with supprimer");

    // 4. Invalid input keeps confirm button disabled
    let invalidConfirmed = false;
    let invalidResult: any = null;
    function InvalidWrapper() {
      invalidResult = DestructiveModal({
        isOpen: true,
        actionType: "factory-reset",
        onConfirm: () => { invalidConfirmed = true; },
        onClose: () => {},
        currentLanguage: "de",
        initialResetInput: "falsch",
      });
      return null;
    }
    renderToStaticMarkup(React.createElement(InvalidWrapper));
    const invalidBtn = findBtn(invalidResult, "destructive-confirm-button");
    assert.ok(invalidBtn, "Confirm button must exist");
    assert.equal(invalidBtn.props.disabled, true, "Confirm button must be disabled with invalid token");
    invalidBtn.props.onClick();
    assert.equal(invalidConfirmed, false, "onConfirm must not be called with invalid token");
  });

  describe("Preview Channel Safeguard Mode (Story 10.2)", () => {
    it("renders preview-channel mode with PREVIEW validation input and disabled confirm button initially", () => {
      const html = renderToStaticMarkup(
        <DestructiveModal
          isOpen={true}
          actionType="preview-channel"
          onConfirm={() => {}}
          onClose={() => {}}
          currentLanguage="en"
        />
      );

      assert.ok(html.includes("Switch to Preview Channel"));
      assert.ok(html.includes("Type PREVIEW to confirm"));
      assert.ok(html.includes('data-testid="preview-channel-input"'));
      assert.ok(html.includes("disabled"));
      assert.ok(html.includes("Switch to Preview Channel"));
      assert.ok(html.includes("bg-amber-600"));
    });

    it("validates preview tokens correctly across EN, DE, ES, FR", () => {
      // EN
      assert.equal(isValidConfirmationToken("preview", "en", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("PREVIEW", "en", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("  preview  ", "en", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("reset", "en", "preview-channel"), false);

      // DE
      assert.equal(isValidConfirmationToken("vorschau", "de", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("VORSCHAU", "de", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("löschen", "de", "preview-channel"), false);

      // ES
      assert.equal(isValidConfirmationToken("preliminar", "es", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("vista previa", "es", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("eliminar", "es", "preview-channel"), false);

      // FR
      assert.equal(isValidConfirmationToken("aperçu", "fr", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("apercu", "fr", "preview-channel"), true);
      assert.equal(isValidConfirmationToken("supprimer", "fr", "preview-channel"), false);
    });

    it("enables confirm button with localized preview tokens and calls onConfirm on click", () => {
      function findBtn(node: any, testId: string): any {
        if (!node) return null;
        if (node.props?.["data-testid"] === testId) return node;
        if (node.props?.children) {
          const children = Array.isArray(node.props.children)
            ? node.props.children
            : [node.props.children];
          for (const child of children) {
            const found = findBtn(child, testId);
            if (found) return found;
          }
        }
        return null;
      }

      // German localized preview token "vorschau"
      let deConfirmed = false;
      let deResult: any = null;
      function DePreviewWrapper() {
        deResult = DestructiveModal({
          isOpen: true,
          actionType: "preview-channel",
          onConfirm: () => { deConfirmed = true; },
          onClose: () => {},
          currentLanguage: "de",
          initialResetInput: "vorschau",
        });
        return null;
      }
      renderToStaticMarkup(React.createElement(DePreviewWrapper));
      const deConfirmBtn = findBtn(deResult, "destructive-confirm-button");
      assert.ok(deConfirmBtn, "Confirm button must exist in DE preview mode");
      assert.equal(deConfirmBtn.props.disabled, false, "Confirm button must be enabled with vorschau");
      deConfirmBtn.props.onClick();
      assert.equal(deConfirmed, true, "onConfirm must be invoked on click with vorschau");

      // French localized preview token "aperçu"
      let frConfirmed = false;
      let frResult: any = null;
      function FrPreviewWrapper() {
        frResult = DestructiveModal({
          isOpen: true,
          actionType: "preview-channel",
          onConfirm: () => { frConfirmed = true; },
          onClose: () => {},
          currentLanguage: "fr",
          initialResetInput: "aperçu",
        });
        return null;
      }
      renderToStaticMarkup(React.createElement(FrPreviewWrapper));
      const frConfirmBtn = findBtn(frResult, "destructive-confirm-button");
      assert.ok(frConfirmBtn, "Confirm button must exist in FR preview mode");
      assert.equal(frConfirmBtn.props.disabled, false, "Confirm button must be enabled with aperçu");
      frConfirmBtn.props.onClick();
      assert.equal(frConfirmed, true, "onConfirm must be invoked on click with aperçu");
    });
  });
});
