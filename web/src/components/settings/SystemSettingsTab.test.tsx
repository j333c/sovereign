import React from "react";
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { SystemSettingsTab } from "./SystemSettingsTab";
import { translations } from "@/lib/i18n";

describe("SystemSettingsTab Component (Story 10.2)", () => {
  const t = translations.en;

  it("renders channel selection dropdown with stable, security-only, and preview options", () => {
    const html = renderToStaticMarkup(
      <SystemSettingsTab
        t={t}
        currentChannel="stable"
        currentVersion="v0.1.0"
        onOpenDestructiveModal={() => {}}
      />
    );

    assert.ok(html.includes('data-testid="update-channel-select"'));
    assert.ok(html.includes('value="stable"'));
    assert.ok(html.includes('value="security-only"'));
    assert.ok(html.includes('value="preview"'));
    assert.ok(html.includes(t.channelStable));
    assert.ok(html.includes(t.channelSecurityOnly));
    assert.ok(html.includes(t.channelPreview));
  });

  it("does not render graduating-to-stable notice when on stable version", () => {
    const html = renderToStaticMarkup(
      <SystemSettingsTab
        t={t}
        currentChannel="stable"
        currentVersion="v0.1.0"
        onOpenDestructiveModal={() => {}}
      />
    );

    assert.ok(!html.includes('data-testid="graduating-to-stable-notice"'));
  });

  it("renders graduating-to-stable notice when running preview release on stable channel", () => {
    const html = renderToStaticMarkup(
      <SystemSettingsTab
        t={t}
        currentChannel="stable"
        currentVersion="v0.2.0-rc.1"
        onOpenDestructiveModal={() => {}}
      />
    );

    assert.ok(html.includes('data-testid="graduating-to-stable-notice"'));
    assert.ok(html.includes(t.graduatingToStableTitle));
    assert.ok(html.includes('href="/settings?tab=services"'));
    assert.ok(html.includes('data-testid="graduating-rollback-link"'));
  });

  it("invokes onOpenDestructiveModal when preview is selected", () => {
    let modalOpenedWith: any = null;
    let result: any = null;

    function Wrapper() {
      result = SystemSettingsTab({
        t,
        currentChannel: "stable",
        currentVersion: "v0.1.0",
        onChannelChange: async () => {},
        onOpenDestructiveModal: (state) => {
          modalOpenedWith = state;
        },
      });
      return null;
    }

    renderToStaticMarkup(React.createElement(Wrapper));

    function findSelect(node: any): any {
      if (!node) return null;
      if (node.props?.["data-testid"] === "update-channel-select") return node;
      if (node.props?.children) {
        const children = Array.isArray(node.props.children)
          ? node.props.children
          : [node.props.children];
        for (const child of children) {
          const found = findSelect(child);
          if (found) return found;
        }
      }
      return null;
    }

    const select = findSelect(result);
    assert.ok(select, "Select element must exist");

    // Simulate choosing "preview"
    select.props.onChange({ target: { value: "preview" } });
    assert.deepEqual(modalOpenedWith, {
      isOpen: true,
      actionType: "preview-channel",
    });
  });

  it("calls onChannelChange directly when switching between stable and security-only", async () => {
    let changedTo: string | null = null;
    let result: any = null;

    function Wrapper() {
      result = SystemSettingsTab({
        t,
        currentChannel: "stable",
        currentVersion: "v0.1.0",
        onChannelChange: async (c) => {
          changedTo = c;
        },
        onOpenDestructiveModal: () => {},
      });
      return null;
    }

    renderToStaticMarkup(React.createElement(Wrapper));

    function findSelect(node: any): any {
      if (!node) return null;
      if (node.props?.["data-testid"] === "update-channel-select") return node;
      if (node.props?.children) {
        const children = Array.isArray(node.props.children)
          ? node.props.children
          : [node.props.children];
        for (const child of children) {
          const found = findSelect(child);
          if (found) return found;
        }
      }
      return null;
    }

    const select = findSelect(result);
    assert.ok(select, "Select element must exist");

    await select.props.onChange({ target: { value: "security-only" } });
    assert.equal(changedTo, "security-only");
  });
});
