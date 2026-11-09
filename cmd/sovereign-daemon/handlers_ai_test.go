package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sovereign/internal/ai"
	"sovereign/internal/supervisor"
	"strings"
	"testing"
	"time"
)

// Story 5.1: Managed llama.cpp Backend & Appliance Boundary Guard Suite Tests

func TestServer_AIGateway_SafeStubMode(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Activate AI service without models: enters Safe Stub Mode (running + degraded)
	ctx := context.Background()
	_, err := srv.serviceMgr.ActivateService(ctx, "ai")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	// Queries to /v1/chat/completions without a model must return HTTP 503 RFC 7807 Problem Details
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"qwen2.5","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "problem+json") {
		t.Errorf("expected application/problem+json content-type, got: %s", ct)
	}

	var prob ai.SafeStubProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}
	if prob.Status != 503 {
		t.Errorf("expected status 503, got %d", prob.Status)
	}
	if prob.Title != "No AI Model Available" {
		t.Errorf("expected title 'No AI Model Available', got: %s", prob.Title)
	}
	if !strings.Contains(prob.Detail, "Settings") {
		t.Errorf("expected reassurance copy directing to Settings, got: %s", prob.Detail)
	}
}

func TestServer_Services_LlamaAliasesAndActivation(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	aliases := []string{"llama", "ai", "llm", "llama-server"}
	for _, alias := range aliases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/services/"+alias, nil)
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for alias %q, got %d: %s", alias, w.Code, w.Body.String())
		}

		var st supervisor.ServiceStatus
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatalf("failed to parse service status for %s: %v", alias, err)
		}
		if st.ID != supervisor.ServiceLlama {
			t.Errorf("expected resolved service ID %s for alias %s, got %s", supervisor.ServiceLlama, alias, st.ID)
		}
	}

	// Activate AI without models on disk: enters Safe Stub Mode (running + degraded) without crashing
	reqAct := httptest.NewRequest(http.MethodPost, "/api/v1/services/ai/activate", nil)
	wAct := httptest.NewRecorder()
	srv.router.ServeHTTP(wAct, reqAct)

	if wAct.Code != http.StatusOK {
		t.Fatalf("expected 200 OK activating ai in safe stub mode, got %d: %s", wAct.Code, wAct.Body.String())
	}

	var actSt supervisor.ServiceStatus
	_ = json.Unmarshal(wAct.Body.Bytes(), &actSt)
	if actSt.Status != supervisor.StatusRunning {
		t.Errorf("expected status running in safe stub mode, got %s", actSt.Status)
	}
	if actSt.Health != supervisor.HealthDegraded {
		t.Errorf("expected health degraded in safe stub mode, got %s", actSt.Health)
	}

	// Non-media service refresh returns 400 Bad Request
	reqRef := httptest.NewRequest(http.MethodPost, "/api/v1/services/ai/refresh", nil)
	wRef := httptest.NewRecorder()
	srv.router.ServeHTTP(wRef, reqRef)
	if wRef.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on AI library refresh, got %d: %s", wRef.Code, wRef.Body.String())
	}
}

func TestServer_AIGateway_ServiceInactiveProblem(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Ensure Llama is inactive
	ctx := context.Background()
	_, _ = srv.serviceMgr.DeactivateService(ctx, supervisor.ServiceLlama)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"default","messages":[{"role":"user","content":"hello"}]}`))
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d", w.Code)
	}

	var prob map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem json: %v", err)
	}
	if prob["type"] != "https://sovereign.local/problems/service-inactive" {
		t.Errorf("expected type https://sovereign.local/problems/service-inactive, got %v", prob["type"])
	}
	if prob["title"] != "AI Service Inactive" {
		t.Errorf("expected title 'AI Service Inactive', got %v", prob["title"])
	}
}

func createMockGGUFFile(t *testing.T, dir, filename string, version uint32) string {
	t.Helper()
	buf := new(bytes.Buffer)
	buf.Write([]byte{0x47, 0x47, 0x55, 0x46}) // "GGUF"
	_ = binary.Write(buf, binary.LittleEndian, version)
	_ = binary.Write(buf, binary.LittleEndian, uint64(100))
	_ = binary.Write(buf, binary.LittleEndian, uint64(10))

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to create mock GGUF file: %v", err)
	}
	return fullPath
}

func TestServer_AIGateway_ProxyWhenRunning(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Create mock upstream llama-server
	upstreamCalled := false
	mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected proxy path /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-123","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"Hello from mock llama"}}]}`))
	}))
	defer mockLlama.Close()

	// 2. Point gateway to mock upstream and create model in modelsDir
	if defGw, ok := srv.aiGateway.(*ai.DefaultGateway); ok {
		defGw.SetLlamaURL(mockLlama.URL)
	}
	modelsDir := filepath.Join(dataDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	createMockGGUFFile(t, modelsDir, "qwen2.5.gguf", 3)

	// 3. Activate Llama so it's running and healthy
	ctx := context.Background()
	st, err := srv.serviceMgr.ActivateService(ctx, supervisor.ServiceLlama)
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}
	if st.Status != supervisor.StatusRunning || st.Health != supervisor.HealthHealthy {
		t.Fatalf("expected running+healthy, got %s/%s", st.Status, st.Health)
	}

	// 4. Send request through router to /v1/chat/completions
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"qwen2.5","messages":[{"role":"user","content":"hi"}]}`))
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from proxy, got %d: %s", w.Code, w.Body.String())
	}
	if !upstreamCalled {
		t.Fatal("expected mock upstream to be called")
	}
	if !strings.Contains(w.Body.String(), "Hello from mock llama") {
		t.Errorf("expected response from mock llama, got: %s", w.Body.String())
	}
}

func TestServer_AIPresets_And_GovernorStatus(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Test GET /api/v1/ai/presets
	reqPresets := httptest.NewRequest(http.MethodGet, "/api/v1/ai/presets", nil)
	wPresets := httptest.NewRecorder()
	srv.router.ServeHTTP(wPresets, reqPresets)

	if wPresets.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /api/v1/ai/presets, got %d: %s", wPresets.Code, wPresets.Body.String())
	}

	var presetsResp struct {
		Presets []ai.Preset `json:"presets"`
	}
	if err := json.Unmarshal(wPresets.Body.Bytes(), &presetsResp); err != nil {
		t.Fatalf("failed to decode presets json: %v", err)
	}

	if len(presetsResp.Presets) != 3 {
		t.Fatalf("expected 3 presets, got %d", len(presetsResp.Presets))
	}
	if presetsResp.Presets[0].ID != "precise" || presetsResp.Presets[0].Temperature != 0.1 {
		t.Errorf("unexpected first preset: %+v", presetsResp.Presets[0])
	}

	// 2. Test GET /api/v1/ai/governor/status
	reqGov := httptest.NewRequest(http.MethodGet, "/api/v1/ai/governor/status", nil)
	wGov := httptest.NewRecorder()
	srv.router.ServeHTTP(wGov, reqGov)

	if wGov.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /api/v1/ai/governor/status, got %d: %s", wGov.Code, wGov.Body.String())
	}

	var govStatus ai.GovernorStatus
	if err := json.Unmarshal(wGov.Body.Bytes(), &govStatus); err != nil {
		t.Fatalf("failed to decode governor status: %v", err)
	}

	if govStatus.TotalSlots != 2 || govStatus.ReservedInteractive != 1 {
		t.Errorf("unexpected governor slots: %+v", govStatus)
	}

	// 3. Mock upstream llama-server to verify presets injection through daemon /v1/chat/completions
	var upstreamReceivedBody []byte
	mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamReceivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "ok"}}]}`))
	}))
	defer mockLlama.Close()

	if defGw, ok := srv.aiGateway.(*ai.DefaultGateway); ok {
		defGw.SetLlamaURL(mockLlama.URL)
	}
	modelsDir := filepath.Join(dataDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	createMockGGUFFile(t, modelsDir, "qwen2.5.gguf", 3)

	// Activate Llama
	_, err := srv.serviceMgr.ActivateService(context.Background(), supervisor.ServiceLlama)
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	reqChat := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"preset": "creative", "messages": [{"role": "user", "content": "draft a story"}]}`))
	wChat := httptest.NewRecorder()
	srv.router.ServeHTTP(wChat, reqChat)

	if wChat.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /v1/chat/completions, got %d: %s", wChat.Code, wChat.Body.String())
	}

	var upstreamJSON map[string]any
	if err := json.Unmarshal(upstreamReceivedBody, &upstreamJSON); err != nil {
		t.Fatalf("failed to parse upstream body: %v", err)
	}
	if upstreamJSON["temperature"] != 0.85 || upstreamJSON["top_p"] != 0.98 {
		t.Errorf("expected injected temp 0.85 and top_p 0.98, got temp=%v top_p=%v", upstreamJSON["temperature"], upstreamJSON["top_p"])
	}

	// Verify governor status updated
	wGovAfter := httptest.NewRecorder()
	srv.router.ServeHTTP(wGovAfter, reqGov)
	var govStatusAfter ai.GovernorStatus
	_ = json.Unmarshal(wGovAfter.Body.Bytes(), &govStatusAfter)
	if govStatusAfter.TotalServed != 1 {
		t.Errorf("expected 1 total served, got %d", govStatusAfter.TotalServed)
	}
}

func TestServer_AIStatusEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Query GET /api/v1/ai/status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/status", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /api/v1/ai/status, got %d: %s", w.Code, w.Body.String())
	}

	var status ai.ArbitratorStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode ai status: %v", err)
	}

	if status.State != ai.ArbitratorStateReady && status.State != ai.ArbitratorStateSafeStub {
		t.Errorf("unexpected initial state: %s", status.State)
	}
	if status.IdleTimeoutSeconds != 300 {
		t.Errorf("expected 300s idle timeout, got %d", status.IdleTimeoutSeconds)
	}

	// 2. Surrender VRAM and verify status reflects idle_surrendered
	if srv.aiGateway != nil {
		if srv.aiGateway.Arbitrator() != nil {
			srv.aiGateway.Arbitrator().SetState(ai.ArbitratorStateReady)
		}
		_ = srv.aiGateway.SurrenderIdleVRAM(context.Background())
		wAfter := httptest.NewRecorder()
		srv.router.ServeHTTP(wAfter, req)
		if wAfter.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 after surrender, got %d", wAfter.Code)
		}
		var statusAfter ai.ArbitratorStatus
		_ = json.Unmarshal(wAfter.Body.Bytes(), &statusAfter)
		if statusAfter.State != ai.ArbitratorStateIdleSurrendered {
			t.Errorf("expected state idle_surrendered, got %s", statusAfter.State)
		}
	}
}

func TestServer_AIIdleSurrender_SSEBroadcast(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	if srv.aiGateway == nil || srv.aiGateway.Arbitrator() == nil {
		t.Skip("AI gateway or arbitrator not enabled on test server")
	}

	// Set arbitrator state to ready so surrender triggers a state change to idle_surrendered
	srv.aiGateway.Arbitrator().SetState(ai.ArbitratorStateReady)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/system/events", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to open SSE connection: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Consume the initial telemetry.update event
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("error reading initial SSE event: %v", err)
		}
		if line == "\n" {
			break
		}
	}

	// Trigger surrender
	surrenderErr := srv.aiGateway.SurrenderIdleVRAM(context.Background())
	if surrenderErr != nil {
		t.Fatalf("unexpected error surrendering VRAM: %v", surrenderErr)
	}

	// Read lines until gpu.idle_surrender is seen
	var foundEvent bool
	var payload map[string]any
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("stream closed before gpu.idle_surrender received: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: gpu.idle_surrender") {
			foundEvent = true
		} else if foundEvent && strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				t.Fatalf("failed to parse gpu.idle_surrender data: %v", err)
			}
			break
		}
	}

	if !foundEvent {
		t.Fatalf("expected gpu.idle_surrender event to be broadcast")
	}
	if state, ok := payload["state"].(string); !ok || state != ai.ArbitratorStateIdleSurrendered {
		t.Errorf("expected state %s, got %v", ai.ArbitratorStateIdleSurrendered, payload["state"])
	}
}

func TestServer_AIPersonas_Endpoints(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. GET /api/v1/ai/personas (default English)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/personas", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/personas, got %d: %s", w.Code, w.Body.String())
	}

	var res ai.PersonaListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode personas response: %v", err)
	}

	if len(res.Personas) != 4 {
		t.Fatalf("expected 4 personas, got %d", len(res.Personas))
	}
	if res.ActiveID != "household" {
		t.Errorf("expected default active_id household, got %s", res.ActiveID)
	}
	if res.Language != "en" {
		t.Errorf("expected default language en, got %s", res.Language)
	}

	// 2. GET /api/v1/ai/personas?lang=de
	reqDE := httptest.NewRequest(http.MethodGet, "/api/v1/ai/personas?lang=de", nil)
	wDE := httptest.NewRecorder()
	srv.router.ServeHTTP(wDE, reqDE)

	if wDE.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/personas?lang=de, got %d: %s", wDE.Code, wDE.Body.String())
	}

	var resDE ai.PersonaListResponse
	if err := json.Unmarshal(wDE.Body.Bytes(), &resDE); err != nil {
		t.Fatalf("failed to decode personas response: %v", err)
	}
	foundLegal := false
	for _, p := range resDE.Personas {
		if p.ID == "legal_advisor" {
			foundLegal = true
			if p.Name != "Rechts- & Vertragsberater" {
				t.Errorf("expected German name 'Rechts- & Vertragsberater', got %q", p.Name)
			}
			if p.Preset != "precise" {
				t.Errorf("expected preset precise, got %s", p.Preset)
			}
		}
	}
	if !foundLegal {
		t.Errorf("legal_advisor not found in German persona list")
	}

	// 3. GET /api/v1/ai/personas/legal_advisor?lang=de
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/ai/personas/legal_advisor?lang=de", nil)
	wDetail := httptest.NewRecorder()
	srv.router.ServeHTTP(wDetail, reqDetail)

	if wDetail.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/personas/legal_advisor?lang=de, got %d: %s", wDetail.Code, wDetail.Body.String())
	}

	var legalDetail ai.Persona
	if err := json.Unmarshal(wDetail.Body.Bytes(), &legalDetail); err != nil {
		t.Fatalf("failed to decode legal_advisor detail: %v", err)
	}
	if legalDetail.ID != "legal_advisor" {
		t.Errorf("expected persona id legal_advisor, got %s", legalDetail.ID)
	}
	if !strings.Contains(legalDetail.SystemPrompt, "BGB § 312g") || !strings.Contains(legalDetail.SystemPrompt, "Gewährleistung") {
		t.Errorf("expected legal_advisor German prompt to mention BGB § 312g and Gewährleistung")
	}

	// 4. GET /api/v1/ai/personas/unknown_persona -> 404 Problem Details
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/ai/personas/unknown_persona", nil)
	w404 := httptest.NewRecorder()
	srv.router.ServeHTTP(w404, req404)

	if w404.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for unknown persona, got %d", w404.Code)
	}

	// 5. POST /api/v1/ai/personas/active with {"id": "legal_advisor", "language": "de"}
	activeBody := `{"id": "legal_advisor", "language": "de"}`
	reqActive := httptest.NewRequest(http.MethodPost, "/api/v1/ai/personas/active", strings.NewReader(activeBody))
	reqActive.Header.Set("Content-Type", "application/json")
	wActive := httptest.NewRecorder()
	srv.router.ServeHTTP(wActive, reqActive)

	if wActive.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from POST /api/v1/ai/personas/active, got %d: %s", wActive.Code, wActive.Body.String())
	}

	var updatedPersona ai.Persona
	if err := json.Unmarshal(wActive.Body.Bytes(), &updatedPersona); err != nil {
		t.Fatalf("failed to decode updated persona: %v", err)
	}
	if updatedPersona.ID != "legal_advisor" || updatedPersona.Language != "de" {
		t.Errorf("expected legal_advisor/de, got %s/%s", updatedPersona.ID, updatedPersona.Language)
	}
	if !updatedPersona.IsActive {
		t.Errorf("expected persona.IsActive to be true")
	}

	// Verify subsequent list query reflects legal_advisor as active
	wCheck := httptest.NewRecorder()
	srv.router.ServeHTTP(wCheck, req)
	var resCheck ai.PersonaListResponse
	_ = json.Unmarshal(wCheck.Body.Bytes(), &resCheck)
	if resCheck.ActiveID != "legal_advisor" {
		t.Errorf("expected active_id to be legal_advisor after update, got %s", resCheck.ActiveID)
	}

	// 6. POST /api/v1/ai/personas/active with invalid ID -> 400 Problem Details
	badBody := `{"id": "invalid_persona_xyz"}`
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/ai/personas/active", strings.NewReader(badBody))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	srv.router.ServeHTTP(wBad, reqBad)

	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for invalid persona, got %d", wBad.Code)
	}

	// 7. GET /api/v1/ai/prompts
	reqPrompts := httptest.NewRequest(http.MethodGet, "/api/v1/ai/prompts", nil)
	wPrompts := httptest.NewRecorder()
	srv.router.ServeHTTP(wPrompts, reqPrompts)

	if wPrompts.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/prompts, got %d: %s", wPrompts.Code, wPrompts.Body.String())
	}

	var promptsRes ai.PromptsResponse
	if err := json.Unmarshal(wPrompts.Body.Bytes(), &promptsRes); err != nil {
		t.Fatalf("failed to decode prompts response: %v", err)
	}
	if len(promptsRes.Prompts) != 16 {
		t.Errorf("expected 16 prompt templates, got %d", len(promptsRes.Prompts))
	}

	// 8. GET /api/v1/ai/prompts?lang=de
	reqPromptsDE := httptest.NewRequest(http.MethodGet, "/api/v1/ai/prompts?lang=de", nil)
	wPromptsDE := httptest.NewRecorder()
	srv.router.ServeHTTP(wPromptsDE, reqPromptsDE)

	if wPromptsDE.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/prompts?lang=de, got %d: %s", wPromptsDE.Code, wPromptsDE.Body.String())
	}

	var promptsResDE ai.PromptsResponse
	if err := json.Unmarshal(wPromptsDE.Body.Bytes(), &promptsResDE); err != nil {
		t.Fatalf("failed to decode German prompts response: %v", err)
	}
	if len(promptsResDE.Prompts) != 4 {
		t.Errorf("expected 4 German prompt templates, got %d", len(promptsResDE.Prompts))
	}
}

type mockDaemonRAGInferenceClient struct {
	answer string
	err    error
}

func (m *mockDaemonRAGInferenceClient) Complete(ctx context.Context, req ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	ans := m.answer
	if ans == "" {
		ans = "Your deductible for storm damage is $500, according to page 2 of Car Insurance Policy."
	}
	return &ai.ChatCompletionResponse{
		ID: "chatcmpl-test-daemon-rag",
		Choices: []ai.ChatCompletionChoice{
			{
				Index: 0,
				Message: ai.ChatCompletionMessage{
					Role:    "assistant",
					Content: ans,
				},
				FinishReason: "stop",
			},
		},
	}, nil
}

func setupPaperlessTestDB(t *testing.T, dataDir string) {
	t.Helper()
	docsDir := filepath.Join(dataDir, "documents", "data")
	_ = os.MkdirAll(docsDir, 0755)

	dbPath := filepath.Join(docsDir, "db.sqlite3")
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open test sqlite db: %v", err)
	}
	defer db.Close()

	createSQL := `
	CREATE TABLE IF NOT EXISTS documents_document (
		id INTEGER PRIMARY KEY,
		title VARCHAR(128) NOT NULL,
		content TEXT NOT NULL,
		created DATETIME,
		modified DATETIME
	);`
	if _, err := db.Exec(createSQL); err != nil {
		t.Fatalf("failed to create documents_document: %v", err)
	}

	doc1 := "Sovereign Insurance Services. Policy number: POL-98231.\n\x0cPage 2: Deductibles and Limits.\nThe deductible for storm damage and hail is $500.\nCollision deductible is $1000."
	doc2 := "Electricity Bill Q3 2026. Account ACC-44219. Total amount due: 184.50 EUR."

	_, err = db.Exec("INSERT INTO documents_document (id, title, content, created, modified) VALUES (1, 'Car Insurance Policy', ?, datetime('now'), datetime('now')), (2, 'Electricity Bill Q3', ?, datetime('now'), datetime('now'));", doc1, doc2)
	if err != nil {
		t.Fatalf("failed to insert test documents: %v", err)
	}
}

func TestServer_RAG_SearchAndQueryEndpoints(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	setupPaperlessTestDB(t, dataDir)

	modelsDir := filepath.Join(dataDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	createMockGGUFFile(t, modelsDir, "qwen2.5.gguf", 3)
	_, _ = srv.serviceMgr.ActivateService(context.Background(), supervisor.ServiceLlama)

	// Assert default InferenceClient configured by NewServer
	if srv.RAGEngine().InferenceClient() == nil {
		t.Fatalf("expected srv.RAGEngine() to have a default InferenceClient configured by NewServer")
	}

	mockInference := &mockDaemonRAGInferenceClient{
		answer: "Your deductible is $500, according to page 2 of Car Insurance Policy.",
	}
	srv.RAGEngine().SetInferenceClient(mockInference)

	// 1a. POST /api/v1/ai/rag/search with empty query -> HTTP 400
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/search", strings.NewReader(`{"query": "   "}`))
	reqEmpty.Header.Set("Content-Type", "application/json")
	wEmpty := httptest.NewRecorder()
	srv.router.ServeHTTP(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for empty search query, got %d", wEmpty.Code)
	}

	// 1b. POST /api/v1/ai/rag/search
	searchPayload := `{"query": "deductible on car insurance", "limit": 5, "language": "en"}`
	reqSearch := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/search", strings.NewReader(searchPayload))
	reqSearch.Header.Set("Content-Type", "application/json")
	wSearch := httptest.NewRecorder()
	srv.router.ServeHTTP(wSearch, reqSearch)

	if wSearch.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from RAG search, got %d: %s", wSearch.Code, wSearch.Body.String())
	}

	var searchRes ai.RAGSearchResponse
	if err := json.Unmarshal(wSearch.Body.Bytes(), &searchRes); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}

	if searchRes.TotalMatches == 0 {
		t.Fatalf("expected matches for 'deductible on car insurance', got 0")
	}
	if searchRes.Results[0].DocumentID != 1 {
		t.Errorf("expected doc ID 1, got %d", searchRes.Results[0].DocumentID)
	}
	if searchRes.Results[0].PageNumber != 2 {
		t.Errorf("expected page number 2, got %d", searchRes.Results[0].PageNumber)
	}
	if searchRes.Results[0].URL != "/docs/documents/1#page=2" {
		t.Errorf("expected URL '/docs/documents/1#page=2', got %s", searchRes.Results[0].URL)
	}

	// 2. POST /api/v1/ai/rag/query
	queryPayload := `{"query": "What is our deductible for storm damage on car insurance?", "language": "en"}`
	reqQuery := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/query", strings.NewReader(queryPayload))
	reqQuery.Header.Set("Content-Type", "application/json")
	wQuery := httptest.NewRecorder()
	srv.router.ServeHTTP(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from RAG query, got %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var queryRes ai.RAGQueryResponse
	if err := json.Unmarshal(wQuery.Body.Bytes(), &queryRes); err != nil {
		t.Fatalf("failed to decode query response: %v", err)
	}

	if !strings.Contains(queryRes.Answer, "$500") {
		t.Errorf("expected answer to contain '$500', got: %s", queryRes.Answer)
	}
	if len(queryRes.Citations) == 0 {
		t.Fatalf("expected citations in query response, got 0")
	}
	if queryRes.Citations[0].PageNumber != 2 {
		t.Errorf("expected citation page number 2, got %d", queryRes.Citations[0].PageNumber)
	}
	if queryRes.Truncated {
		t.Errorf("expected query not to be truncated")
	}
}

func TestServer_RAG_ContextExceedsHeadroom_GermanNotice(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	docsDir := filepath.Join(dataDir, "documents", "data")
	_ = os.MkdirAll(docsDir, 0755)
	dbPath := filepath.Join(docsDir, "db.sqlite3")
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open test sqlite db: %v", err)
	}
	defer db.Close()

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS documents_document (
		id INTEGER PRIMARY KEY,
		title VARCHAR(128) NOT NULL,
		content TEXT NOT NULL,
		created DATETIME,
		modified DATETIME
	);`)

	// Insert 25 large documents matching query
	heavyText := strings.Repeat("Klausel für Versicherungsvertrag und Hausratversicherung mit vielen Details. ", 60)
	for i := 1; i <= 25; i++ {
		_, _ = db.Exec("INSERT INTO documents_document (id, title, content, created, modified) VALUES (?, ?, ?, datetime('now'), datetime('now'));",
			i, fmt.Sprintf("Versicherungspolice Band %d", i), heavyText)
	}

	modelsDir := filepath.Join(dataDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	createMockGGUFFile(t, modelsDir, "qwen2.5.gguf", 3)
	_, _ = srv.serviceMgr.ActivateService(context.Background(), supervisor.ServiceLlama)

	mockInference := &mockDaemonRAGInferenceClient{
		answer: "Die Selbstbeteiligung beträgt 500 Euro laut Police.",
	}
	srv.RAGEngine().SetInferenceClient(mockInference)

	// POST /api/v1/ai/rag/query with context_tokens limited to 1200
	queryPayload := `{"query": "Versicherungspolice", "language": "de", "context_tokens": 1200}`
	reqQuery := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/query", strings.NewReader(queryPayload))
	reqQuery.Header.Set("Content-Type", "application/json")
	wQuery := httptest.NewRecorder()
	srv.router.ServeHTTP(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from RAG query exceeding headroom, got %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var queryRes ai.RAGQueryResponse
	if err := json.Unmarshal(wQuery.Body.Bytes(), &queryRes); err != nil {
		t.Fatalf("failed to decode query response: %v", err)
	}

	if !queryRes.Truncated {
		t.Errorf("expected truncated=true when excerpts exceed headroom")
	}

	expectedNoticeDE := "Hinweis: Dokumentenauszüge wurden priorisiert, um in das aktive Kontextfenster zu passen."
	if queryRes.Notice != expectedNoticeDE {
		t.Errorf("expected German notice %q, got %q", expectedNoticeDE, queryRes.Notice)
	}

	if !strings.Contains(queryRes.Answer, expectedNoticeDE) {
		t.Errorf("expected answer to contain German notice, got: %s", queryRes.Answer)
	}
}

func TestServer_RAG_UnprovisionedDatabase(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Search against empty/unprovisioned database
	searchPayload := `{"query": "insurance", "language": "en"}`
	reqSearch := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/search", strings.NewReader(searchPayload))
	reqSearch.Header.Set("Content-Type", "application/json")
	wSearch := httptest.NewRecorder()
	srv.router.ServeHTTP(wSearch, reqSearch)

	if wSearch.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from unprovisioned search, got %d: %s", wSearch.Code, wSearch.Body.String())
	}

	var searchRes ai.RAGSearchResponse
	_ = json.Unmarshal(wSearch.Body.Bytes(), &searchRes)
	if searchRes.TotalMatches != 0 {
		t.Errorf("expected 0 matches, got %d", searchRes.TotalMatches)
	}
	expectedNotice := "Document archive database is unprovisioned or empty."
	if searchRes.Notice != expectedNotice {
		t.Errorf("expected notice %q, got %q", expectedNotice, searchRes.Notice)
	}

	// 2. Query against empty/unprovisioned database
	queryPayload := `{"query": "insurance", "language": "en"}`
	reqQuery := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/query", strings.NewReader(queryPayload))
	reqQuery.Header.Set("Content-Type", "application/json")
	wQuery := httptest.NewRecorder()
	srv.router.ServeHTTP(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from unprovisioned query, got %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var queryRes ai.RAGQueryResponse
	_ = json.Unmarshal(wQuery.Body.Bytes(), &queryRes)
	if len(queryRes.Citations) != 0 {
		t.Errorf("expected 0 citations, got %d", len(queryRes.Citations))
	}
	if queryRes.Notice != expectedNotice {
		t.Errorf("expected notice %q, got %q", expectedNotice, queryRes.Notice)
	}
}

func TestServer_RAG_SafeStub_WhenNoModel(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	setupPaperlessTestDB(t, dataDir)

	// models directory is empty (no model provisioned)
	queryPayload := `{"query": "deductible on car insurance", "language": "en"}`
	reqQuery := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rag/query", strings.NewReader(queryPayload))
	reqQuery.Header.Set("Content-Type", "application/json")
	wQuery := httptest.NewRecorder()
	srv.router.ServeHTTP(wQuery, reqQuery)

	if wQuery.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 Safe Stub when no model provisioned, got %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var prob ProblemDetails
	_ = json.Unmarshal(wQuery.Body.Bytes(), &prob)
	if prob.Status != http.StatusServiceUnavailable {
		t.Errorf("expected problem status 503, got %d", prob.Status)
	}
}

func TestServer_AI_Models_ListAndActive(t *testing.T) {
	srv, tmpDir := setupTestServer(t)
	defer srv.Close()

	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	createMockGGUFFile(t, modelsDir, "qwen2.5-7b.gguf", 3)
	createMockGGUFFile(t, modelsDir, "llama3-8b.gguf", 3)
	_ = os.Symlink("qwen2.5-7b.gguf", filepath.Join(modelsDir, "active.gguf"))

	// 1. GET /api/v1/ai/models
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/ai/models", nil)
	wList := httptest.NewRecorder()
	srv.router.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/models, got %d: %s", wList.Code, wList.Body.String())
	}

	var listResp ai.AIModelsResponse
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}

	if listResp.ActiveModel != "qwen2.5-7b.gguf" {
		t.Errorf("expected active model 'qwen2.5-7b.gguf', got %q", listResp.ActiveModel)
	}
	if len(listResp.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(listResp.Models))
	}

	// 2. POST /api/v1/ai/models/active with non-existent model -> 404
	missingPayload := `{"filename": "missing.gguf"}`
	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/ai/models/active", strings.NewReader(missingPayload))
	reqMissing.Header.Set("Content-Type", "application/json")
	wMissing := httptest.NewRecorder()
	srv.router.ServeHTTP(wMissing, reqMissing)

	if wMissing.Code != http.StatusNotFound {
		t.Errorf("expected HTTP 404 for missing model, got %d: %s", wMissing.Code, wMissing.Body.String())
	}

	// 3. POST /api/v1/ai/models/active with valid model -> 200
	switchPayload := `{"filename": "llama3-8b.gguf"}`
	reqSwitch := httptest.NewRequest(http.MethodPost, "/api/v1/ai/models/active", strings.NewReader(switchPayload))
	reqSwitch.Header.Set("Content-Type", "application/json")
	wSwitch := httptest.NewRecorder()
	srv.router.ServeHTTP(wSwitch, reqSwitch)

	if wSwitch.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for model switch, got %d: %s", wSwitch.Code, wSwitch.Body.String())
	}

	// Verify active model was updated
	reqList2 := httptest.NewRequest(http.MethodGet, "/api/v1/ai/models", nil)
	wList2 := httptest.NewRecorder()
	srv.router.ServeHTTP(wList2, reqList2)

	var listResp2 ai.AIModelsResponse
	_ = json.Unmarshal(wList2.Body.Bytes(), &listResp2)
	if listResp2.ActiveModel != "llama3-8b.gguf" {
		t.Errorf("expected active model 'llama3-8b.gguf', got %q", listResp2.ActiveModel)
	}
}

func TestServer_AI_Models_Delete(t *testing.T) {
	srv, tmpDir := setupTestServer(t)
	defer srv.Close()

	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	createMockGGUFFile(t, modelsDir, "active-model.gguf", 3)
	createMockGGUFFile(t, modelsDir, "inactive-model.gguf", 3)
	_ = os.Symlink("active-model.gguf", filepath.Join(modelsDir, "active.gguf"))

	// 1. DELETE active model -> 409 Conflict
	reqDelActive := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/active-model.gguf", nil)
	wDelActive := httptest.NewRecorder()
	srv.router.ServeHTTP(wDelActive, reqDelActive)

	if wDelActive.Code != http.StatusConflict {
		t.Errorf("expected HTTP 409 when deleting active model, got %d: %s", wDelActive.Code, wDelActive.Body.String())
	}

	// 2. DELETE inactive model -> 200 OK
	reqDelInactive := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/inactive-model.gguf", nil)
	wDelInactive := httptest.NewRecorder()
	srv.router.ServeHTTP(wDelInactive, reqDelInactive)

	if wDelInactive.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 when deleting inactive model, got %d: %s", wDelInactive.Code, wDelInactive.Body.String())
	}

	if _, err := os.Stat(filepath.Join(modelsDir, "inactive-model.gguf")); !os.IsNotExist(err) {
		t.Errorf("expected inactive-model.gguf to be deleted")
	}
}

func TestServer_AI_Settings_GetAndUpdate(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. GET /api/v1/ai/settings
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/ai/settings", nil)
	wGet := httptest.NewRecorder()
	srv.router.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/settings, got %d: %s", wGet.Code, wGet.Body.String())
	}

	var settings ai.AISettings
	if err := json.Unmarshal(wGet.Body.Bytes(), &settings); err != nil {
		t.Fatalf("failed to decode settings: %v", err)
	}

	if settings.ContextLength != 4096 {
		t.Errorf("expected default ContextLength 4096, got %d", settings.ContextLength)
	}
	if settings.IdleTimeoutSeconds != 300 {
		t.Errorf("expected default IdleTimeoutSeconds 300, got %d", settings.IdleTimeoutSeconds)
	}
	if !settings.FlashAttention {
		t.Errorf("expected default FlashAttention true, got %v", settings.FlashAttention)
	}

	// 2. POST /api/v1/ai/settings
	updatePayload := `{"gpu_layers": 33, "context_length": 8192, "idle_timeout_seconds": 600}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/ai/settings", strings.NewReader(updatePayload))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	srv.router.ServeHTTP(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from POST /api/v1/ai/settings, got %d: %s", wPost.Code, wPost.Body.String())
	}

	var updated ai.AISettings
	if err := json.Unmarshal(wPost.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to decode updated settings: %v", err)
	}

	if updated.GPULayers != 33 {
		t.Errorf("expected GPULayers 33, got %d", updated.GPULayers)
	}
	if updated.ContextLength != 8192 {
		t.Errorf("expected ContextLength 8192, got %d", updated.ContextLength)
	}
	if updated.IdleTimeoutSeconds != 600 {
		t.Errorf("expected IdleTimeoutSeconds 600, got %d", updated.IdleTimeoutSeconds)
	}
}

func TestServer_AI_ReloadAndLogs(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. POST /api/v1/ai/reload
	reqReload := httptest.NewRequest(http.MethodPost, "/api/v1/ai/reload", nil)
	wReload := httptest.NewRecorder()
	srv.router.ServeHTTP(wReload, reqReload)

	if wReload.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from POST /api/v1/ai/reload, got %d: %s", wReload.Code, wReload.Body.String())
	}

	// 2. GET /api/v1/ai/logs
	reqLogs := httptest.NewRequest(http.MethodGet, "/api/v1/ai/logs", nil)
	wLogs := httptest.NewRecorder()
	srv.router.ServeHTTP(wLogs, reqLogs)

	if wLogs.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET /api/v1/ai/logs, got %d: %s", wLogs.Code, wLogs.Body.String())
	}

	var logsResp ai.AIInferenceLogsResponse
	if err := json.Unmarshal(wLogs.Body.Bytes(), &logsResp); err != nil {
		t.Fatalf("failed to decode logs response: %v", err)
	}

	if len(logsResp.Logs) == 0 {
		t.Errorf("expected at least one log entry")
	}
}

func TestServer_AI_Models_DownloadAndCancel(t *testing.T) {
	srv, tmpDir := setupTestServer(t)
	defer srv.Close()

	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	blockCh := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-blockCh:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(blockCh)
		ts.Close()
	}()
	srv.ModelManager().SetDiskFree(func(path string) (int64, error) {
		return 50 * 1024 * 1024 * 1024, nil
	})

	// 1. POST /api/v1/ai/models/download
	dlPayload := fmt.Sprintf(`{"url": "%s/test-model.gguf", "filename": "test-model.gguf"}`, ts.URL)
	reqDl := httptest.NewRequest(http.MethodPost, "/api/v1/ai/models/download", strings.NewReader(dlPayload))
	reqDl.Header.Set("Content-Type", "application/json")
	wDl := httptest.NewRecorder()
	srv.router.ServeHTTP(wDl, reqDl)

	if wDl.Code != http.StatusAccepted {
		t.Fatalf("expected HTTP 202 from POST download, got %d: %s", wDl.Code, wDl.Body.String())
	}

	var prog ai.AIModelDownloadProgress
	if err := json.Unmarshal(wDl.Body.Bytes(), &prog); err != nil {
		t.Fatalf("failed to decode download progress: %v", err)
	}

	if prog.ID == "" {
		t.Errorf("expected non-empty download job ID")
	}

	// 2. GET /api/v1/ai/models/downloads
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/ai/models/downloads", nil)
	wList := httptest.NewRecorder()
	srv.router.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from GET downloads, got %d: %s", wList.Code, wList.Body.String())
	}

	// 3. DELETE /api/v1/ai/models/downloads/{id}
	reqCancel := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/downloads/"+prog.ID, nil)
	wCancel := httptest.NewRecorder()
	srv.router.ServeHTTP(wCancel, reqCancel)

	if wCancel.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from cancel download, got %d: %s", wCancel.Code, wCancel.Body.String())
	}

	var cancelledProg ai.AIModelDownloadProgress
	if err := json.Unmarshal(wCancel.Body.Bytes(), &cancelledProg); err != nil {
		t.Fatalf("failed to decode cancelled progress: %v", err)
	}
	if cancelledProg.Status != "cancelled" {
		t.Errorf("expected download status 'cancelled', got %q", cancelledProg.Status)
	}

	// 4. Unknown download job ID should return 404
	reqUnknown := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/downloads/nonexistent-id", nil)
	wUnknown := httptest.NewRecorder()
	srv.router.ServeHTTP(wUnknown, reqUnknown)
	if wUnknown.Code != http.StatusNotFound {
		t.Errorf("expected HTTP 404 for nonexistent download job, got %d", wUnknown.Code)
	}
}

func TestServer_AI_IdleTimeout_RestoredOnBoot(t *testing.T) {
	srv, dataDir := setupTestServer(t)

	// Persist 600s in DB
	if err := srv.db.SetSetting(context.Background(), "ai_idle_timeout_seconds", "600"); err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}
	srv.Close()

	mockEngine := supervisor.NewMockRuntimeEngine()
	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
	})

	srv2, err := NewServer(Config{
		DataDir:    dataDir,
		ServiceMgr: svcMgr,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv2.Close()

	if srv2.aiGateway != nil && srv2.aiGateway.Arbitrator() != nil {
		status := srv2.aiGateway.Arbitrator().Status()
		if status.IdleTimeoutSeconds != 600 {
			t.Errorf("expected restored arbitrator idle timeout 600s, got %d", status.IdleTimeoutSeconds)
		}
	}
}

func TestModelDownload_InsufficientStorage(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Mock HTTP server serving a 4.0 GB model
	const modelSize int64 = 4 * 1024 * 1024 * 1024 // 4 GB
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", modelSize))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// 2. Set available disk space to 8.0 GB
	// Requires modelSize (4 GB) + SafetyBufferBytes (5 GB) = 9 GB total.
	// Since 8 GB < 9 GB, pre-flight check must abort with 507 Insufficient Storage.
	const freeSpace int64 = 8 * 1024 * 1024 * 1024 // 8 GB
	srv.ModelManager().SetDiskFree(func(path string) (int64, error) {
		return freeSpace, nil
	})

	modelsDir := filepath.Join(dataDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	// 3. Request model download
	payload := fmt.Sprintf(`{"url": "%s/large-model.gguf", "filename": "large-model.gguf"}`, mockServer.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/models/download", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	// 4. Assert HTTP 507 Insufficient Storage
	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected HTTP 507 Insufficient Storage, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/problem+json") {
		t.Errorf("expected Content-Type application/problem+json, got %q", contentType)
	}

	// 5. Assert RFC 7807 problem details
	var prob ProblemDetails
	if err := json.NewDecoder(w.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}
	if prob.Type != "https://sovereign.local/errors/insufficient-storage" {
		t.Errorf("expected problem type 'https://sovereign.local/errors/insufficient-storage', got %q", prob.Type)
	}
	if prob.Status != http.StatusInsufficientStorage {
		t.Errorf("expected problem status 507, got %d", prob.Status)
	}
	if prob.Title != "Insufficient Storage" {
		t.Errorf("expected problem title 'Insufficient Storage', got %q", prob.Title)
	}

	// 6. Assert zero partial files or clutter in models directory
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		t.Fatalf("failed to read models directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files in models dir after pre-flight rejection, found %d entries", len(entries))
		for _, e := range entries {
			t.Errorf("unexpected file in models directory: %s", e.Name())
		}
	}

	partialFile := filepath.Join(modelsDir, "large-model.gguf.downloading")
	if _, err := os.Stat(partialFile); err == nil {
		t.Errorf("unexpected partial download file %q exists on disk", partialFile)
	}
}
