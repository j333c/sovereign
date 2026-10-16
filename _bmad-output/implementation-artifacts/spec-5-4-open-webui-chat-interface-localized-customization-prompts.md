---
title: 'Story 5.4: Open WebUI Chat Interface & Localized Customization Prompts'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: '01dfcae2aad7a38b9e2ab094b6889adf2290e031'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Family members accessing the local AI require an intuitive, friendly, responsive chat interface accessible on smartphones, tablets, and laptops that operates 100% offline without telemetry or third-party cloud leaks. Raw generic chat interfaces lack understanding of local household workflows, and default system prompts fail to account for regional language nuances and contractual/legal norms (e.g. statutory cancellation rights, warranty obligations, formal notice formats).

**Approach:** Implement turnkey Open WebUI chat integration and a Localized Customization Prompt & Persona engine:
1. **On-Demand Open WebUI Container Spoke (`internal/supervisor/openwebui.go`, `supervisor.go`)**:
   - Pinned official image `ghcr.io/open-webui/open-webui:0.12.0`.
   - Starts on-demand and connects directly to the internal AI Gateway (`http://sovereign-daemon:8080/v1`) inside the isolated `sovereign-ai` / `sovereign-ingress` network.
   - Accessible via both dedicated port `:3001` and subpath `/chat` routed through Caddy with prefix stripping.
   - Zero Docker socket mounts (`/var/run/docker.sock` strictly prohibited).
   - Strict telemetry suppression: `SCARF_NO_ANALYTICS=True`, `DO_NOT_TRACK=True`, `ANONYMIZED_TELEMETRY=False`, `ENABLE_COMMUNITY_SHARING=False`, `WEBUI_AUTH=False`.
2. **Localized Customization Prompts & Persona Engine (`internal/ai/personas.go`)**:
   - Built-in personas tailored to household domains:
     - `HouseholdAssistant`: Everyday household helper, task organization, meal planning, general questions.
     - `LegalContractAdvisor`: Specialized in German / EU / regional consumer rights (e.g. BGB § 312g 14-day statutory revocation, warranty terms, cancellation deadlines, formal objection letters).
     - `CreativeWriter`: Brainstorming, storytelling, family celebration speeches, imaginative writing.
     - `TechnicalSupport`: Practical step-by-step troubleshooting for home networking, electronics, and household appliances.
   - Pre-configured localized system prompts supporting 4 languages: English (`en`), German (`de`), Spanish (`es`), and French (`fr`).
   - Seeding and storage in `/data/state/services/openwebui` and API endpoints in `sovereign-daemon`:
     - `GET /api/v1/ai/personas`: List personas with localized names, descriptions, and generation presets (`precise`, `balanced`, `creative`).
     - `GET /api/v1/ai/personas/{id}`: Detailed persona information with localized system prompt.
     - `GET /api/v1/ai/prompts`: Localized system prompt templates across languages.
     - `POST /api/v1/ai/personas/active`: Set active default persona.
3. **OpenAPI & Frontend Integration**:
   - Expose persona and chat routing in `api/openapi.yaml`.
   - TypeScript client methods `getPersonas(lang?: string)` and `getPersona(id: string, lang?: string)`.
   - Dashboard AI ServiceCard action button directly opens `/chat` in a new tab.

## Boundaries & Constraints

**Always:**
- Keep Open WebUI container strictly air-gapped from cloud analytics, tracking, and telemetry (`DO_NOT_TRACK=True`, `SCARF_NO_ANALYTICS=True`, `ANONYMIZED_TELEMETRY=False`).
- Route all inference from Open WebUI through the supervisor daemon's mediated AI Gateway (`:8080/v1`), never directly to raw llama-server.
- Support dual-access routing: dedicated port `:3001` and subpath `/chat` via Caddy.
- Provide localized system prompts for all 4 supported languages (EN, DE, ES, FR) incorporating regional legal and contractual norms.
- Respect container memory limits (1024MB limit, 512MB reservation).
- Support zero Docker socket mounts in Open WebUI container spec.

**Never:**
- Never publish raw inference ports directly to the host network without gateway mediation.
- Never mount the Docker socket into the Open WebUI container.
- Never emit external telemetry, error reports, or telemetry pings to third-party endpoints.
- Never drop chat history or user customizations on service restart (`/data/state/services/openwebui` persistent volume).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Open WebUI Activation | Admin or UI requests `openwebui` / `chat` activation | On-demand image pull and container startup; connects to AI gateway; ingress routes synced | Emits `service.error` if startup fails |
| Dual-Port Ingress | Browser requests `http://sovereign.local/chat` or `http://sovereign.local:3001` | Caddy proxies request to `sovereign-openwebui:3001` | Returns 502/503 if Open WebUI container is inactive |
| Persona List Query | `GET /api/v1/ai/personas?lang=de` | Returns 4 personas with German names, descriptions, and regional legal prompts | Defaults to English if requested language is unsupported |
| Persona Detail Query | `GET /api/v1/ai/personas/legal_advisor?lang=de` | Returns Legal Advisor persona with German BGB § 312g, Kündigungsfristen, and precise preset | Returns HTTP 404 Problem Details if persona ID unknown |
| Set Active Persona | `POST /api/v1/ai/personas/active` with `{"id": "legal_advisor", "language": "de"}` | Updates active persona preference in state store; seeds Open WebUI config | Rejects invalid persona IDs with HTTP 400 |
| Empty Models Directory | Open WebUI queries AI Gateway while in Safe Stub Mode | Gateway returns 503 RFC 7807 Problem Details; Open WebUI displays friendly error notice | No crash; prompts user to download model in Settings |
| Telemetry Verification | Network inspections during active chat session | Zero outbound packets to external IPs or domains | Strict container isolation |

</frozen-after-approval>

## Code Map

- `internal/supervisor/openwebui.go` -- Open WebUI provisioner, container spec synthesis, environment setup (zero telemetry), and persistence directory management.
- `internal/supervisor/supervisor.go` -- Register `ServiceOpenWebUI`, alias resolution (`chat`, `open-webui`, `openwebui`), and lifecycle hooks.
- `internal/ai/personas.go` -- Persona definitions, localized system prompts across EN/DE/ES/FR, legal/contractual norms, and persona store.
- `cmd/sovereign-daemon/server.go` -- Endpoints `GET /api/v1/ai/personas`, `GET /api/v1/ai/personas/{id}`, `POST /api/v1/ai/personas/active`.
- `api/openapi.yaml` -- OpenAPI schemas for Persona, PersonaListResponse, and SetActivePersonaRequest.
- `caddy/Caddyfile` -- Ingress route configuration for `/chat` and `:3001`.
- `internal/ingress/caddy_test.go` -- Ingress tests covering `/chat` subpath and `:3001` dedicated port routing.
- `web/src/lib/api/client.ts` -- TypeScript API client methods for AI personas.
- `web/src/lib/i18n.ts` -- Localized persona names and descriptions.

## Implementation Plan

1. **Persona Engine (`internal/ai/personas.go`, `internal/ai/personas_test.go`)**:
   - Define `Persona` and `PersonaPrompt` structs.
   - Implement localized personas (`household`, `legal_advisor`, `creative`, `tech_support`) across EN, DE, ES, FR.
   - Implement `PersonaStore` for persisting active persona selection.
   - Add unit tests validating persona retrieval, localization fallback, and legal prompt contents.
2. **Open WebUI Provisioner (`internal/supervisor/openwebui.go`, `internal/supervisor/openwebui_test.go`)**:
   - Implement `OpenWebUIProvisioner` with persistent directory `/data/state/services/openwebui`.
   - Synthesize environment variables: `OPENAI_API_BASE_URL`, `OPENAI_API_KEY`, `PORT=3001`, `WEBUI_AUTH=False`, `SCARF_NO_ANALYTICS=True`, `DO_NOT_TRACK=True`, `ANONYMIZED_TELEMETRY=False`.
   - Register `ServiceOpenWebUI` in `supervisor.go`.
   - Add unit tests verifying container spec, ports, volumes, and telemetry blocking.
3. **Daemon HTTP Endpoints & Ingress (`cmd/sovereign-daemon/server.go`, `caddy/Caddyfile`)**:
   - Register `/api/v1/ai/personas`, `/api/v1/ai/personas/{id}`, and `/api/v1/ai/personas/active`.
   - Wire Caddy ingress routes for `/chat` and port 3001.
   - Add server tests in `server_test.go`.
4. **OpenAPI & Frontend Client (`api/openapi.yaml`, `web/`)**:
   - Update OpenAPI YAML and generate TypeScript client types (`npm run codegen`).
   - Add `getPersonas` and `getActivePersona` in `SovereignApiClient`.
   - Verify web test suite passes (`npm --prefix web test`).
5. **Automated Verification**:
   - Run `go test ./...` and `npm --prefix web test`.

## Review Triage Log

- **Review Cycle**: Iteration 1
- **Reviewers**:
  - Blind Hunter Reviewer (`2bbffe84-681b-4304-af96-4eea04b3b323`)
  - Edge Case Hunter Reviewer (`9c67e6df-4f68-43db-ae30-946f6d3e1f34`)
  - Verification Gap Reviewer (`7395e5eb-c640-4c4a-ba54-00d550ca4b88`)

### Findings Triaged & Remediations Applied

1. **PersonaStore Thread Safety & Disk Persistence (`internal/ai/personas.go`)**:
   - *Finding*: `persist()` and `load()` accessed `s.activeID`, `s.language`, and `s.stateDir` without synchronization across locks. `SetActive()` returned `nil` on persistence failure.
   - *Fix*: Protected `load()` with write mutex, protected `persist()` reads with read mutex, propagated errors from `persist()` and `SeedOpenWebUI()`, and added `defer os.Remove(...)` to clean up temporary files reliably on write or rename failures.
2. **Open WebUI Air-Gap Hardening & System Prompt Injection (`internal/supervisor/openwebui.go`)**:
   - *Finding*: Default Open WebUI startup emitted background version checks, Ollama connectivity probes, and web search checks. System prompts were not automatically surfaced to new chat sessions.
   - *Fix*: Injected `ENABLE_VERSION_CHECK=False`, `ENABLE_OLLAMA_API=False`, `ENABLE_SEARCH_QUERY=False`, and `WEBUI_BUILD_HASH=""`. Added `DEFAULT_SYSTEM_PROMPT` injection from `system_prompt.txt` into container environment so chat sessions immediately start with the localized persona prompt.
3. **Caddy `/chat` Ingress Redirect (`caddy/Caddyfile`)**:
   - *Finding*: Requesting `http://sovereign.local/chat` without trailing slash stripped the prefix to an empty URI, breaking relative assets and upstream routes.
   - *Fix*: Added `redir /chat /chat/ permanent` in Caddyfile and scoped `@chat path /chat/*`.
4. **Lifecycle Coordination for AI Bundle (`internal/supervisor/supervisor.go`)**:
   - *Finding*: Activating Open WebUI independently could leave Llama stopped, producing immediate 503 errors on chat inference.
   - *Fix*: Added AI bundle coordination in `ActivateService`, `DeactivateService`, and `RestartService`: activating `"ai"` coordinates both `llama` and `openwebui`, and activating `openwebui` ensures `llama` is started.
5. **REST API Problem Details Standardization (`cmd/sovereign-daemon/server.go`)**:
   - *Finding*: Persona handlers wrote ad-hoc JSON errors rather than using the daemon's standardized `writeProblem` helper, and failed to default missing `lang` to `"en"`.
   - *Fix*: Refactored `handleListAIPersonas`, `handleGetAIPersona`, and `handleSetActiveAIPersona` to use `s.writeProblem` and `s.writeJSON`, defaulting empty `lang` to `"en"`.
6. **Dashboard Multi-Spoke Status Priority Aggregation (`web/src/app/page.tsx`, `page.test.tsx`)**:
   - *Finding*: Inactive `openwebui` spoke unconditionally overwrote running `llama` backend status on dashboard load.
   - *Fix*: Implemented priority status ranking (`error > degraded > starting > healthy > stopped`) in `getServices()` and guarded `handleServiceHealthChanged` against inactive auxiliary spoke downgrades. Added test in `web/src/app/page.test.tsx`.

