import React from "react";
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { DestructiveModal } from "./DestructiveModal";
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
});
