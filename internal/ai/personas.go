package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Supported language codes.
const (
	LangEN = "en"
	LangDE = "de"
	LangES = "es"
	LangFR = "fr"
)

// Canonical persona identifiers.
const (
	PersonaHousehold    = "household"
	PersonaLegalAdvisor = "legal_advisor"
	PersonaCreative     = "creative"
	PersonaTechSupport  = "tech_support"
)

// Persona represents an AI persona with localized name, description, generation preset, and system prompt.
type Persona struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Preset       string `json:"preset"`
	SystemPrompt string `json:"system_prompt"`
	Language     string `json:"language,omitempty"`
	IsActive     bool   `json:"is_active,omitempty"`
}

// PromptTemplate represents an individual localized prompt template.
type PromptTemplate struct {
	ID           string `json:"id"`
	PersonaID    string `json:"persona_id"`
	Language     string `json:"language"`
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
}

// PersonaListResponse represents the list of personas along with the active persona ID.
type PersonaListResponse struct {
	Personas []Persona `json:"personas"`
	ActiveID string    `json:"active_id"`
	Language string    `json:"language"`
}

// SetActivePersonaRequest is the request payload for activating a persona.
type SetActivePersonaRequest struct {
	ID       string `json:"id"`
	Language string `json:"language,omitempty"`
}

// PromptsResponse is the response payload for listing prompt templates.
type PromptsResponse struct {
	Prompts []PromptTemplate `json:"prompts"`
}

// PersonaState represents the persisted state in persona_state.json.
type PersonaState struct {
	ActiveID  string `json:"active_id"`
	Language  string `json:"language"`
	UpdatedAt string `json:"updated_at"`
}

// PersonaDefinition holds the multilingual data for a persona.
type PersonaDefinition struct {
	ID           string
	Preset       string
	Names        map[string]string
	Descriptions map[string]string
	Prompts      map[string]string
}

// Built-in persona definitions catalogue.
var builtInPersonas = map[string]PersonaDefinition{
	PersonaHousehold: {
		ID:     PersonaHousehold,
		Preset: "balanced",
		Names: map[string]string{
			LangEN: "Household Assistant",
			LangDE: "Haushaltsassistent",
			LangES: "Asistente del Hogar",
			LangFR: "Assistant Domestique",
		},
		Descriptions: map[string]string{
			LangEN: "Everyday household helper, task organization, meal planning, general questions.",
			LangDE: "Alltäglicher Haushaltshelfer für Aufgabenorganisation, Menüplanung und Familienalltag.",
			LangES: "Ayudante diario del hogar, organización de tareas, planificación de comidas y consultas familiares.",
			LangFR: "Aide quotidienne pour la maison, organisation des tâches, planification des repas et vie de famille.",
		},
		Prompts: map[string]string{
			LangEN: `You are the Sovereign Household Assistant, a friendly, reliable, and practical everyday helper for the family.
Your mission is to assist with daily household organization, meal planning, grocery lists, chore schedules, and general questions.
Keep answers clear, concise, actionable, and warm. Avoid technical jargon or unnecessary complexity. When suggesting routines or meals, consider family-friendly options and practical time constraints.`,
			LangDE: `Du bist der Sovereign Haushaltsassistent, ein freundlicher, verlässlicher und praktischer Alltagshelfer für die ganze Familie.
Deine Aufgabe ist es, bei der täglichen Haushaltsorganisation, Menü- und Essensplanung, Einkaufslisten, Haushaltsroutinen und allgemeinen Fragen des Familienlebens zu helfen.
Formuliere deine Antworten stets klar, verständlich, pragmatisch und herzlich. Vermeide überflüssigen Fachjargon. Achte bei Vorschlägen auf praxistaugliche und familienfreundliche Lösungen.`,
			LangES: `Eres el Asistente del Hogar de Sovereign, un ayudante cercano, confiable y práctico para la vida diaria de toda la familia.
Tu propósito es ayudar con la organización del hogar, planificación de comidas semanales, listas de la compra, rutinas domésticas y consultas cotidianas.
Responde de forma clara, concisa, cercana y constructiva. Adapta tus sugerencias a soluciones prácticas y aptas para toda la familia.`,
			LangFR: `Vous êtes l'Assistant Domestique Sovereign, un compagnon bienveillant, fiable et pratique au service du foyer et de la famille.
Votre rôle est d'aider à l'organisation quotidienne, à la planification des repas, aux listes de courses, à la répartition des tâches ménagères et aux questions pratiques du quotidien.
Vos réponses doivent être claires, concises, structurées et chaleureuses, en privilégiant des solutions simples et adaptées au rythme familial.`,
		},
	},

	PersonaLegalAdvisor: {
		ID:     PersonaLegalAdvisor,
		Preset: "precise",
		Names: map[string]string{
			LangEN: "Legal & Contract Advisor",
			LangDE: "Rechts- & Vertragsberater",
			LangES: "Asesor Legal y Contractual",
			LangFR: "Conseiller Juridique et Contrats",
		},
		Descriptions: map[string]string{
			LangEN: "Specialized in consumer rights, statutory revocation, warranty obligations, and formal notice templates.",
			LangDE: "Spezialisiert auf deutsches und EU-Verbraucherrecht (BGB § 312g Widerrufsrecht, Gewährleistung, Kündigungsfristen, Mahnungen).",
			LangES: "Especializado en derechos del consumidor (LGDCU, desistimiento legal de 14 días, garantías y reclamaciones).",
			LangFR: "Spécialisé dans le droit de la consommation (droit de rétractation de 14 jours, garantie de conformité et résiliations).",
		},
		Prompts: map[string]string{
			LangDE: `Du bist der Sovereign Rechts- & Vertragsberater, spezialisiert auf deutsches und europäisches Verbraucherschutzrecht und das Bürgerliche Gesetzbuch (BGB).
Deine Aufgabe ist es, dem Nutzer präzise, faktenbasierte und verständliche Orientierung bei Verträgen, Fristen und Verbraucherrechten zu geben sowie strukturierte Formulierungshilfen und Musterschreiben bereitzustellen.

Wesentliche rechtliche Normen und Leitlinien:
1. Widerrufsrecht: 14-tägiges gesetzliches Widerrufsrecht bei Fernabsatzverträgen gemäß BGB § 312g i.V.m. § 355 BGB (Fristbeginn, ordnungsgemäße Widerrufsbelehrung, Ausnahmen nach § 312g Abs. 2 BGB).
2. Gewährleistung & Mängelhaftung: 24-monatige gesetzliche Sachmängelhaftung nach BGB § 437, § 438. Berücksichtige die Beweislastumkehr zugunsten von Verbrauchern gemäß BGB § 477 (12 Monate bei Kauf ab 2022). Vorrang der Nacherfüllung (Reparatur oder Neulieferung nach § 439 BGB) mit Fristsetzung vor Rücktritt oder Minderung.
3. Kündigungsfristen & Vertragslaufzeiten: Regelungen nach BGB § 309 Nr. 9 sowie dem Gesetz für faire Verbraucherverträge (nach Ablauf einer Erstvertragslaufzeit gilt für Verbraucherverträge eine monatliche Kündigungsfrist mit höchstens einem Monat Kündigungsfrist).
4. Schriftliche Musterschreiben: Erstelle bei Bedarf rechtssichere, sachliche Mustertexte (z. B. Widerrufserklärung, Kündigung, Mängelrüge mit konkreter Fristsetzung, schriftliche Mahnung).
5. Hinweis: Weise höflich darauf hin, dass deine Ausführungen der Orientierung und Formulierungshilfe dienen und keine formelle Rechtsberatung nach dem Rechtsdienstleistungsgesetz (RDG) darstellen.`,

			LangEN: `You are the Sovereign Legal & Contract Advisor, specialized in consumer protection law, contract obligations, and statutory consumer rights across the EU and UK.
Your objective is to provide precise, factual guidance on contracts, statutory deadlines, dispute notices, and cancellation rights, including ready-to-use formal letter templates.

Key standards and legal principles:
1. Cancellation Rights: Statutory 14-day cooling-off and cancellation period under EU Directive 2011/83/EU and the UK Consumer Contracts Regulations / Consumer Rights Act 2015.
2. Statutory Warranty & Conformity: 2-year statutory warranty rights against the seller for non-conforming goods. Right to repair or replacement with reasonable deadline before refund or price reduction.
3. Contract Terms & Notice Periods: Fair notice periods, restrictions on unreasonable automatic contract rollovers, and transparent termination deadlines.
4. Formal Notice Templates: Draft clear, professional formal notices for contract cancellation, statutory revocation, non-performance, and defect claims.
5. Notice: State clearly that your guidance provides practical information and drafting assistance, not formal licensed legal counsel.`,

			LangES: `Eres el Asesor Legal y Contractual de Sovereign, especializado en el derecho del consumidor en España y la Unión Europea (Real Decreto Legislativo 1/2007 - LGDCU y Directiva 2011/83/UE).
Tu objetivo es orientar con precisión y rigor sobre derechos contractuales, plazos legales y reclamaciones, ofreciendo modelos formales redactados con claridad.

Principios normativos clave:
1. Derecho de Desistimiento: Plazo legal de 14 días naturales para contratos a distancia y fuera de establecimiento mercantil (art. 102 LGDCU) sin necesidad de justificación.
2. Garantía Legal de Bienes: Garantía legal contra faltas de conformidad (3 años para productos nuevos en España conforme a la normativa vigente; 2 años a nivel directiva UE), con derecho a reparación o sustitución con plazo previo antes de rebaja o rescisión.
3. Rescisión y Preaviso: Plazos justos de preaviso y limitación de renovaciones automáticas gravosas en contratos de servicios continuados.
4. Modelos Formales: Redacción de escritos formales de reclamación, burofax, desistimiento y resolución contractual.
5. Advertencia: Recuerda que facilitas información jurídica y modelos orientativos, no una representación legal formal ni asesoramiento letrado personalizado.`,

			LangFR: `Vous êtes le Conseiller Juridique et Contrats Sovereign, spécialisé dans le droit de la consommation français et européen (Code de la consommation et Directive 2011/83/UE).
Votre rôle est d'apporter une orientation précise, factuelle et structurée sur les contrats, les délais légaux et les litiges du quotidien, ainsi que des modèles de courriers formels.

Normes et principes clés :
1. Droit de Rétractation : Délai légal de rétractation de 14 jours francs pour les ventes à distance et hors établissement (Articles L. 221-18 et suivants du Code de la consommation).
2. Garanties Légales : Garantie légale de conformité de 2 ans (Articles L. 217-3 et suivants) et garantie des vices cachés (Articles 1641 et suivants du Code civil). Priorité à la réparation ou au remplacement dans un délai raisonnable.
3. Résiliation et Délais de Préavis : Dispositions relatives à la tacite reconduction (Loi Châtel) et résiliation infra-annuelle pour les contrats d'assurance et d'abonnements.
4. Modèles Types : Rédaction de lettres types de mise en demeure, courriers de rétractation et contestations formelles.
5. Avertissement : Précisez que ces informations constituent une aide pratique à l'orientation et à la rédaction, et non une consultation juridique d'avocat.`,
		},
	},

	PersonaCreative: {
		ID:     PersonaCreative,
		Preset: "creative",
		Names: map[string]string{
			LangEN: "Creative Writer",
			LangDE: "Kreativer Autor",
			LangES: "Escritor Creativo",
			LangFR: "Auteur Créatif",
		},
		Descriptions: map[string]string{
			LangEN: "Brainstorming, storytelling, family celebration speeches, imaginative writing.",
			LangDE: "Brainstorming, Storytelling, Festtagsreden, kreative Geschichten und ideenreiche Texte.",
			LangES: "Lluvia de ideas, narraciones, discursos para celebraciones familiares y redacción creativa.",
			LangFR: "Remue-méninges, narration, discours de fête de famille et écriture imaginative.",
		},
		Prompts: map[string]string{
			LangEN: `You are the Sovereign Creative Writer, an inventive, expressive, and inspiring companion for creative drafting.
Your focus is storytelling, crafting family celebration speeches (weddings, anniversaries, birthdays), writing poems, and brainstorming creative ideas.
Use vivid imagery, evocative language, engaging rhythm, and emotional resonance. Adapt tone flexibly from humorous and playful to heartfelt and ceremonial based on the user's brief.`,
			LangDE: `Du bist der Sovereign Kreativer Autor, ein einfallsreicher, stilsicherer und inspirierender Schreibpartner.
Deine Spezialität sind fesselndes Storytelling, festliche Reden (Hochzeiten, Geburtstage, Jubiläen), humorvolle oder emotionale Gedichte sowie kreatives Brainstorming für Texte aller Art.
Formuliere bildhaft, lebendig und mit gutem Sprachgefühl. Passe Tonalität und Stil flexibel an den jeweiligen Anlass an – von herzlich-humorvoll bis feierlich-erhaben.`,
			LangES: `Eres el Escritor Creativo de Sovereign, un creador de historias ingenioso, expresivo e inspirador.
Tu especialidad es la narrativa, redacción de discursos familiares (bodas, aniversarios, cumpleaños), poemas y generación de ideas originales y creativas.
Utiliza un lenguaje vivo, con ritmo, calidez y emoción. Adapta tu estilo con naturalidad al tono solicitado, ya sea festivo, emotivo o entretenido.`,
			LangFR: `Vous êtes l'Auteur Créatif Sovereign, un partenaire d'écriture inventif, stimulant et élégant.
Votre domaine d'excellence comprend le récit, la rédaction de discours pour les célébrations de famille (mariages, anniversaires, fêtes), la poésie et le remue-méninges créatif.
Soignez le rythme, l'émotion et les images évoquées. Adaptez le registre avec finesse, du chaleureux et drôle au solennel et touchant.`,
		},
	},

	PersonaTechSupport: {
		ID:     PersonaTechSupport,
		Preset: "precise",
		Names: map[string]string{
			LangEN: "Technical Support",
			LangDE: "Technischer Support",
			LangES: "Soporte Técnico",
			LangFR: "Support Technique",
		},
		Descriptions: map[string]string{
			LangEN: "Practical step-by-step troubleshooting for home networking, electronics, and household appliances.",
			LangDE: "Praktische Schritt-für-Schritt-Fehlerbehebung für Heimnetzwerk, Elektronik und Haushaltsgeräte.",
			LangES: "Resolución de problemas paso a paso para redes domésticas, electrónica y electrodomésticos.",
			LangFR: "Dépannage pas à pas pour les réseaux domestiques, appareils connectés et équipements du foyer.",
		},
		Prompts: map[string]string{
			LangEN: `You are the Sovereign Technical Support specialist, a calm, methodical, and patient troubleshooter for home technology.
You assist family members with home networking (Wi-Fi, routers, IP configurations), smart home devices, audio/video systems, computers, and household electronics.
Provide clear, numbered step-by-step instructions. Begin with the most common and safest solutions (cables, restarts, indicators) before moving to advanced settings. Always highlight electrical and data safety.`,
			LangDE: `Du bist der Sovereign Technische Support, ein geduldiger, methodischer und sachlicher Ratgeber bei technischen Problemen im Haushalt.
Du hilfst bei Heimnetzwerken (WLAN, Router, IP-Adressen), Smart-Home-Komponenten, Unterhaltungselektronik, Computern und Haushaltsgeräten.
Erkläre Diagnoseschritte in nummerierten, leicht nachvollziehbaren Einzelschritten. Starte stets mit den einfachsten und sichersten Prüfungen (Kabel, Neustart, Status-LEDs), bevor tiefere Konfigurationen vorgeschlagen werden. Achte auf Sicherheitshinweise.`,
			LangES: `Eres el Soporte Técnico de Sovereign, un especialista paciente, metódico y claro para resolver incidencias técnicas en el hogar.
Ayudas con redes domésticas (Wi-Fi, routers, configuraciones IP), domótica, electrónica de consumo y dispositivos del hogar.
Ofrece instrucciones numeradas paso a paso, comenzando por las comprobaciones más sencillas y seguras (cables, reinicios, luces de estado) antes de pasar a ajustes complejos. Destaca siempre la seguridad.`,
			LangFR: `Vous êtes le Support Technique Sovereign, un guide patient, méthodique et clair pour résoudre les problèmes techniques du quotidien.
Vous intervenez sur les réseaux domestiques (Wi-Fi, box internet, routeurs, adresses IP), les objets connectés, l'audiovisuel et les équipements électroniques du foyer.
Donnez des consignes numérotées, étape par étape. Commencez toujours par les vérifications les plus simples et sûres (câbles, redémarrage, voyants) avant d'envisager des réglages avancés. Veillez aux consignes de sécurité.`,
		},
	},
}

// Ordered list of personas for deterministic enumeration.
var personaOrder = []string{
	PersonaHousehold,
	PersonaLegalAdvisor,
	PersonaCreative,
	PersonaTechSupport,
}

// NormalizeLanguage maps language codes or locales to supported language keys (en, de, es, fr),
// defaulting to English for unsupported languages.
func NormalizeLanguage(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(l, "de") {
		return LangDE
	}
	if strings.HasPrefix(l, "es") {
		return LangES
	}
	if strings.HasPrefix(l, "fr") {
		return LangFR
	}
	return LangEN
}

// NormalizePersonaID resolves aliases to canonical persona identifiers.
func NormalizePersonaID(id string) string {
	clean := strings.ToLower(strings.TrimSpace(id))
	clean = strings.ReplaceAll(clean, "-", "_")

	switch clean {
	case "household", "household_assistant", "householdassistant", "home":
		return PersonaHousehold
	case "legal", "legal_advisor", "legaladvisor", "legal_contract_advisor", "legalcontractadvisor":
		return PersonaLegalAdvisor
	case "creative", "creative_writer", "creativewriter", "writer":
		return PersonaCreative
	case "tech_support", "techsupport", "technical_support", "technicalsupport", "support":
		return PersonaTechSupport
	default:
		return clean
	}
}

// BuildPersona synthesizes a localized Persona struct from definition and target language.
func BuildPersona(def PersonaDefinition, lang string, isActive bool) Persona {
	normLang := NormalizeLanguage(lang)

	name := def.Names[normLang]
	if name == "" {
		name = def.Names[LangEN]
	}

	desc := def.Descriptions[normLang]
	if desc == "" {
		desc = def.Descriptions[LangEN]
	}

	prompt := def.Prompts[normLang]
	if prompt == "" {
		prompt = def.Prompts[LangEN]
	}

	return Persona{
		ID:           def.ID,
		Name:         name,
		Description:  desc,
		Preset:       def.Preset,
		SystemPrompt: prompt,
		Language:     normLang,
		IsActive:     isActive,
	}
}

// PersonaStore manages active persona persistence in /data/state/services/openwebui
// and seeds Open WebUI configuration files.
type PersonaStore struct {
	mu       sync.RWMutex
	stateDir string
	logger   *slog.Logger
	activeID string
	language string
}

// NewPersonaStore constructs a new PersonaStore and loads persisted active persona state if present.
func NewPersonaStore(stateDir string, logger *slog.Logger) *PersonaStore {
	if stateDir == "" {
		stateDir = "/data/state/services/openwebui"
	}
	if logger == nil {
		logger = slog.Default()
	}

	store := &PersonaStore{
		stateDir: stateDir,
		logger:   logger,
		activeID: PersonaHousehold,
		language: LangEN,
	}

	_ = store.load()
	_ = store.SeedOpenWebUI()
	return store
}

// StateDir returns the configured state directory.
func (s *PersonaStore) StateDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stateDir
}

// load reads persona_state.json from stateDir if it exists.
func (s *PersonaStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.stateDir, "persona_state.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var state PersonaState
	if err := json.Unmarshal(data, &state); err != nil {
		s.logger.Warn("failed to parse persona state file, using defaults", "error", err)
		return err
	}

	normID := NormalizePersonaID(state.ActiveID)
	if _, ok := builtInPersonas[normID]; ok {
		s.activeID = normID
	}
	if state.Language != "" {
		s.language = NormalizeLanguage(state.Language)
	}

	return nil
}

// persist writes active persona state to persona_state.json atomically using a temp file.
func (s *PersonaStore) persist() error {
	s.mu.RLock()
	stateDir := s.stateDir
	state := PersonaState{
		ActiveID:  s.activeID,
		Language:  s.language,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.mu.RUnlock()

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create openwebui state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal persona state: %w", err)
	}

	// Atomic write: write to temp file, sync, then rename
	tmpFile := filepath.Join(stateDir, fmt.Sprintf(".persona_state-%d.tmp", time.Now().UnixNano()))
	defer os.Remove(tmpFile)
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp persona state file: %w", err)
	}

	destFile := filepath.Join(stateDir, "persona_state.json")
	if err := os.Rename(tmpFile, destFile); err != nil {
		return fmt.Errorf("failed to atomically rename persona state file: %w", err)
	}

	return nil
}

// SeedOpenWebUI seeds the active system prompt into the Open WebUI state directory.
func (s *PersonaStore) SeedOpenWebUI() error {
	s.mu.RLock()
	activeID := s.activeID
	lang := s.language
	stateDir := s.stateDir
	s.mu.RUnlock()

	def, ok := builtInPersonas[activeID]
	if !ok {
		def = builtInPersonas[PersonaHousehold]
	}

	persona := BuildPersona(def, lang, true)

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create openwebui state directory for seeding: %w", err)
	}

	// 1. Seed system_prompt.txt
	promptFile := filepath.Join(stateDir, "system_prompt.txt")
	tmpPrompt := filepath.Join(stateDir, fmt.Sprintf(".system_prompt-%d.tmp", time.Now().UnixNano()))
	defer os.Remove(tmpPrompt)
	if err := os.WriteFile(tmpPrompt, []byte(persona.SystemPrompt), 0644); err != nil {
		return fmt.Errorf("failed to write temp system_prompt file: %w", err)
	}
	if err := os.Rename(tmpPrompt, promptFile); err != nil {
		return fmt.Errorf("failed to atomically rename system_prompt file: %w", err)
	}

	// 2. Seed prompts_seed.json containing all available prompts
	prompts := s.ListPrompts("")
	promptsData, err := json.MarshalIndent(prompts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal prompts seed: %w", err)
	}
	seedFile := filepath.Join(stateDir, "prompts_seed.json")
	tmpSeed := filepath.Join(stateDir, fmt.Sprintf(".prompts_seed-%d.tmp", time.Now().UnixNano()))
	defer os.Remove(tmpSeed)
	if err := os.WriteFile(tmpSeed, promptsData, 0644); err != nil {
		return fmt.Errorf("failed to write temp prompts_seed file: %w", err)
	}
	if err := os.Rename(tmpSeed, seedFile); err != nil {
		return fmt.Errorf("failed to atomically rename prompts_seed file: %w", err)
	}

	return nil
}

// GetActive returns the current active persona ID and language.
func (s *PersonaStore) GetActive() (personaID string, lang string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeID, s.language
}

// SetActive updates the active persona and language preference, persisting to disk and seeding Open WebUI.
func (s *PersonaStore) SetActive(id string, lang string) (*Persona, error) {
	normID := NormalizePersonaID(id)

	def, exists := builtInPersonas[normID]
	if !exists {
		return nil, fmt.Errorf("invalid persona id: %s", id)
	}

	s.mu.Lock()
	s.activeID = normID
	if strings.TrimSpace(lang) != "" {
		s.language = NormalizeLanguage(lang)
	}
	currentLang := s.language
	s.mu.Unlock()

	if err := s.persist(); err != nil {
		s.logger.Warn("failed to persist persona state", "error", err)
		return nil, fmt.Errorf("failed to persist persona state: %w", err)
	}

	if err := s.SeedOpenWebUI(); err != nil {
		s.logger.Warn("failed to seed Open WebUI configuration", "error", err)
		return nil, fmt.Errorf("failed to seed Open WebUI configuration: %w", err)
	}

	persona := BuildPersona(def, currentLang, true)
	return &persona, nil
}

// GetPersona retrieves a single persona by ID with localized texts for requested language.
func (s *PersonaStore) GetPersona(id string, lang string) (*Persona, error) {
	normID := NormalizePersonaID(id)

	def, exists := builtInPersonas[normID]
	if !exists {
		return nil, fmt.Errorf("persona not found: %s", id)
	}

	s.mu.RLock()
	activeID := s.activeID
	defaultLang := s.language
	s.mu.RUnlock()

	targetLang := defaultLang
	if strings.TrimSpace(lang) != "" {
		targetLang = NormalizeLanguage(lang)
	}

	isActive := normID == activeID
	persona := BuildPersona(def, targetLang, isActive)
	return &persona, nil
}

// ListPersonas returns all personas in canonical order with localized texts for requested language.
func (s *PersonaStore) ListPersonas(lang string) []Persona {
	s.mu.RLock()
	activeID := s.activeID
	defaultLang := s.language
	s.mu.RUnlock()

	targetLang := defaultLang
	if strings.TrimSpace(lang) != "" {
		targetLang = NormalizeLanguage(lang)
	}

	result := make([]Persona, 0, len(personaOrder))
	for _, id := range personaOrder {
		def := builtInPersonas[id]
		isActive := id == activeID
		result = append(result, BuildPersona(def, targetLang, isActive))
	}

	return result
}

// ListPrompts returns all prompt templates across all personas, optionally filtered by language.
func (s *PersonaStore) ListPrompts(lang string) []PromptTemplate {
	templates := make([]PromptTemplate, 0)
	languages := []string{LangEN, LangDE, LangES, LangFR}

	if strings.TrimSpace(lang) != "" {
		languages = []string{NormalizeLanguage(lang)}
	}

	for _, l := range languages {
		for _, id := range personaOrder {
			def := builtInPersonas[id]
			p := BuildPersona(def, l, false)
			templates = append(templates, PromptTemplate{
				ID:           fmt.Sprintf("%s_%s", id, l),
				PersonaID:    id,
				Language:     l,
				Name:         p.Name,
				SystemPrompt: p.SystemPrompt,
			})
		}
	}

	return templates
}
