import { describe, it } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

function findRepoRoot(): string {
  let cur = process.cwd();
  while (cur !== path.parse(cur).root) {
    if (fs.existsSync(path.join(cur, "go.mod"))) {
      return cur;
    }
    cur = path.dirname(cur);
  }
  if (fs.existsSync(path.join(cur, "go.mod"))) {
    return cur;
  }
  throw new Error("go.mod not found in parent directory hierarchy");
}

describe("Open-Source Governance & License Compliance Suite (Story 7.3)", () => {
  const root = findRepoRoot();

  it("validates root README.md has architecture, quickstart, BMad attribution, and AI transparency", () => {
    const readmePath = path.join(root, "README.md");
    assert.ok(fs.existsSync(readmePath), "README.md exists");
    const content = fs.readFileSync(readmePath, "utf-8");

    assert.ok(content.includes("Architecture Overview"), "includes architecture overview");
    assert.ok(content.includes("Quickstart & Installation"), "includes quickstart guide");
    assert.ok(content.includes("BMad Method Attribution"), "includes BMad Method attribution");
    assert.ok(content.includes("Transparent AI-Assisted Development Notice"), "includes AI transparency notice");
    assert.ok(content.includes("sovereign-ai"), "documents air-gapped AI bridge");
    assert.ok(content.includes("sovereign-web"), "documents sovereign-web frontend in architecture");
  });

  it("validates root LICENSE is Apache License 2.0", () => {
    const licensePath = path.join(root, "LICENSE");
    assert.ok(fs.existsSync(licensePath), "LICENSE exists");
    const content = fs.readFileSync(licensePath, "utf-8");

    assert.ok(content.includes("Apache License"), "contains Apache License header");
    assert.ok(content.includes("Version 2.0"), "specifies Version 2.0");
  });

  it("validates root NOTICE file exists with canonical Apache 2.0 attributions", () => {
    const noticePath = path.join(root, "NOTICE");
    assert.ok(fs.existsSync(noticePath), "NOTICE exists");
    const content = fs.readFileSync(noticePath, "utf-8");

    assert.ok(content.includes("Sovereign"), "includes Sovereign project name");
    assert.ok(content.includes("BMad Method"), "includes BMad Method attribution");
    assert.ok(content.includes("Jellyfin"), "includes Jellyfin spoke attribution");
  });

  it("validates LICENSES.md catalogs all 8 open-source spokes, repos, and container boundaries", () => {
    const licensesPath = path.join(root, "LICENSES.md");
    assert.ok(fs.existsSync(licensesPath), "LICENSES.md exists");
    const content = fs.readFileSync(licensesPath, "utf-8");

    const requiredSpokes = [
      { name: "Jellyfin", repo: "https://github.com/jellyfin/jellyfin" },
      { name: "Navidrome", repo: "https://github.com/navidrome/navidrome" },
      { name: "Audiobookshelf", repo: "https://github.com/advplyr/audiobookshelf" },
      { name: "Paperless-ngx", repo: "https://github.com/paperless-ngx/paperless-ngx" },
      { name: "Open WebUI", repo: "https://github.com/open-webui/open-webui" },
      { name: "llama.cpp", repo: "https://github.com/ggerganov/llama.cpp" },
      { name: "Caddy", repo: "https://github.com/caddyserver/caddy" },
      { name: "Automatic Ripping Machine", repo: "https://github.com/automatic-ripping-machine/automatic-ripping-machine" },
    ];

    for (const spoke of requiredSpokes) {
      assert.ok(content.includes(spoke.name), `LICENSES.md catalogs ${spoke.name}`);
      assert.ok(content.includes(spoke.repo), `LICENSES.md includes repository URL for ${spoke.name}`);
    }

    assert.ok(
      content.includes("Architectural Isolation & Legal Boundary Statement"),
      "documents container legal isolation boundary"
    );
  });

  it("validates docs/licenses/README.md links to root LICENSES.md", () => {
    const docPath = path.join(root, "docs", "licenses", "README.md");
    assert.ok(fs.existsSync(docPath), "docs/licenses/README.md exists");
    const content = fs.readFileSync(docPath, "utf-8");
    assert.ok(content.includes("LICENSES.md"), "links to root LICENSES.md");
  });

  it("validates CONTRIBUTING.md, GOVERNANCE.md, and SECURITY.md exist and contain required policies", () => {
    // CONTRIBUTING.md
    const contrib = fs.readFileSync(path.join(root, "CONTRIBUTING.md"), "utf-8");
    assert.ok(contrib.includes("Code of Conduct"), "CONTRIBUTING.md has Code of Conduct");
    assert.ok(contrib.includes("Core Architectural Invariants"), "CONTRIBUTING.md has Invariants");
    assert.ok(contrib.includes("Go 1.27+"), "CONTRIBUTING.md requires Go 1.27+");

    // GOVERNANCE.md
    const gov = fs.readFileSync(path.join(root, "GOVERNANCE.md"), "utf-8");
    assert.ok(gov.includes("Maintainership Model"), "GOVERNANCE.md has Maintainership Model");
    assert.ok(gov.includes("Decision-Making Process"), "GOVERNANCE.md has Decision-Making Process");

    // SECURITY.md
    const sec = fs.readFileSync(path.join(root, "SECURITY.md"), "utf-8");
    assert.ok(sec.includes("Reporting a Vulnerability"), "SECURITY.md has Vulnerability Reporting");
    assert.ok(sec.includes("security@crins.de"), "SECURITY.md has security contact email");
  });
});
