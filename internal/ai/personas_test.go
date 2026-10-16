package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonas_Normalization(t *testing.T) {
	// Language normalization
	langCases := map[string]string{
		"en":      LangEN,
		"EN":      LangEN,
		"en-US":   LangEN,
		"de":      LangDE,
		"de-DE":   LangDE,
		"de-AT":   LangDE,
		"es":      LangES,
		"es-ES":   LangES,
		"fr":      LangFR,
		"fr-FR":   LangFR,
		"unknown": LangEN,
		"":        LangEN,
	}

	for input, expected := range langCases {
		got := NormalizeLanguage(input)
		if got != expected {
			t.Errorf("NormalizeLanguage(%q) = %q, expected %q", input, got, expected)
		}
	}

	// Persona ID normalization
	idCases := map[string]string{
		"household":                PersonaHousehold,
		"household_assistant":      PersonaHousehold,
		"household-assistant":      PersonaHousehold,
		"HouseholdAssistant":       PersonaHousehold,
		"legal_advisor":            PersonaLegalAdvisor,
		"legal-advisor":            PersonaLegalAdvisor,
		"legal_contract_advisor":   PersonaLegalAdvisor,
		"legal":                    PersonaLegalAdvisor,
		"creative":                 PersonaCreative,
		"creative_writer":          PersonaCreative,
		"creative-writer":          PersonaCreative,
		"tech_support":             PersonaTechSupport,
		"tech-support":             PersonaTechSupport,
		"technical_support":        PersonaTechSupport,
		"support":                  PersonaTechSupport,
	}

	for input, expected := range idCases {
		got := NormalizePersonaID(input)
		if got != expected {
			t.Errorf("NormalizePersonaID(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestPersonas_CatalogueAndPresets(t *testing.T) {
	languages := []string{LangEN, LangDE, LangES, LangFR}
	expectedIDs := []string{PersonaHousehold, PersonaLegalAdvisor, PersonaCreative, PersonaTechSupport}

	expectedPresets := map[string]string{
		PersonaHousehold:    "balanced",
		PersonaLegalAdvisor: "precise",
		PersonaCreative:     "creative",
		PersonaTechSupport:  "precise",
	}

	for _, lang := range languages {
		for _, id := range expectedIDs {
			def, ok := builtInPersonas[id]
			if !ok {
				t.Fatalf("builtInPersonas missing persona %q", id)
			}

			p := BuildPersona(def, lang, false)
			if p.ID != id {
				t.Errorf("expected persona ID %q, got %q", id, p.ID)
			}
			if p.Preset != expectedPresets[id] {
				t.Errorf("persona %q expected preset %q, got %q", id, expectedPresets[id], p.Preset)
			}
			if p.Name == "" {
				t.Errorf("persona %q has empty name for language %q", id, lang)
			}
			if p.Description == "" {
				t.Errorf("persona %q has empty description for language %q", id, lang)
			}
			if p.SystemPrompt == "" {
				t.Errorf("persona %q has empty system prompt for language %q", id, lang)
			}
			if p.Language != lang {
				t.Errorf("expected persona language %q, got %q", lang, p.Language)
			}
		}
	}
}

func TestPersonas_LegalAdvisor_GermanNorms(t *testing.T) {
	def := builtInPersonas[PersonaLegalAdvisor]
	p := BuildPersona(def, LangDE, true)

	if p.Preset != "precise" {
		t.Fatalf("expected legal advisor preset to be 'precise', got %q", p.Preset)
	}

	// Must incorporate BGB statutory norms
	requiredSubstrings := []string{
		"BGB § 312g",
		"14-tägig",
		"Widerrufsrecht",
		"Gewährleistung",
		"BGB § 437",
		"BGB § 477",
		"Kündigungsfrist",
		"BGB § 309",
		"Musterschreiben",
	}

	for _, s := range requiredSubstrings {
		if !strings.Contains(p.SystemPrompt, s) {
			t.Errorf("German LegalAdvisor system prompt missing required statutory norm: %q", s)
		}
	}
}

func TestPersonas_LegalAdvisor_MultilingualNorms(t *testing.T) {
	def := builtInPersonas[PersonaLegalAdvisor]

	// English check
	pEN := BuildPersona(def, LangEN, false)
	if !strings.Contains(pEN.SystemPrompt, "14-day") || !strings.Contains(pEN.SystemPrompt, "warranty") {
		t.Errorf("English LegalAdvisor prompt missing cooling-off or warranty terms")
	}

	// Spanish check
	pES := BuildPersona(def, LangES, false)
	if !strings.Contains(pES.SystemPrompt, "14 días") || !strings.Contains(pES.SystemPrompt, "desistimiento") {
		t.Errorf("Spanish LegalAdvisor prompt missing 14-day desistimiento terms")
	}

	// French check
	pFR := BuildPersona(def, LangFR, false)
	if !strings.Contains(pFR.SystemPrompt, "14 jours") || !strings.Contains(pFR.SystemPrompt, "rétractation") {
		t.Errorf("French LegalAdvisor prompt missing 14-day rétractation terms")
	}
}

func TestPersonaStore_PersistenceAndSeeding(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sovereign-personas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewPersonaStore(tempDir, nil)

	// 1. Initial defaults
	activeID, lang := store.GetActive()
	if activeID != PersonaHousehold || lang != LangEN {
		t.Fatalf("expected initial active household/en, got %s/%s", activeID, lang)
	}

	// 2. Set active to Legal Advisor in German
	persona, err := store.SetActive("legal_advisor", "de")
	if err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	if persona.ID != PersonaLegalAdvisor || persona.Language != LangDE {
		t.Fatalf("expected active persona legal_advisor/de, got %s/%s", persona.ID, persona.Language)
	}
	if !persona.IsActive {
		t.Errorf("expected persona.IsActive to be true")
	}

	// 3. Verify persona_state.json exists and contains saved data
	statePath := filepath.Join(tempDir, "persona_state.json")
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read persona_state.json: %v", err)
	}
	if !strings.Contains(string(stateBytes), "legal_advisor") || !strings.Contains(string(stateBytes), "de") {
		t.Errorf("persona_state.json content mismatch: %s", string(stateBytes))
	}

	// 4. Verify system_prompt.txt seeded with German legal prompt
	promptPath := filepath.Join(tempDir, "system_prompt.txt")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("failed to read system_prompt.txt: %v", err)
	}
	if !strings.Contains(string(promptBytes), "BGB § 312g") {
		t.Errorf("system_prompt.txt does not contain German legal prompt: %s", string(promptBytes))
	}

	// 5. Verify reload in new store instance preserves state
	newStore := NewPersonaStore(tempDir, nil)
	reloadedID, reloadedLang := newStore.GetActive()
	if reloadedID != PersonaLegalAdvisor || reloadedLang != LangDE {
		t.Fatalf("expected reloaded store to have legal_advisor/de, got %s/%s", reloadedID, reloadedLang)
	}

	// 6. Test GetPersona with localization
	pDE, err := newStore.GetPersona("legal_advisor", "de")
	if err != nil {
		t.Fatalf("GetPersona failed: %v", err)
	}
	if pDE.Name != "Rechts- & Vertragsberater" {
		t.Errorf("expected German name 'Rechts- & Vertragsberater', got %q", pDE.Name)
	}
	if !pDE.IsActive {
		t.Errorf("expected active persona to have IsActive=true")
	}

	// 7. Test invalid persona
	_, err = newStore.GetPersona("nonexistent_id", "en")
	if err == nil {
		t.Errorf("expected error for nonexistent persona, got nil")
	}
	_, err = newStore.SetActive("nonexistent_id", "en")
	if err == nil {
		t.Errorf("expected error setting invalid persona, got nil")
	}

	// 8. Test ListPersonas
	list := newStore.ListPersonas("de")
	if len(list) != 4 {
		t.Fatalf("expected 4 personas in list, got %d", len(list))
	}
	foundActive := false
	for _, p := range list {
		if p.ID == PersonaLegalAdvisor {
			if !p.IsActive {
				t.Errorf("expected legal_advisor in list to be active")
			}
			foundActive = true
		}
	}
	if !foundActive {
		t.Errorf("legal_advisor not found in list")
	}

	// 9. Test ListPrompts
	allPrompts := newStore.ListPrompts("")
	if len(allPrompts) != 16 { // 4 personas * 4 languages
		t.Fatalf("expected 16 prompt templates, got %d", len(allPrompts))
	}
	dePrompts := newStore.ListPrompts("de")
	if len(dePrompts) != 4 {
		t.Fatalf("expected 4 de prompt templates, got %d", len(dePrompts))
	}
}
