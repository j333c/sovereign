import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import DropzonePage, { formatBytes, MediaCategory } from "./page";
import { translations } from "@/lib/i18n";
import { DropHistoryItem } from "@/lib/api/client";

describe("DropzonePage Component", () => {
  it("renders DropzonePage with header, category tabs, and drop target", () => {
    const html = renderToStaticMarkup(<DropzonePage />);

    // Header and title
    assert.ok(html.includes("Media Dropzone"), "renders Media Dropzone heading");
    assert.ok(html.includes("Storage Ingestion Ready"), "renders reassurance badge");

    // All 4 category tabs
    assert.ok(html.includes("Movies"), "renders Movies category tab");
    assert.ok(html.includes("TV Shows"), "renders TV Shows category tab");
    assert.ok(html.includes("Music"), "renders Music category tab");
    assert.ok(html.includes("Audiobooks"), "renders Audiobooks category tab");

    // Default tab selected is movies
    assert.ok(html.includes('id="tab-movies"'), "tab-movies element exists");
    assert.ok(html.includes('aria-selected="true"'), "a tab is selected by default");

    // Drop target prompt and format support
    assert.ok(html.includes("Drop files here or click to browse"), "renders drag & drop prompt");
    assert.ok(html.includes("Supports MP4, MKV, WebM, AVI, TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC"), "renders supported formats");
  });

  it("renders copyable network share helper paths for SMB file drops", () => {
    const html = renderToStaticMarkup(<DropzonePage />);

    assert.ok(html.includes("Home Network Folder"), "renders network share title");
    assert.ok(html.includes("\\\\sovereign\\drop"), "renders Windows UNC share path");
    assert.ok(html.includes("smb://sovereign.local/drop"), "renders macOS/Linux SMB path");
    assert.ok(html.includes("Copy Windows drop path"), "renders accessible Windows copy button aria-label");
    assert.ok(html.includes("Copy SMB drop path"), "renders accessible SMB copy button aria-label");
  });

  it("renders recent uploads container with accessible empty state", () => {
    const html = renderToStaticMarkup(<DropzonePage />);

    assert.ok(html.includes("Recent Uploads"), "renders Recent Uploads section");
    assert.ok(html.includes("Refresh"), "renders refresh button");
  });

  it("formatBytes formats file sizes correctly", () => {
    assert.equal(formatBytes(0), "0 B");
    assert.equal(formatBytes(1024), "1 KB");
    assert.equal(formatBytes(1048576), "1 MB");
    assert.equal(formatBytes(14889779), "14.2 MB");
    assert.equal(formatBytes(1073741824), "1 GB");
    assert.equal(formatBytes(5368709120), "5 GB");
  });

  it("includes accessible keyboard triggers and touch targets", () => {
    const html = renderToStaticMarkup(<DropzonePage />);

    // Touch target minimum 44px
    assert.ok(html.includes("min-h-[44px]"), "ensures 44px minimum touch target height");
    // Visible focus ring
    assert.ok(html.includes("focus-visible:ring-2"), "has visible focus rings");
    // Tablist & tabs role
    assert.ok(html.includes('role="tablist"'), "has tablist role");
    assert.ok(html.includes('role="tab"'), "has tab role");
  });
});
