import React from "react";
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { RollbackModal } from "./RollbackModal";
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
  if (typeof node.type === "function") {
    try {
      const rendered = renderComponent(node.type, node.props);
      const found = findByTestId(rendered, testId);
      if (found) return found;
    } catch {
      // not a functional component
    }
  }
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

describe("RollbackModal Component (Story 6.3)", () => {
  it("renders null when isOpen is false", () => {
    const html = renderToStaticMarkup(
      <RollbackModal
        isOpen={false}
        serviceId="jellyfin"
        serviceName="Video Streaming"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );
    assert.equal(html, "");
  });

  it("renders accessible dialog attributes, version diff, and reassurance copy", () => {
    const html = renderToStaticMarkup(
      <RollbackModal
        isOpen={true}
        serviceId="jellyfin"
        serviceName="Video Streaming"
        serviceIcon="🎬"
        currentVersion="jellyfin/jellyfin:12.3.0"
        previousVersion="jellyfin/jellyfin:10.8.13"
        snapshotTimestamp="2026-10-17T03:00:00Z"
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    // Dialog accessibility attributes
    assert.ok(html.includes('role="dialog"'));
    assert.ok(html.includes('aria-modal="true"'));
    assert.ok(html.includes('aria-labelledby="rollback-title-jellyfin"'));
    assert.ok(html.includes('aria-describedby="rollback-desc-jellyfin"'));
    assert.ok(html.includes('data-testid="rollback-modal-jellyfin"'));

    // Service & version comparison
    assert.ok(html.includes("Roll Back to Previous Version: Video Streaming"));
    assert.ok(html.includes("12.3.0"));
    assert.ok(html.includes("10.8.13"));
    assert.ok(html.includes("Current Version"));
    assert.ok(html.includes("Previous Version"));
    assert.ok(html.includes("Backup Snapshot"));
    assert.ok(html.includes("SQLite WAL + Config"));

    // Reassurance note
    assert.ok(
      html.includes(
        "Your media, files, and documents are preserved and will not be touched."
      )
    );

    // Buttons
    assert.ok(html.includes('data-testid="cancel-rollback-jellyfin"'));
    assert.ok(html.includes('data-testid="confirm-rollback-jellyfin"'));
  });

  it("triggers onConfirm and onClose callbacks on action button clicks", async () => {
    let confirmCalled = false;
    let closeCalled = false;

    const props = {
      isOpen: true,
      serviceId: "paperless",
      serviceName: "Document Archive",
      currentVersion: "paperless:2.4.0",
      previousVersion: "paperless:2.3.0",
      snapshotTimestamp: "2026-10-17T02:30:00Z",
      onConfirm: () => {
        confirmCalled = true;
      },
      onClose: () => {
        closeCalled = true;
      },
      currentLanguage: "en" as const,
    };

    const tree = renderComponent(RollbackModal, props);

    const cancelBtn = findByTestId(tree, "cancel-rollback-paperless");
    assert.ok(cancelBtn, "Cancel button must exist");
    cancelBtn.props.onClick();
    assert.equal(closeCalled, true);

    const confirmBtn = findByTestId(tree, "confirm-rollback-paperless");
    assert.ok(confirmBtn, "Confirm button must exist");
    await confirmBtn.props.onClick();
    assert.equal(confirmCalled, true);
  });

  it("displays loading spinner and disables buttons when isRollingBack is true", () => {
    const html = renderToStaticMarkup(
      <RollbackModal
        isOpen={true}
        serviceId="paperless"
        serviceName="Document Archive"
        isRollingBack={true}
        onConfirm={() => {}}
        onClose={() => {}}
        currentLanguage="en"
      />
    );

    assert.ok(html.includes("Rolling back..."));
    assert.ok(html.includes("animate-spin"));
  });

  it("renders localized version diff and reassurance copy across all 4 languages", () => {
    const languages: Language[] = ["en", "de", "es", "fr"];

    for (const lang of languages) {
      const t = translations[lang];
      const html = renderToStaticMarkup(
        <RollbackModal
          isOpen={true}
          serviceId="navidrome"
          serviceName="Music & Audiobooks"
          currentVersion="navidrome:0.51.0"
          previousVersion="navidrome:0.50.0"
          snapshotTimestamp="2026-10-17T01:00:00Z"
          onConfirm={() => {}}
          onClose={() => {}}
          currentLanguage={lang}
        />
      );

      assert.ok(
        html.includes(t.rollbackVersion),
        `expected rollback title in ${lang}`
      );
      assert.ok(
        html.includes(t.currentVersionLabel),
        `expected current version label in ${lang}`
      );
      assert.ok(
        html.includes(t.previousVersionLabel),
        `expected previous version label in ${lang}`
      );
      assert.ok(
        html.includes(t.snapshotTimestampLabel),
        `expected snapshot label in ${lang}`
      );
      assert.ok(
        html.includes(t.rollbackPreservedNote),
        `expected preserved reassurance copy in ${lang}`
      );
      assert.ok(html.includes(t.cancel), `expected cancel button in ${lang}`);
      assert.ok(html.includes(t.walAndConfig), `expected walAndConfig badge in ${lang}`);
    }
  });

  it("disables confirm button when previousVersion is undefined and displays neutral placeholder for missing current version", () => {
    const props = {
      isOpen: true,
      serviceId: "navidrome",
      serviceName: "Music Streaming",
      currentVersion: undefined,
      previousVersion: undefined,
      onConfirm: () => {},
      onClose: () => {},
      currentLanguage: "en" as const,
    };

    const tree = renderComponent(RollbackModal, props);
    const confirmBtn = findByTestId(tree, "confirm-rollback-navidrome");
    assert.ok(confirmBtn, "Confirm button must exist");
    assert.equal(confirmBtn.props.disabled, true, "Confirm button must be disabled when no previous version exists");

    const html = renderToStaticMarkup(<RollbackModal {...props} />);
    assert.ok(html.includes("data-testid=\"current-version-tag\""));
    // Neutral placeholder "—"
    assert.ok(html.includes(">—</span>"));
    // Previous version shows "No prior version available"
    assert.ok(html.includes("No prior version available"));
  });

  it("triggers onClose on backdrop click when not rolling back", () => {
    let closed = false;
    const props = {
      isOpen: true,
      serviceId: "jellyfin",
      serviceName: "Video Streaming",
      onConfirm: () => {},
      onClose: () => {
        closed = true;
      },
      currentLanguage: "en" as const,
    };

    const tree = renderComponent(RollbackModal, props);
    const modalBackdrop = findByTestId(tree, "rollback-modal-jellyfin");
    assert.ok(modalBackdrop, "Modal backdrop must exist");

    // Click outside (target === currentTarget)
    const event = { target: "backdrop", currentTarget: "backdrop" };
    modalBackdrop.props.onClick(event);
    assert.equal(closed, true, "Clicking backdrop overlay must call onClose");
  });
});
