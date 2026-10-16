package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Preset represents a pre-calibrated generation intent profile.
type Preset struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Temperature         float64  `json:"temperature"`
	TopP                float64  `json:"top_p"`
	RecommendedUseCases []string `json:"recommended_use_cases"`
}

// Pre-calibrated generation intent catalogue.
var (
	PresetPrecise = Preset{
		ID:                  "precise",
		Name:                "Precise & Factual",
		Description:         "Calibrated for factual extraction, document analysis, and invoice parsing with minimal hallucinations.",
		Temperature:         0.1,
		TopP:                0.9,
		RecommendedUseCases: []string{"document analysis", "deadline extraction", "invoice parsing", "fact retrieval"},
	}

	PresetBalanced = Preset{
		ID:                  "balanced",
		Name:                "Balanced Assistant",
		Description:         "Calibrated for natural conversational assistance and everyday household questions.",
		Temperature:         0.7,
		TopP:                0.95,
		RecommendedUseCases: []string{"conversational assistant", "general QA", "writing assistance"},
	}

	PresetCreative = Preset{
		ID:                  "creative",
		Name:                "Creative Drafting",
		Description:         "Calibrated for creative writing, idea brainstorming, and drafting.",
		Temperature:         0.85,
		TopP:                0.98,
		RecommendedUseCases: []string{"brainstorming", "creative drafting", "ideation", "story writing"},
	}

	// Presets maps preset IDs to Preset configurations.
	Presets = map[string]Preset{
		"precise":  PresetPrecise,
		"balanced": PresetBalanced,
		"creative": PresetCreative,
	}
)

// GetPreset returns the Preset for a given identifier, falling back to false if unknown.
func GetPreset(id string) (Preset, bool) {
	key := strings.ToLower(strings.TrimSpace(id))
	p, ok := Presets[key]
	return p, ok
}

// ListPresets returns the ordered list of available presets.
func ListPresets() []Preset {
	return []Preset{
		PresetPrecise,
		PresetBalanced,
		PresetCreative,
	}
}

// NormalizeInferencePayload inspects an inference JSON body, detects any requested
// preset/intent (or falls back to defaultPreset / balanced), and injects calibrated
// temperature and top_p values unless explicit numeric overrides were provided by the user.
func NormalizeInferencePayload(body []byte, defaultPreset string) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("invalid json inference payload: %w", err)
	}

	// Detect requested preset/intent
	var requestedPreset string
	if p, ok := payload["preset"].(string); ok && strings.TrimSpace(p) != "" {
		requestedPreset = strings.TrimSpace(p)
	} else if intent, ok := payload["intent"].(string); ok && strings.TrimSpace(intent) != "" {
		requestedPreset = strings.TrimSpace(intent)
	} else if strings.TrimSpace(defaultPreset) != "" {
		requestedPreset = strings.TrimSpace(defaultPreset)
	}

	// Look up preset; if invalid or omitted, fall back to balanced
	preset, ok := GetPreset(requestedPreset)
	if !ok {
		preset = PresetBalanced
	}

	// Inject calibrated temperature unless explicitly specified by power user
	if _, exists := payload["temperature"]; !exists || payload["temperature"] == nil {
		payload["temperature"] = preset.Temperature
	}

	// Inject calibrated top_p unless explicitly specified by power user
	if _, exists := payload["top_p"]; !exists || payload["top_p"] == nil {
		payload["top_p"] = preset.TopP
	}

	return json.Marshal(payload)
}
