package compliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root containing go.mod from %s", wd)
		}
		dir = parent
	}
}

func TestOpenSourceGovernanceSuite(t *testing.T) {
	root := findRepoRoot(t)

	t.Run("Root README.md exists and contains required attributions", func(t *testing.T) {
		readmePath := filepath.Join(root, "README.md")
		data, err := os.ReadFile(readmePath)
		if err != nil {
			t.Fatalf("expected README.md to exist: %v", err)
		}
		content := string(data)

		// 1. Architecture overview
		if !strings.Contains(content, "Architecture Overview") || !strings.Contains(content, "Caddy") {
			t.Errorf("README.md missing architecture overview")
		}

		// 2. Quickstart instructions
		if !strings.Contains(content, "Quickstart & Installation") || !strings.Contains(content, "sovereign.local") {
			t.Errorf("README.md missing quickstart instructions")
		}

		// 3. BMad Method attribution
		if !strings.Contains(content, "BMad Method") || !strings.Contains(content, "BMad Method Attribution") {
			t.Errorf("README.md missing explicit BMad Method attribution")
		}

		// 4. AI-assisted development transparency notice
		if !strings.Contains(content, "AI-Assisted") || !strings.Contains(content, "Transparent AI-Assisted Development Notice") {
			t.Errorf("README.md missing transparent AI-assisted development notice")
		}

		// 5. Zero-telemetry invariants
		if !strings.Contains(content, "sovereign-ai") || !strings.Contains(content, "Zero Docker Socket") {
			t.Errorf("README.md missing zero-telemetry architectural invariants")
		}
	})

	t.Run("Root LICENSE exists and contains Apache 2.0 terms", func(t *testing.T) {
		licensePath := filepath.Join(root, "LICENSE")
		data, err := os.ReadFile(licensePath)
		if err != nil {
			t.Fatalf("expected LICENSE file to exist: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "Apache License") || !strings.Contains(content, "Version 2.0") {
			t.Errorf("LICENSE does not appear to be Apache License 2.0")
		}
	})

	t.Run("Root NOTICE file exists with canonical Apache 2.0 attributions", func(t *testing.T) {
		noticePath := filepath.Join(root, "NOTICE")
		data, err := os.ReadFile(noticePath)
		if err != nil {
			t.Fatalf("expected NOTICE file to exist: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "Sovereign") || !strings.Contains(content, "BMad Method") {
			t.Errorf("NOTICE missing Sovereign project or BMad Method attribution")
		}

		for _, spoke := range []string{"Jellyfin", "Navidrome", "Paperless-ngx", "llama.cpp"} {
			if !strings.Contains(content, spoke) {
				t.Errorf("NOTICE missing spoke attribution: %s", spoke)
			}
		}
	})

	t.Run("LICENSES.md catalogs all 8 bundled open-source spoke applications with repos and copyrights", func(t *testing.T) {
		licensesPath := filepath.Join(root, "LICENSES.md")
		data, err := os.ReadFile(licensesPath)
		if err != nil {
			t.Fatalf("expected LICENSES.md to exist: %v", err)
		}
		content := string(data)

		type spokeRequirement struct {
			name      string
			repoUrl   string
			copyright string
		}

		requiredSpokes := []spokeRequirement{
			{name: "Jellyfin", repoUrl: "https://github.com/jellyfin/jellyfin", copyright: "Jellyfin Contributors"},
			{name: "Navidrome", repoUrl: "https://github.com/navidrome/navidrome", copyright: "Deluan Quintao"},
			{name: "Audiobookshelf", repoUrl: "https://github.com/advplyr/audiobookshelf", copyright: "advplyr"},
			{name: "Paperless-ngx", repoUrl: "https://github.com/paperless-ngx/paperless-ngx", copyright: "Paperless-ngx Contributors"},
			{name: "Open WebUI", repoUrl: "https://github.com/open-webui/open-webui", copyright: "Open WebUI Community"},
			{name: "llama.cpp", repoUrl: "https://github.com/ggerganov/llama.cpp", copyright: "Georgi Gerganov"},
			{name: "Caddy", repoUrl: "https://github.com/caddyserver/caddy", copyright: "Matthew Holt"},
			{name: "Automatic Ripping Machine", repoUrl: "https://github.com/automatic-ripping-machine/automatic-ripping-machine", copyright: "1337-server"},
		}

		for _, spoke := range requiredSpokes {
			if !strings.Contains(content, spoke.name) {
				t.Errorf("LICENSES.md missing required spoke name: %s", spoke.name)
			}
			if !strings.Contains(content, spoke.repoUrl) {
				t.Errorf("LICENSES.md missing repository URL for %s", spoke.name)
			}
			if !strings.Contains(content, spoke.copyright) {
				t.Errorf("LICENSES.md missing copyright holder for %s", spoke.name)
			}
		}

		// Check isolation boundary statement
		if !strings.Contains(content, "Architectural Isolation & Legal Boundary Statement") {
			t.Errorf("LICENSES.md missing architectural isolation & legal boundary statement")
		}

		if !strings.Contains(content, "GPL") {
			t.Errorf("LICENSES.md missing GPL compliance references")
		}

		// Check host dependencies
		for _, dep := range []string{"modernc.org/sqlite", "zeroconf", "yaml.v3"} {
			if !strings.Contains(content, dep) {
				t.Errorf("LICENSES.md missing host dependency: %s", dep)
			}
		}
	})

	t.Run("docs/licenses/README.md links to root LICENSES.md", func(t *testing.T) {
		docPath := filepath.Join(root, "docs", "licenses", "README.md")
		data, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("expected docs/licenses/README.md to exist: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "LICENSES.md") || !strings.Contains(content, "Jellyfin") {
			t.Errorf("docs/licenses/README.md missing reference to LICENSES.md or spokes")
		}
	})

	t.Run("Community governance and security policies exist with required sections", func(t *testing.T) {
		// CONTRIBUTING.md
		contribPath := filepath.Join(root, "CONTRIBUTING.md")
		contribData, err := os.ReadFile(contribPath)
		if err != nil {
			t.Fatalf("expected CONTRIBUTING.md to exist: %v", err)
		}
		contrib := string(contribData)
		if !strings.Contains(contrib, "Code of Conduct") || !strings.Contains(contrib, "Core Architectural Invariants") || !strings.Contains(contrib, "Go 1.27+") {
			t.Errorf("CONTRIBUTING.md missing required sections")
		}

		// GOVERNANCE.md
		govPath := filepath.Join(root, "GOVERNANCE.md")
		govData, err := os.ReadFile(govPath)
		if err != nil {
			t.Fatalf("expected GOVERNANCE.md to exist: %v", err)
		}
		gov := string(govData)
		if !strings.Contains(gov, "Maintainership Model") || !strings.Contains(gov, "Decision-Making Process") || !strings.Contains(gov, "Ethical AI Usage Disclosures") {
			t.Errorf("GOVERNANCE.md missing required sections")
		}

		// SECURITY.md
		secPath := filepath.Join(root, "SECURITY.md")
		secData, err := os.ReadFile(secPath)
		if err != nil {
			t.Fatalf("expected SECURITY.md to exist: %v", err)
		}
		sec := string(secData)
		if !strings.Contains(sec, "Air-Gapped AI Network") || !strings.Contains(sec, "Reporting a Vulnerability") || !strings.Contains(sec, "security@crins.de") {
			t.Errorf("SECURITY.md missing required sections")
		}
	})
}
