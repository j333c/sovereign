import { describe, it } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import SettingsPage, { formatBytes, formatSpeed, formatDuration } from "./page";
import { translations, Language } from "@/lib/i18n";
import {
  SovereignApiClient,
  AISettingsResponse,
  AIModelsResponse,
  AIModelDownloadProgress,
  AIInferenceLogsResponse,
} from "@/lib/api";

const defaultSettings: AISettingsResponse = {
  active_model: "qwen2.5-7b.gguf",
  gpu_layers: 33,
  context_length: 4096,
  idle_timeout_seconds: 300,
  flash_attention: true,
  tokens_per_sec: 24.5,
  vram_usage_bytes: 4800000000,
  ram_usage_bytes: 1200000000,
};

const defaultModels: AIModelsResponse = {
  active_model: "qwen2.5-7b.gguf",
  models: [
    {
      filename: "qwen2.5-7b.gguf",
      name: "Qwen 2.5 7B",
      path: "/data/models/qwen2.5-7b.gguf",
      size_bytes: 4700000000,
      modified_at: "2026-10-15T12:00:00Z",
      is_active: true,
      valid: true,
      ram_usage_bytes: 1200000000,
      vram_usage_bytes: 4800000000,
      tokens_per_sec: 24.5,
      flash_attention: true,
      header: {
        magic: "GGUF",
        version: 3,
        tensor_count: 338,
        metadata_kv_count: 28,
      },
    },
    {
      filename: "llama-3.2-3b.gguf",
      name: "Llama 3.2 3B",
      path: "/data/models/llama-3.2-3b.gguf",
      size_bytes: 2000000000,
      modified_at: "2026-10-14T10:00:00Z",
      is_active: false,
      valid: true,
      ram_usage_bytes: 800000000,
      vram_usage_bytes: 2000000000,
      tokens_per_sec: 45.0,
      flash_attention: true,
    },
  ],
};

const defaultDownloads: AIModelDownloadProgress[] = [
  {
    id: "dl-123",
    url: "https://huggingface.co/mistralai/Mistral-7B-Instruct-v0.3-GGUF/resolve/main/mistral-7b.gguf",
    filename: "mistral-7b.gguf",
    total_bytes: 4500000000,
    bytes_downloaded: 2250000000,
    percent: 50,
    status: "downloading",
    speed_bytes_per_sec: 15000000,
    eta_seconds: 150,
    created_at: "2026-10-16T12:00:00Z",
  },
];

const defaultLogs: AIInferenceLogsResponse = {
  logs: [
    {
      timestamp: "2026-10-16T14:30:00Z",
      level: "INFO",
      component: "ai-engine",
      message: "Generated 128 tokens in 1250ms",
      tokens_generated: 128,
      duration_ms: 1250,
      tokens_per_sec: 28.4,
    },
    {
      timestamp: "2026-10-16T14:32:00Z",
      level: "ERROR",
      component: "ai-engine",
      message: "Context length exceeded",
    },
  ],
};

function createMockApiClient(overrides?: Partial<SovereignApiClient>): SovereignApiClient {
  const mockClient = {
    getAISettings: async () => defaultSettings,
    getAIModels: async () => defaultModels,
    getAIModelDownloads: async () => ({ downloads: defaultDownloads }),
    getAIInferenceLogs: async () => defaultLogs,
    setActiveAIModel: async (filename: string) => ({
      status: "ok",
      active_model: filename,
    }),
    deleteAIModel: async () => ({ status: "ok", deleted: "test.gguf" }),
    updateAISettings: async (req: any) => ({ ...defaultSettings, ...req }),
    reloadAIEngine: async () => ({ status: "reloaded", message: "AI engine reloaded" }),
    downloadAIModel: async () => defaultDownloads[0],
    cancelAIModelDownload: async () => ({ status: "cancelled", id: "dl-123" }),
    ...overrides,
  } as unknown as SovereignApiClient;

  return mockClient;
}

describe("SettingsPage & AI Administration Interface", () => {
  it("formats bytes, speed, and duration correctly", () => {
    assert.equal(formatBytes(0), "0 B");
    assert.equal(formatBytes(1024), "1 KB");
    assert.equal(formatBytes(1048576), "1 MB");
    assert.equal(formatBytes(4700000000), "4.4 GB");

    assert.equal(formatSpeed(0), "0 B/s");
    assert.equal(formatSpeed(15000000), "14.3 MB/s");

    assert.equal(formatDuration(0), "0s");
    assert.equal(formatDuration(45), "45s");
    assert.equal(formatDuration(150), "2m 30s");
    assert.equal(formatDuration(3600), "60m 0s");
  });

  it("renders SettingsPage layout with header and sidebar navigation tabs", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="en"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("Back to Dashboard"), "renders Back to Dashboard link");
    assert.ok(html.includes("Services &amp; Add-ons") || html.includes("Services & Add-ons"), "renders Services tab");
    assert.ok(html.includes("AI Backend &amp; Models") || html.includes("AI Backend & Models"), "renders AI Backend tab");
    assert.ok(html.includes("Smart Hardware Sharing"), "renders Hardware Sharing tab");
    assert.ok(html.includes("System &amp; Hardware") || html.includes("System & Hardware"), "renders System tab");

    // Touch targets and focus visible rings
    assert.ok(html.includes("min-h-[44px]"), "ensures 44px minimum touch target height");
    assert.ok(html.includes("focus-visible:ring-2"), "has visible focus rings for keyboard navigation");
  });

  it("renders AI Inference Engine panel with header, badges, and active model card", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="en"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("AI Inference Engine (llama.cpp)"), "renders AI engine section title");
    assert.ok(html.includes("FlashAttention Active"), "renders FlashAttention status pill");
    assert.ok(html.includes("Default Setup Active") || html.includes("CPU-Optimized Mode"), "renders default setup or cpu badge");
    assert.ok(html.includes("Active Model"), "renders Active Model card title");
    assert.ok(html.includes("RAM Footprint"), "renders RAM footprint metric");
    assert.ok(html.includes("VRAM Footprint"), "renders VRAM footprint metric");
    assert.ok(html.includes("Inference Speed"), "renders Inference Speed metric");
  });

  it("renders parameter tuning controls (Presets, Context Length, GPU Offload, Idle Surrender)", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="en"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    // Generation Intent Presets
    assert.ok(html.includes("Generation Intent Preset"), "renders Generation Intent Preset control");
    assert.ok(html.includes("Precise &amp; Factual") || html.includes("Precise & Factual"), "renders Precise preset");
    assert.ok(html.includes("Balanced Assistant"), "renders Balanced Assistant preset");
    assert.ok(html.includes("Creative Drafting"), "renders Creative Drafting preset");

    // Context Length buttons
    assert.ok(html.includes("Context Window Size"), "renders Context Window Size control");
    assert.ok(html.includes(">2048<"), "renders 2048 context pill");
    assert.ok(html.includes(">4096<"), "renders 4096 context pill");
    assert.ok(html.includes(">8192<"), "renders 8192 context pill");

    // Sliders
    assert.ok(html.includes("GPU Layer Offload (-ngl)"), "renders GPU layers slider");
    assert.ok(html.includes('id="gpu-layers-slider"'), "renders GPU layers range input");
    assert.ok(html.includes("Dynamic Idle Surrender Timeout"), "renders Idle surrender timeout slider");
    assert.ok(html.includes('id="idle-timeout-slider"'), "renders Idle timeout range input");
  });

  it("renders Operational Action buttons (Reload Backend Engine, View Inference Logs)", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="en"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("Reload Backend Engine"), "renders Reload Backend Engine button");
    assert.ok(html.includes("View Inference Logs"), "renders View Inference Logs button");
  });

  it("renders Installed Models table and Download Manager form", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="en"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("Installed Models"), "renders Installed Models heading");
    assert.ok(html.includes("Model Name"), "renders Model Name column");
    assert.ok(html.includes("Download New Model"), "renders Download New Model heading");
    assert.ok(html.includes("Model Download URL (.gguf)"), "renders model URL input label");
    assert.ok(html.includes("Optional SHA-256 Checksum"), "renders SHA-256 input label");
    assert.ok(html.includes("Download Model"), "renders Download Model submit button");
  });

  it("renders accurately in German (de)", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="de"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("Zurück zum Dashboard"), "renders German back link");
    assert.ok(html.includes("KI-Backend &amp; Modelle") || html.includes("KI-Backend & Modelle"), "renders German AI tab");
    assert.ok(html.includes("KI-Inferenz-Engine (llama.cpp)"), "renders German section title");
    assert.ok(html.includes("FlashAttention aktiv"), "renders German FlashAttention pill");
    assert.ok(html.includes("Aktives Modell"), "renders German Active Model title");
    assert.ok(html.includes("Inferenzgeschwindigkeit"), "renders German speed label");
    assert.ok(html.includes("Installierte Modelle"), "renders German installed models title");
    assert.ok(html.includes("Neues Modell herunterladen"), "renders German download title");
  });

  it("renders accurately in Spanish (es)", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="es"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("Volver al panel"), "renders Spanish back link");
    assert.ok(html.includes("Backend de IA y modelos"), "renders Spanish AI tab");
    assert.ok(html.includes("Motor de inferencia de IA (llama.cpp)"), "renders Spanish section title");
    assert.ok(html.includes("FlashAttention activo"), "renders Spanish FlashAttention pill");
    assert.ok(html.includes("Modelo activo"), "renders Spanish Active Model title");
    assert.ok(html.includes("Descargar nuevo modelo"), "renders Spanish download title");
  });

  it("renders accurately in French (fr)", () => {
    const mock = createMockApiClient();
    const html = renderToStaticMarkup(
      <SettingsPage
        apiClientOverride={mock}
        initialLanguage="fr"
        initialTab="ai"
        initialAISettings={defaultSettings}
        initialModels={defaultModels.models}
        initialDownloads={defaultDownloads}
      />
    );

    assert.ok(html.includes("Retour au tableau de bord"), "renders French back link");
    assert.ok(html.includes("Moteur IA et modèles"), "renders French AI tab");
    assert.ok(html.includes("Moteur d&#x27;inférence IA (llama.cpp)") || html.includes("Moteur d'inférence IA"), "renders French section title");
    assert.ok(html.includes("FlashAttention actif"), "renders French FlashAttention pill");
    assert.ok(html.includes("Modèle actif"), "renders French Active Model title");
    assert.ok(html.includes("Télécharger un nouveau modèle"), "renders French download title");
  });

  it("renders other tabs (services, gpu, system) when selected", () => {
    const mock = createMockApiClient();

    const servicesHtml = renderToStaticMarkup(
      <SettingsPage apiClientOverride={mock} initialLanguage="en" initialTab="services" />
    );
    assert.ok(servicesHtml.includes("Video Streaming (Jellyfin)"), "renders Video service row in services tab");

    const gpuHtml = renderToStaticMarkup(
      <SettingsPage apiClientOverride={mock} initialLanguage="en" initialTab="gpu" />
    );
    assert.ok(gpuHtml.includes("Interactive AI (18h)"), "renders GPU arbitration schedule timeline");

    const systemHtml = renderToStaticMarkup(
      <SettingsPage apiClientOverride={mock} initialLanguage="en" initialTab="system" />
    );
    assert.ok(systemHtml.includes("Daily Update Window"), "renders Daily Update Window in system tab");
  });
});
