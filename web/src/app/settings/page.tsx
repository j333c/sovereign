"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { Language, translations } from "@/lib/i18n";
import {
  apiClient as defaultApiClient,
  SovereignApiClient,
  AIModelInfo,
  AIModelDownloadProgress,
  AISettingsResponse,
  AIInferenceLogEntry,
  ProblemDetailsError,
} from "@/lib/api";

export interface SettingsPageProps {
  apiClientOverride?: SovereignApiClient;
  initialLanguage?: Language;
  initialTab?: string;
  initialAuthenticated?: boolean;
  initialAISettings?: AISettingsResponse | null;
  initialModels?: AIModelInfo[];
  initialDownloads?: AIModelDownloadProgress[];
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export function formatSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec <= 0) return "0 B/s";
  return `${formatBytes(bytesPerSec)}/s`;
}

export function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return "0s";
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  if (mins > 0) {
    return `${mins}m ${secs}s`;
  }
  return `${secs}s`;
}

export default function SettingsPage({
  apiClientOverride,
  initialLanguage = "en",
  initialTab,
  initialAISettings = null,
  initialModels = [],
  initialDownloads = [],
}: SettingsPageProps) {
  const client = apiClientOverride || defaultApiClient;
  const [lang, setLang] = useState<Language>(initialLanguage);
  const t = translations[lang];

  // Tab navigation state
  const [activeTab, setActiveTab] = useState<string>(initialTab || "ai");

  // AI Administration state
  const [aiSettings, setAiSettings] = useState<AISettingsResponse | null>(
    initialAISettings
  );
  const [models, setModels] = useState<AIModelInfo[]>(initialModels);
  const [downloads, setDownloads] = useState<AIModelDownloadProgress[]>(
    initialDownloads
  );
  const [inferenceLogs, setInferenceLogs] = useState<AIInferenceLogEntry[]>([]);
  const [loadingAI, setLoadingAI] = useState<boolean>(true);
  const [activatingModel, setActivatingModel] = useState<string | null>(null);
  const [isReloading, setIsReloading] = useState<boolean>(false);
  const [reloadNotice, setReloadNotice] = useState<string | null>(null);
  const [showLogsModal, setShowLogsModal] = useState<boolean>(false);
  const [modelToDelete, setModelToDelete] = useState<string | null>(null);
  const [generationPreset, setGenerationPreset] = useState<string>("balanced");

  // Download form state
  const [downloadUrl, setDownloadUrl] = useState<string>("");
  const [downloadSha256, setDownloadSha256] = useState<string>("");
  const [downloadFilename, setDownloadFilename] = useState<string>("");
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const [isStartingDownload, setIsStartingDownload] = useState<boolean>(false);

  // Focus trap / dialog refs
  const logsModalRef = useRef<HTMLDivElement>(null);
  const deleteModalRef = useRef<HTMLDivElement>(null);
  const navRef = useRef<HTMLElement>(null);

  // Hash synchronization
  useEffect(() => {
    if (typeof window !== "undefined") {
      const handleHashChange = () => {
        const hash = window.location.hash.replace("#", "");
        if (hash === "ai" || hash === "ai-backend") {
          setActiveTab("ai");
        } else if (hash === "services") {
          setActiveTab("services");
        } else if (hash === "system") {
          setActiveTab("system");
        } else if (hash === "gpu") {
          setActiveTab("gpu");
        }
      };

      if (!initialTab && window.location.hash) {
        handleHashChange();
      }

      window.addEventListener("hashchange", handleHashChange);
      return () => window.removeEventListener("hashchange", handleHashChange);
    }
  }, [initialTab]);

  const selectTab = (tabId: string) => {
    setActiveTab(tabId);
    if (typeof window !== "undefined") {
      window.location.hash = tabId === "ai" ? "ai" : tabId;
    }
  };

  // Keyboard navigation for sidebar tabs (ArrowUp / ArrowDown)
  const handleNavKeyDown = (e: React.KeyboardEvent<HTMLElement>) => {
    const tabs = ["services", "ai", "gpu", "system"];
    const currentIndex = tabs.indexOf(activeTab);
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIndex = (currentIndex + 1) % tabs.length;
      selectTab(tabs[nextIndex]);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIndex = (currentIndex - 1 + tabs.length) % tabs.length;
      selectTab(tabs[prevIndex]);
    }
  };

  // Fetch AI engine state
  const loadAIState = useCallback(async () => {
    try {
      setLoadingAI(true);
      const [settingsRes, modelsRes, downloadsRes] = await Promise.all([
        client.getAISettings().catch(() => null),
        client.getAIModels().catch(() => null),
        client.getAIModelDownloads().catch(() => ({ downloads: [] })),
      ]);

      if (settingsRes) {
        setAiSettings(settingsRes);
      }
      if (modelsRes) {
        setModels(modelsRes.models || []);
      }
      if (downloadsRes) {
        setDownloads(downloadsRes.downloads || []);
      }
    } finally {
      setLoadingAI(false);
    }
  }, [client]);

  useEffect(() => {
    if (activeTab === "ai") {
      void loadAIState();
    }
  }, [activeTab, loadAIState]);

  // Polling for active downloads
  useEffect(() => {
    const hasActiveDownloads = downloads.some(
      (d) => d.status === "downloading" || d.status === "verifying"
    );

    if (!hasActiveDownloads) return;

    const interval = setInterval(async () => {
      try {
        const [updatedDownloads, updatedModels] = await Promise.all([
          client.getAIModelDownloads(),
          client.getAIModels(),
        ]);
        setDownloads(updatedDownloads?.downloads || []);
        if (updatedModels) {
          setModels(updatedModels.models || []);
        }
      } catch {
        // Suppress polling error
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [downloads, client]);

  // Model switching handler
  const handleSwitchModel = async (filename: string) => {
    try {
      setActivatingModel(filename);
      await client.setActiveAIModel(filename);
      await loadAIState();
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      } else if (err instanceof Error) {
        alert(err.message);
      }
    } finally {
      setActivatingModel(null);
    }
  };

  // Model deletion handler
  const handleDeleteModel = async () => {
    if (!modelToDelete) return;
    try {
      await client.deleteAIModel(modelToDelete);
      setModelToDelete(null);
      await loadAIState();
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      } else if (err instanceof Error) {
        alert(err.message);
      }
    }
  };

  // Download submit handler
  const handleStartDownload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!downloadUrl.trim()) return;

    try {
      setIsStartingDownload(true);
      setDownloadError(null);
      await client.downloadAIModel({
        url: downloadUrl.trim(),
        sha256: downloadSha256.trim() || undefined,
        filename: downloadFilename.trim() || undefined,
      });

      setDownloadUrl("");
      setDownloadSha256("");
      setDownloadFilename("");
      const activeDownloads = await client.getAIModelDownloads();
      setDownloads(activeDownloads?.downloads || []);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        if (err.status === 507) {
          setDownloadError(
            t.storageWarning
              .replace("{modelSize}", "4.7 GB")
              .replace("{requiredSpace}", "9.7 GB")
          );
        } else {
          setDownloadError(err.detail || err.title);
        }
      } else if (err instanceof Error) {
        setDownloadError(err.message);
      } else {
        setDownloadError("Download failed to initiate");
      }
    } finally {
      setIsStartingDownload(false);
    }
  };

  // Cancel download handler
  const handleCancelDownload = async (id: string) => {
    try {
      await client.cancelAIModelDownload(id);
      const activeDownloads = await client.getAIModelDownloads();
      setDownloads(activeDownloads?.downloads || []);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      }
    }
  };

  // Parameter update handlers
  const handleContextLengthChange = async (length: number) => {
    if (!aiSettings || aiSettings.context_length === length) return;
    try {
      const updated = await client.updateAISettings({ context_length: length });
      setAiSettings(updated);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      }
    }
  };

  const handleGpuLayersChange = async (layers: number) => {
    if (!aiSettings) return;
    try {
      const updated = await client.updateAISettings({ gpu_layers: layers });
      setAiSettings(updated);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      }
    }
  };

  const handleIdleTimeoutChange = async (seconds: number) => {
    if (!aiSettings) return;
    try {
      const updated = await client.updateAISettings({
        idle_timeout_seconds: seconds,
      });
      setAiSettings(updated);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      }
    }
  };

  const handlePresetChange = (preset: string) => {
    setGenerationPreset(preset);
  };

  // Reload engine handler
  const handleReloadEngine = async () => {
    try {
      setIsReloading(true);
      setReloadNotice(null);
      await client.reloadAIEngine();
      setReloadNotice(t.reloadSuccess);
      await loadAIState();
      setTimeout(() => setReloadNotice(null), 5000);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        alert(err.detail || err.title);
      } else if (err instanceof Error) {
        alert(err.message);
      }
    } finally {
      setIsReloading(false);
    }
  };

  // Open inference logs modal
  const handleOpenLogs = async () => {
    try {
      const res = await client.getAIInferenceLogs();
      setInferenceLogs(res.logs || []);
    } catch {
      setInferenceLogs([]);
    } finally {
      setShowLogsModal(true);
    }
  };

  // Escape key handler for dialogs
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (showLogsModal) setShowLogsModal(false);
        if (modelToDelete) setModelToDelete(null);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [showLogsModal, modelToDelete]);

  const activeModelObj = models.find((m) => m.is_active) || null;
  const isCpuOnly = !aiSettings || (aiSettings.vram_usage_bytes === 0 && aiSettings.gpu_layers === 0);

  return (
    <div className="flex min-h-screen flex-col bg-[#F8F9FA] text-[#0F172A]">
      {/* 64px Header */}
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={setLang}
        aggregateStatus="operational"
      />

      {/* Main Settings Layout */}
      <div className="mx-auto flex w-full max-w-[1200px] flex-1 flex-col gap-6 px-4 py-8 sm:px-6 sm:py-10 md:grid md:grid-cols-[240px_1fr] md:gap-8">
        {/* Sidebar Navigation */}
        <aside>
          <div className="mb-4">
            <Link
              href="/"
              className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-[#0F172A] shadow-xs hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              aria-label={t.backToDashboard}
            >
              <span aria-hidden="true">←</span>
              <span>{t.backToDashboard}</span>
            </Link>
          </div>

          <nav
            ref={navRef}
            onKeyDown={handleNavKeyDown}
            className="flex flex-col gap-1.5"
            aria-label="Settings categories"
          >
            <button
              type="button"
              onClick={() => selectTab("services")}
              className={`flex min-h-[44px] items-center gap-2.5 rounded-lg px-3.5 py-2.5 text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                activeTab === "services"
                  ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                  : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
              }`}
              aria-current={activeTab === "services" ? "page" : undefined}
            >
              <span aria-hidden="true">🧩</span>
              <span>{t.tabServices}</span>
            </button>

            <button
              type="button"
              onClick={() => selectTab("ai")}
              className={`flex min-h-[44px] items-center gap-2.5 rounded-lg px-3.5 py-2.5 text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                activeTab === "ai"
                  ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                  : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
              }`}
              aria-current={activeTab === "ai" ? "page" : undefined}
            >
              <span aria-hidden="true">🤖</span>
              <span>{t.tabAI}</span>
            </button>

            <button
              type="button"
              onClick={() => selectTab("gpu")}
              className={`flex min-h-[44px] items-center gap-2.5 rounded-lg px-3.5 py-2.5 text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                activeTab === "gpu"
                  ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                  : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
              }`}
              aria-current={activeTab === "gpu" ? "page" : undefined}
            >
              <span aria-hidden="true">⚡</span>
              <span>{t.gpuSharing}</span>
            </button>

            <button
              type="button"
              onClick={() => selectTab("system")}
              className={`flex min-h-[44px] items-center gap-2.5 rounded-lg px-3.5 py-2.5 text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                activeTab === "system"
                  ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                  : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
              }`}
              aria-current={activeTab === "system" ? "page" : undefined}
            >
              <span aria-hidden="true">⚙️</span>
              <span>{t.tabSystem}</span>
            </button>
          </nav>
        </aside>

        {/* Content Pane */}
        <main className="flex flex-col gap-6">
          {/* TAB: AI Backend & Models */}
          {activeTab === "ai" && (
            <div className="flex flex-col gap-6" data-testid="ai-settings-panel">
              {/* Card Header & Badges */}
              <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
                <div className="flex flex-wrap items-start justify-between gap-4 pb-4 border-b border-[#F1F5F9]">
                  <div>
                    <h1 className="text-lg font-semibold tracking-tight text-[#0F172A]">
                      {t.aiEngineTitle}
                    </h1>
                    <p className="mt-1 text-xs text-[#64748B]">
                      {t.aiEngineSubtitle}
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {/* FlashAttention Pill */}
                    {aiSettings?.flash_attention && (
                      <span
                        className="inline-flex items-center gap-1.5 rounded-full border border-[#BBF7D0] bg-[#F0FDF4] px-2.5 py-1 text-xs font-semibold text-[#166534]"
                        data-testid="flash-attention-pill"
                      >
                        <span aria-hidden="true">⚡</span>
                        {t.flashAttentionPill}
                      </span>
                    )}

                    {/* Default Turnkey / CPU badge */}
                    {isCpuOnly ? (
                      <span
                        className="inline-flex items-center gap-1.5 rounded-full border border-[#CBD5E1] bg-[#F8F9FA] px-2.5 py-1 text-xs font-semibold text-[#0F172A]"
                        data-testid="cpu-optimized-badge"
                      >
                        <span aria-hidden="true" className="text-gray-500">■</span>
                        {t.cpuOptimizedBadge}
                      </span>
                    ) : (
                      <span
                        className="inline-flex items-center gap-1.5 rounded-full border border-[#A7F3D0] bg-[#ECFDF5] px-2.5 py-1 text-xs font-semibold text-[#065F46]"
                        data-testid="default-setup-badge"
                      >
                        <span aria-hidden="true" className="text-emerald-500">●</span>
                        {t.defaultSetupBadge}
                      </span>
                    )}
                  </div>
                </div>

                {/* Reload Notice Banner */}
                {reloadNotice && (
                  <div
                    className="mt-4 rounded-lg border border-[#BBF7D0] bg-[#F0FDF4] p-3 text-xs text-[#166534] font-medium flex items-center justify-between"
                    role="status"
                    data-testid="reload-notice"
                  >
                    <span>{reloadNotice}</span>
                    <button
                      type="button"
                      onClick={() => setReloadNotice(null)}
                      className="text-[#166534] hover:text-[#065F46] p-1 font-bold"
                      aria-label="Dismiss notice"
                    >
                      ✕
                    </button>
                  </div>
                )}

                {/* Active Model Summary Card */}
                <div
                  className="mt-6 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-4"
                  data-testid="active-model-card"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="text-xs font-semibold text-[#64748B] uppercase tracking-wider">
                      {t.activeModelCardTitle}
                    </span>
                    <span className="text-sm font-bold text-[#0F172A]">
                      {aiSettings?.active_model || activeModelObj?.filename || t.noActiveModel}
                    </span>
                  </div>

                  <div className="mt-4 grid grid-cols-1 sm:grid-cols-3 gap-4 border-t border-[#E5E7EB] pt-4">
                    <div>
                      <span className="block text-xs text-[#64748B]">
                        {t.ramFootprint}
                      </span>
                      <span className="text-sm font-semibold text-[#0F172A]" data-testid="ram-footprint">
                        {formatBytes(aiSettings?.ram_usage_bytes || 0)}
                      </span>
                    </div>

                    <div>
                      <span className="block text-xs text-[#64748B]">
                        {t.vramFootprint}
                      </span>
                      <span className="text-sm font-semibold text-[#0F172A]" data-testid="vram-footprint">
                        {formatBytes(aiSettings?.vram_usage_bytes || 0)}
                      </span>
                    </div>

                    <div>
                      <span className="block text-xs text-[#64748B]">
                        {t.inferenceSpeed}
                      </span>
                      <span className="text-sm font-semibold text-[#0F172A]" data-testid="tokens-per-sec">
                        {aiSettings?.tokens_per_sec
                          ? `${aiSettings.tokens_per_sec.toFixed(1)} ${t.tokensPerSec}`
                          : `0.0 ${t.tokensPerSec}`}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Parameter Tuning Controls */}
                <div className="mt-6 divide-y divide-[#F1F5F9]">
                  {/* Generation Intent Preset */}
                  <div className="py-4 flex flex-wrap items-center justify-between gap-4">
                    <div>
                      <span className="text-sm font-medium text-[#0F172A]">
                        {t.generationPresetLabel}
                      </span>
                      <p className="text-xs text-[#64748B]">
                        {t.aiDesc}
                      </p>
                    </div>

                    <div
                      className="inline-flex rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-1 gap-1"
                      role="radiogroup"
                      aria-label={t.generationPresetLabel}
                    >
                      <button
                        type="button"
                        onClick={() => handlePresetChange("precise")}
                        className={`min-h-[36px] px-3 py-1.5 text-xs font-medium rounded-md transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                          generationPreset === "precise"
                            ? "bg-[#0F172A] text-white shadow-xs font-semibold"
                            : "text-[#0F172A] hover:bg-white"
                        }`}
                        role="radio"
                        aria-checked={generationPreset === "precise"}
                      >
                        {t.presetPrecise}
                      </button>

                      <button
                        type="button"
                        onClick={() => handlePresetChange("balanced")}
                        className={`min-h-[36px] px-3 py-1.5 text-xs font-medium rounded-md transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                          generationPreset === "balanced"
                            ? "bg-[#0F172A] text-white shadow-xs font-semibold"
                            : "text-[#0F172A] hover:bg-white"
                        }`}
                        role="radio"
                        aria-checked={generationPreset === "balanced"}
                      >
                        {t.presetBalanced}
                      </button>

                      <button
                        type="button"
                        onClick={() => handlePresetChange("creative")}
                        className={`min-h-[36px] px-3 py-1.5 text-xs font-medium rounded-md transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                          generationPreset === "creative"
                            ? "bg-[#0F172A] text-white shadow-xs font-semibold"
                            : "text-[#0F172A] hover:bg-white"
                        }`}
                        role="radio"
                        aria-checked={generationPreset === "creative"}
                      >
                        {t.presetCreative}
                      </button>
                    </div>
                  </div>

                  {/* Context Length Selector */}
                  <div className="py-4 flex flex-wrap items-center justify-between gap-4">
                    <div>
                      <span className="text-sm font-medium text-[#0F172A]">
                        {t.contextLengthLabel}
                      </span>
                      <p className="text-xs text-[#64748B]">
                        {t.contextLengthDesc.replace(
                          "{mem}",
                          aiSettings?.context_length === 8192
                            ? "3.2 GB"
                            : aiSettings?.context_length === 2048
                            ? "0.8 GB"
                            : "1.6 GB"
                        )}
                      </p>
                    </div>

                    <div
                      className="inline-flex rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-1 gap-1"
                      role="radiogroup"
                      aria-label={t.contextLengthLabel}
                    >
                      {[2048, 4096, 8192].map((len) => (
                        <button
                          key={len}
                          type="button"
                          onClick={() => handleContextLengthChange(len)}
                          className={`min-h-[36px] px-3.5 py-1.5 text-xs font-medium rounded-md transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                            aiSettings?.context_length === len
                              ? "bg-[#0F172A] text-white shadow-xs font-semibold"
                              : "text-[#0F172A] hover:bg-white"
                          }`}
                          role="radio"
                          aria-checked={aiSettings?.context_length === len}
                        >
                          {len}
                        </button>
                      ))}
                    </div>
                  </div>

                  {/* GPU Offload Layers Slider */}
                  <div className="py-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="text-sm font-medium text-[#0F172A]">
                          {t.gpuLayersLabel}
                        </span>
                        <p className="text-xs text-[#64748B]">
                          {isCpuOnly ? t.cpuOnlyLocked : t.gpuLayersDesc}
                        </p>
                      </div>

                      <span
                        className="text-xs font-semibold text-[#0F172A]"
                        data-testid="gpu-layers-value"
                      >
                        {isCpuOnly
                          ? "0 layers (CPU)"
                          : `${aiSettings?.gpu_layers ?? 0} layers`}
                      </span>
                    </div>

                    <div className="mt-3 flex items-center gap-3">
                      <input
                        type="range"
                        id="gpu-layers-slider"
                        min="0"
                        max="99"
                        disabled={isCpuOnly}
                        value={isCpuOnly ? 0 : aiSettings?.gpu_layers ?? 0}
                        onChange={(e) =>
                          handleGpuLayersChange(parseInt(e.target.value, 10))
                        }
                        className="w-full h-2 bg-[#E2E8F0] rounded-lg appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                        aria-label={t.gpuLayersLabel}
                      />
                    </div>
                  </div>

                  {/* Dynamic Idle Memory Surrender Slider */}
                  <div className="py-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="text-sm font-medium text-[#0F172A]">
                          {t.idleTimeoutLabel}
                        </span>
                        <p className="text-xs text-[#64748B]">
                          {t.idleTimeoutDesc.replace(
                            "{minutes}",
                            String(
                              Math.round(
                                (aiSettings?.idle_timeout_seconds ?? 300) / 60
                              )
                            )
                          )}
                        </p>
                      </div>

                      <span
                        className="text-xs font-semibold text-[#0F172A]"
                        data-testid="idle-timeout-value"
                      >
                        {Math.round(
                          (aiSettings?.idle_timeout_seconds ?? 300) / 60
                        )}{" "}
                        min
                      </span>
                    </div>

                    <div className="mt-3 flex items-center gap-3">
                      <input
                        type="range"
                        id="idle-timeout-slider"
                        min="60"
                        max="3600"
                        step="60"
                        value={aiSettings?.idle_timeout_seconds ?? 300}
                        onChange={(e) =>
                          handleIdleTimeoutChange(parseInt(e.target.value, 10))
                        }
                        className="w-full h-2 bg-[#E2E8F0] rounded-lg appearance-none cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                        aria-label={t.idleTimeoutLabel}
                      />
                    </div>
                  </div>
                </div>

                {/* Operational Actions */}
                <div className="mt-6 pt-4 border-t border-[#F1F5F9] flex flex-wrap gap-3">
                  <button
                    type="button"
                    onClick={handleReloadEngine}
                    disabled={isReloading}
                    className="min-h-[44px] inline-flex items-center justify-center gap-2 rounded-lg border border-[#CBD5E1] bg-[#F8F9FA] px-4 py-2 text-xs sm:text-sm font-semibold text-[#0F172A] hover:bg-[#F1F5F9] disabled:opacity-50 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors shadow-xs"
                    aria-label={t.reloadEngine}
                    data-testid="reload-engine-button"
                  >
                    {isReloading && (
                      <span className="inline-block animate-spin">⟳</span>
                    )}
                    <span>{isReloading ? t.reloadingEngine : t.reloadEngine}</span>
                  </button>

                  <button
                    type="button"
                    onClick={handleOpenLogs}
                    className="min-h-[44px] inline-flex items-center justify-center gap-2 rounded-lg border border-[#CBD5E1] bg-white px-4 py-2 text-xs sm:text-sm font-medium text-[#475569] hover:bg-[#F8F9FA] hover:text-[#0F172A] cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors shadow-xs"
                    aria-label={t.viewInferenceLogs}
                    data-testid="view-logs-button"
                  >
                    <span>📜</span>
                    <span>{t.viewInferenceLogs}</span>
                  </button>
                </div>
              </section>

              {/* Installed Models Table */}
              <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
                <div className="flex items-center justify-between pb-4 border-b border-[#F1F5F9]">
                  <h2 className="text-base font-semibold text-[#0F172A]">
                    {t.installedModelsTitle}
                  </h2>
                  <span className="text-xs text-[#64748B]">
                    {models.length} {models.length === 1 ? "model" : "models"}
                  </span>
                </div>

                {models.length === 0 ? (
                  <div className="py-8 text-center text-xs text-[#64748B]">
                    {t.noModelsInstalled}
                  </div>
                ) : (
                  <div className="overflow-x-auto mt-4">
                    <table className="w-full text-left text-xs text-[#0F172A]">
                      <thead>
                        <tr className="border-b border-[#E5E7EB] text-[#64748B] font-medium">
                          <th className="pb-3 pr-4">{t.modelNameCol}</th>
                          <th className="pb-3 px-4">{t.modelSizeCol}</th>
                          <th className="pb-3 px-4">{t.modelModifiedCol}</th>
                          <th className="pb-3 px-4">{t.modelStatusCol}</th>
                          <th className="pb-3 pl-4 text-right">{t.modelActionsCol}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-[#F1F5F9]">
                        {models.map((model) => (
                          <tr key={model.filename} className="hover:bg-[#F8F9FA]">
                            <td className="py-3 pr-4 font-medium text-[#0F172A]">
                              <div className="flex flex-col">
                                <span className="font-semibold">{model.name || model.filename}</span>
                                {model.header?.tensor_count ? (
                                  <span className="text-[11px] text-[#64748B]">
                                    {model.header.tensor_count} tensors · {model.filename}
                                  </span>
                                ) : (
                                  <span className="text-[11px] text-[#64748B]">
                                    {model.filename}
                                  </span>
                                )}
                              </div>
                            </td>
                            <td className="py-3 px-4 text-[#64748B]">
                              {formatBytes(model.size_bytes)}
                            </td>
                            <td className="py-3 px-4 text-[#64748B]">
                              {model.modified_at
                                ? new Date(model.modified_at).toLocaleDateString()
                                : "—"}
                            </td>
                            <td className="py-3 px-4">
                              {model.is_active ? (
                                <span
                                  className="inline-flex items-center gap-1 rounded-full bg-[#ECFDF5] border border-[#A7F3D0] px-2 py-0.5 text-[11px] font-semibold text-[#065F46]"
                                  data-testid={`active-badge-${model.filename}`}
                                >
                                  <span aria-hidden="true">●</span>
                                  {t.activeBadge}
                                </span>
                              ) : (
                                <span className="text-[#64748B] text-[11px]">
                                  {t.inactive}
                                </span>
                              )}
                            </td>
                            <td className="py-3 pl-4 text-right">
                              <div className="flex items-center justify-end gap-2">
                                {!model.is_active && (
                                  <button
                                    type="button"
                                    onClick={() => handleSwitchModel(model.filename)}
                                    disabled={activatingModel === model.filename}
                                    className="min-h-[36px] inline-flex items-center justify-center rounded-md border border-[#CBD5E1] bg-white px-3 py-1 text-xs font-semibold text-[#0F172A] hover:bg-[#F1F5F9] disabled:opacity-50 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
                                    aria-label={`${t.activateModel} ${model.filename}`}
                                    data-testid={`activate-button-${model.filename}`}
                                  >
                                    {activatingModel === model.filename
                                      ? t.activatingModel
                                      : t.activateModel}
                                  </button>
                                )}

                                <button
                                  type="button"
                                  onClick={() => setModelToDelete(model.filename)}
                                  disabled={model.is_active}
                                  className="min-h-[36px] inline-flex items-center justify-center rounded-md border border-[#FECACA] bg-white px-3 py-1 text-xs font-medium text-[#DC2626] hover:bg-[#FEF2F2] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
                                  title={model.is_active ? "Cannot delete active model" : t.deleteModel}
                                  aria-label={`${t.deleteModel} ${model.filename}`}
                                  data-testid={`delete-button-${model.filename}`}
                                >
                                  {t.deleteModel}
                                </button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>

              {/* Download Manager Card */}
              <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
                <div className="pb-4 border-b border-[#F1F5F9]">
                  <h2 className="text-base font-semibold text-[#0F172A]">
                    {t.downloadManagerTitle}
                  </h2>
                  <p className="mt-1 text-xs text-[#64748B]">
                    {t.downloadManagerSubtitle}
                  </p>
                </div>

                {/* Storage warning banner */}
                {downloadError && (
                  <div
                    className="mt-4 rounded-lg border border-[#FDE68A] bg-[#FFFBEB] p-3 text-xs text-[#B45309] font-medium flex items-center justify-between"
                    role="alert"
                    data-testid="storage-warning-banner"
                  >
                    <span>{downloadError}</span>
                    <button
                      type="button"
                      onClick={() => setDownloadError(null)}
                      className="text-[#B45309] hover:text-[#92400E] p-1 font-bold"
                      aria-label="Dismiss error"
                    >
                      ✕
                    </button>
                  </div>
                )}

                {/* Download Form */}
                <form onSubmit={handleStartDownload} className="mt-4 flex flex-col gap-4">
                  <div>
                    <label
                      htmlFor="download-url"
                      className="block text-xs font-semibold text-[#0F172A] mb-1"
                    >
                      {t.modelUrlLabel}
                    </label>
                    <input
                      id="download-url"
                      type="url"
                      required
                      value={downloadUrl}
                      onChange={(e) => setDownloadUrl(e.target.value)}
                      placeholder={t.modelUrlPlaceholder}
                      className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] bg-white px-3 py-2 text-xs sm:text-sm text-[#0F172A] placeholder-[#94A3B8] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                    />
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div>
                      <label
                        htmlFor="download-sha256"
                        className="block text-xs font-semibold text-[#0F172A] mb-1"
                      >
                        {t.sha256Label}
                      </label>
                      <input
                        id="download-sha256"
                        type="text"
                        value={downloadSha256}
                        onChange={(e) => setDownloadSha256(e.target.value)}
                        placeholder={t.sha256Placeholder}
                        className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] bg-white px-3 py-2 text-xs sm:text-sm text-[#0F172A] placeholder-[#94A3B8] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                      />
                    </div>

                    <div>
                      <label
                        htmlFor="download-filename"
                        className="block text-xs font-semibold text-[#0F172A] mb-1"
                      >
                        {t.customFilenameLabel}
                      </label>
                      <input
                        id="download-filename"
                        type="text"
                        value={downloadFilename}
                        onChange={(e) => setDownloadFilename(e.target.value)}
                        placeholder="model.gguf"
                        className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] bg-white px-3 py-2 text-xs sm:text-sm text-[#0F172A] placeholder-[#94A3B8] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                      />
                    </div>
                  </div>

                  <div className="flex justify-end">
                    <button
                      type="submit"
                      disabled={isStartingDownload || !downloadUrl.trim()}
                      className="min-h-[44px] inline-flex items-center justify-center gap-2 rounded-lg bg-[#0F172A] px-5 py-2.5 text-xs sm:text-sm font-semibold text-white hover:bg-[#1E293B] disabled:opacity-50 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors shadow-xs"
                      aria-label={t.startDownload}
                      data-testid="start-download-button"
                    >
                      {isStartingDownload ? t.downloadingState : t.startDownload}
                    </button>
                  </div>
                </form>

                {/* Active / Past Downloads List */}
                {downloads.length > 0 && (
                  <div className="mt-6 border-t border-[#F1F5F9] pt-4 flex flex-col gap-3">
                    <h3 className="text-xs font-semibold text-[#64748B] uppercase tracking-wider">
                      Downloads
                    </h3>

                    {downloads.map((item) => (
                      <div
                        key={item.id}
                        className="rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-3 text-xs"
                        data-testid={`download-item-${item.id}`}
                      >
                        <div className="flex items-center justify-between font-semibold text-[#0F172A]">
                          <span className="truncate max-w-[200px] sm:max-w-md">
                            {item.filename}
                          </span>
                          <span className="text-[11px] uppercase font-bold text-[#64748B]">
                            {item.status} ({item.percent}%)
                          </span>
                        </div>

                        {/* Progress Bar */}
                        <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-[#E2E8F0]">
                          <div
                            className={`h-full transition-all duration-300 ${
                              item.status === "failed"
                                ? "bg-red-500"
                                : item.status === "completed"
                                ? "bg-emerald-500"
                                : "bg-blue-600"
                            }`}
                            style={{ width: `${item.percent}%` }}
                          />
                        </div>

                        <div className="mt-2 flex items-center justify-between text-[11px] text-[#64748B]">
                          <div className="flex items-center gap-3">
                            <span>
                              {formatBytes(item.bytes_downloaded)} /{" "}
                              {formatBytes(item.total_bytes)}
                            </span>
                            {item.speed_bytes_per_sec > 0 && (
                              <span>
                                {t.downloadSpeed}: {formatSpeed(item.speed_bytes_per_sec)}
                              </span>
                            )}
                            {item.eta_seconds > 0 && (
                              <span>
                                {t.downloadETA}: {formatDuration(item.eta_seconds)}
                              </span>
                            )}
                          </div>

                          {(item.status === "downloading" ||
                            item.status === "verifying") && (
                            <button
                              type="button"
                              onClick={() => handleCancelDownload(item.id)}
                              className="text-red-600 hover:text-red-800 font-semibold cursor-pointer p-1"
                              aria-label={`${t.cancelDownload} ${item.filename}`}
                            >
                              {t.cancelDownload}
                            </button>
                          )}
                        </div>

                        {item.error && (
                          <div className="mt-2 text-red-600 font-medium">
                            {item.error}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </section>
            </div>
          )}

          {/* TAB: Services & Add-ons */}
          {activeTab === "services" && (
            <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
              <h1 className="text-lg font-semibold text-[#0F172A]">
                {t.tabServices}
              </h1>
              <p className="mt-1 text-xs text-[#64748B]">
                Enable or disable sovereign appliance modules without losing stored user data.
              </p>

              <div className="mt-6 divide-y divide-[#F1F5F9]">
                <div className="py-3 flex items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-[#0F172A]">{t.videoTitle} ({t.videoSubtitle})</span>
                    <p className="text-xs text-[#64748B]">{t.videoDesc}</p>
                  </div>
                  <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
                </div>

                <div className="py-3 flex items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-[#0F172A]">{t.audioTitle} ({t.audioSubtitle})</span>
                    <p className="text-xs text-[#64748B]">{t.audioDesc}</p>
                  </div>
                  <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
                </div>

                <div className="py-3 flex items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-[#0F172A]">{t.dmsTitle} ({t.dmsSubtitle})</span>
                    <p className="text-xs text-[#64748B]">{t.dmsDesc}</p>
                  </div>
                  <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
                </div>

                <div className="py-3 flex items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-[#0F172A]">{t.aiTitle} ({t.aiSubtitle})</span>
                    <p className="text-xs text-[#64748B]">{t.aiDesc}</p>
                  </div>
                  <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
                </div>
              </div>
            </section>
          )}

          {/* TAB: Smart Hardware Sharing */}
          {activeTab === "gpu" && (
            <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
              <h1 className="text-lg font-semibold text-[#0F172A]">
                {t.gpuSharing}
              </h1>
              <p className="mt-1 text-xs text-[#64748B]">
                Balance real-time AI responsiveness against heavy batch video transcoding.
              </p>

              <div className="mt-6 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-4">
                <div className="flex justify-between text-xs font-semibold text-[#0F172A]">
                  <span>☀️ Daytime (AI Priority): 07:00 – 01:00</span>
                  <span>🌙 Nighttime (Batch Transcode): 01:00 – 07:00</span>
                </div>

                <div className="mt-3 flex h-8 overflow-hidden rounded-md text-xs font-bold text-white">
                  <div className="flex items-center justify-center bg-blue-600 w-3/4">
                    Interactive AI (18h)
                  </div>
                  <div className="flex items-center justify-center bg-[#0F172A] w-1/4">
                    Batch (6h)
                  </div>
                </div>

                <div className="mt-2 flex justify-between text-[11px] text-[#64748B]">
                  <span>00:00</span>
                  <span>06:00</span>
                  <span>12:00</span>
                  <span>18:00</span>
                  <span>24:00</span>
                </div>
              </div>
            </section>
          )}

          {/* TAB: System & Hardware */}
          {activeTab === "system" && (
            <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
              <h1 className="text-lg font-semibold text-[#0F172A]">
                {t.tabSystem}
              </h1>
              <p className="mt-1 text-xs text-[#64748B]">
                System maintenance, updates, and storage diagnostic health.
              </p>

              <div className="mt-6 divide-y divide-[#F1F5F9] text-xs">
                <div className="py-3 flex justify-between">
                  <span className="text-[#64748B]">Daily Update Window</span>
                  <span className="font-semibold text-[#0F172A]">03:00 AM</span>
                </div>
                <div className="py-3 flex justify-between">
                  <span className="text-[#64748B]">Rollback Protection</span>
                  <span className="font-semibold text-emerald-600">Active (180s probe window)</span>
                </div>
              </div>
            </section>
          )}
        </main>
      </div>

      {/* Inference Logs Modal Dialog */}
      {showLogsModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby="logs-modal-title"
          ref={logsModalRef}
        >
          <div className="w-full max-w-3xl rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xl max-h-[85vh] flex flex-col">
            <div className="flex items-start justify-between pb-3 border-b border-[#E5E7EB]">
              <div>
                <h3 id="logs-modal-title" className="text-base font-semibold text-[#0F172A]">
                  {t.logsModalTitle}
                </h3>
                <p className="mt-1 text-xs text-[#64748B]">
                  {t.logsModalDesc}
                </p>
              </div>

              <button
                type="button"
                onClick={() => setShowLogsModal(false)}
                className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-lg text-[#64748B] hover:text-[#0F172A] hover:bg-[#F1F5F9] cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                aria-label={t.closeLogs}
              >
                ✕
              </button>
            </div>

            <div className="overflow-y-auto flex-1 mt-4">
              {inferenceLogs.length === 0 ? (
                <div className="py-12 text-center text-xs text-[#64748B]">
                  {t.noLogsRecorded}
                </div>
              ) : (
                <table className="w-full text-left text-xs">
                  <thead>
                    <tr className="border-b border-[#E5E7EB] text-[#64748B] font-semibold">
                      <th className="pb-2 pr-3">{t.logTimestamp}</th>
                      <th className="pb-2 px-3">{t.logStatus}</th>
                      <th className="pb-2 px-3">Message</th>
                      <th className="pb-2 px-3">{t.logCompletionTokens}</th>
                      <th className="pb-2 px-3">{t.logLatency}</th>
                      <th className="pb-2 pl-3">{t.logSpeed}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[#F1F5F9]">
                    {inferenceLogs.map((log, idx) => (
                      <tr key={idx} className="hover:bg-[#F8F9FA]">
                        <td className="py-2.5 pr-3 text-[#64748B]">
                          {new Date(log.timestamp).toLocaleTimeString()}
                        </td>
                        <td className="py-2.5 px-3">
                          <span
                            className={`font-semibold ${
                              log.level === "ERROR"
                                ? "text-red-600"
                                : log.level === "WARN"
                                ? "text-amber-600"
                                : "text-emerald-600"
                            }`}
                          >
                            {log.level === "ERROR" ? "▲ ERROR" : `● ${log.level}`}
                          </span>
                        </td>
                        <td className="py-2.5 px-3 text-[#0F172A] max-w-xs truncate">
                          {log.message}
                        </td>
                        <td className="py-2.5 px-3">
                          {log.tokens_generated !== undefined ? log.tokens_generated : "—"}
                        </td>
                        <td className="py-2.5 px-3">
                          {log.duration_ms !== undefined ? `${log.duration_ms} ms` : "—"}
                        </td>
                        <td className="py-2.5 pl-3">
                          {log.tokens_per_sec !== undefined && log.tokens_per_sec > 0
                            ? `${log.tokens_per_sec.toFixed(1)} ${t.tokensPerSec}`
                            : "—"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>

            <div className="mt-4 pt-3 border-t border-[#E5E7EB] flex justify-end">
              <button
                type="button"
                onClick={() => setShowLogsModal(false)}
                className="min-h-[44px] rounded-lg border border-[#CBD5E1] bg-[#F8F9FA] px-4 py-2 text-xs font-semibold text-[#0F172A] hover:bg-[#F1F5F9] cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
              >
                {t.closeLogs}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {modelToDelete && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          role="dialog"
          aria-modal="true"
          ref={deleteModalRef}
        >
          <div className="w-full max-w-md rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xl">
            <h3 className="text-base font-semibold text-[#0F172A]">
              {t.confirmDeleteModel}
            </h3>
            <p className="mt-2 text-xs text-[#64748B]">
              {modelToDelete}
            </p>

            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setModelToDelete(null)}
                className="min-h-[44px] rounded-lg border border-[#CBD5E1] bg-white px-4 py-2 text-xs font-medium text-[#0F172A] hover:bg-[#F1F5F9] cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
              >
                {t.cancel}
              </button>
              <button
                type="button"
                onClick={handleDeleteModel}
                className="min-h-[44px] rounded-lg bg-red-600 px-4 py-2 text-xs font-semibold text-white hover:bg-red-700 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                data-testid="confirm-delete-button"
              >
                {t.deleteModel}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
