"use client";

import React from "react";
import { DashboardTranslations } from "@/lib/i18n";
import {
  AIModelInfo,
  AIModelDownloadProgress,
  AISettingsResponse,
} from "@/lib/api";
import { formatBytes, formatSpeed, formatDuration } from "@/lib/format";

export interface AISettingsTabProps {
  t: DashboardTranslations;
  aiSettings: AISettingsResponse | null;
  activeModelObj: AIModelInfo | null;
  isCpuOnly: boolean;
  reloadNotice: string | null;
  setReloadNotice: (val: string | null) => void;
  actionError: { title: string; detail?: string } | null;
  setActionError: (val: { title: string; detail?: string } | null) => void;
  generationPreset: string;
  onPresetChange: (preset: string) => void;
  onContextLengthChange: (len: number) => Promise<void> | void;
  sliderGpuLayers: number | null;
  setSliderGpuLayers: (val: number | null) => void;
  onGpuLayersChange: (val: number) => Promise<void> | void;
  sliderIdleTimeout: number | null;
  setSliderIdleTimeout: (val: number | null) => void;
  onIdleTimeoutChange: (val: number) => Promise<void> | void;
  isReloading: boolean;
  onReloadEngine: () => Promise<void> | void;
  onOpenLogs: () => Promise<void> | void;
  models: AIModelInfo[];
  activatingModel: string | null;
  onSwitchModel: (filename: string) => Promise<void> | void;
  setModelToDelete: (filename: string | null) => void;
  downloadError: string | null;
  setDownloadError: (val: string | null) => void;
  downloadUrl: string;
  setDownloadUrl: (val: string) => void;
  downloadSha256: string;
  setDownloadSha256: (val: string) => void;
  downloadFilename: string;
  setDownloadFilename: (val: string) => void;
  isStartingDownload: boolean;
  onStartDownload: (e: React.FormEvent) => Promise<void> | void;
  downloads: AIModelDownloadProgress[];
  onCancelDownload: (id: string) => Promise<void> | void;
}

export function AISettingsTab({
  t,
  aiSettings,
  activeModelObj,
  isCpuOnly,
  reloadNotice,
  setReloadNotice,
  actionError,
  setActionError,
  generationPreset,
  onPresetChange,
  onContextLengthChange,
  sliderGpuLayers,
  setSliderGpuLayers,
  onGpuLayersChange,
  sliderIdleTimeout,
  setSliderIdleTimeout,
  onIdleTimeoutChange,
  isReloading,
  onReloadEngine,
  onOpenLogs,
  models,
  activatingModel,
  onSwitchModel,
  setModelToDelete,
  downloadError,
  setDownloadError,
  downloadUrl,
  setDownloadUrl,
  downloadSha256,
  setDownloadSha256,
  downloadFilename,
  setDownloadFilename,
  isStartingDownload,
  onStartDownload,
  downloads,
  onCancelDownload,
}: AISettingsTabProps) {
  return (
    <div className="flex flex-col gap-6" data-testid="ai-settings-panel">
      {/* Card Header & Badges */}
      <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
        <div className="flex flex-wrap items-start justify-between gap-4 pb-4 border-b border-[#F1F5F9]">
          <div>
            <h1 className="text-lg font-semibold tracking-tight text-[#0F172A]">
              {t.aiEngineTitle}
            </h1>
            <p className="mt-1 text-xs text-[#64748B]">{t.aiEngineSubtitle}</p>
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
                <span aria-hidden="true" className="text-gray-500">
                  ■
                </span>
                {t.cpuOptimizedBadge}
              </span>
            ) : (
              <span
                className="inline-flex items-center gap-1.5 rounded-full border border-[#A7F3D0] bg-[#ECFDF5] px-2.5 py-1 text-xs font-semibold text-[#065F46]"
                data-testid="default-setup-badge"
              >
                <span aria-hidden="true" className="text-emerald-500">
                  ●
                </span>
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

        {/* Operational Error Banner */}
        {actionError && (
          <div
            className="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-800 font-medium flex items-center justify-between"
            role="alert"
            data-testid="action-error"
          >
            <div>
              <span className="font-bold">{actionError.title}</span>
              {actionError.detail && (
                <span className="ml-2 font-normal">{actionError.detail}</span>
              )}
            </div>
            <button
              type="button"
              onClick={() => setActionError(null)}
              className="text-red-700 hover:text-red-900 p-1 font-bold cursor-pointer"
              aria-label="Dismiss error"
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
              <span className="block text-xs text-[#64748B]">{t.ramFootprint}</span>
              <span
                className="text-sm font-semibold text-[#0F172A]"
                data-testid="ram-footprint"
              >
                {formatBytes(aiSettings?.ram_usage_bytes || 0)}
              </span>
            </div>

            <div>
              <span className="block text-xs text-[#64748B]">{t.vramFootprint}</span>
              <span
                className="text-sm font-semibold text-[#0F172A]"
                data-testid="vram-footprint"
              >
                {formatBytes(aiSettings?.vram_usage_bytes || 0)}
              </span>
            </div>

            <div>
              <span className="block text-xs text-[#64748B]">{t.inferenceSpeed}</span>
              <span
                className="text-sm font-semibold text-[#0F172A]"
                data-testid="tokens-per-sec"
              >
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
              <p className="text-xs text-[#64748B]">{t.aiDesc}</p>
            </div>

            <div
              className="inline-flex rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-1 gap-1"
              role="radiogroup"
              aria-label={t.generationPresetLabel}
            >
              <button
                type="button"
                onClick={() => onPresetChange("precise")}
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
                onClick={() => onPresetChange("balanced")}
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
                onClick={() => onPresetChange("creative")}
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
                  onClick={() => onContextLengthChange(len)}
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
                  : `${sliderGpuLayers ?? aiSettings?.gpu_layers ?? 0} layers`}
              </span>
            </div>

            <div className="mt-3 flex items-center gap-3">
              <input
                type="range"
                id="gpu-layers-slider"
                min="0"
                max="99"
                disabled={isCpuOnly}
                value={isCpuOnly ? 0 : sliderGpuLayers ?? aiSettings?.gpu_layers ?? 0}
                onChange={(e) => setSliderGpuLayers(parseInt(e.target.value, 10))}
                onPointerUp={(e) => {
                  const val = parseInt((e.target as HTMLInputElement).value, 10);
                  setSliderGpuLayers(null);
                  void onGpuLayersChange(val);
                }}
                onKeyUp={(e) => {
                  const val = parseInt((e.target as HTMLInputElement).value, 10);
                  setSliderGpuLayers(null);
                  void onGpuLayersChange(val);
                }}
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
                        (sliderIdleTimeout ?? aiSettings?.idle_timeout_seconds ?? 300) /
                          60
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
                  (sliderIdleTimeout ?? aiSettings?.idle_timeout_seconds ?? 300) / 60
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
                value={sliderIdleTimeout ?? aiSettings?.idle_timeout_seconds ?? 300}
                onChange={(e) => setSliderIdleTimeout(parseInt(e.target.value, 10))}
                onPointerUp={(e) => {
                  const val = parseInt((e.target as HTMLInputElement).value, 10);
                  setSliderIdleTimeout(null);
                  void onIdleTimeoutChange(val);
                }}
                onKeyUp={(e) => {
                  const val = parseInt((e.target as HTMLInputElement).value, 10);
                  setSliderIdleTimeout(null);
                  void onIdleTimeoutChange(val);
                }}
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
            onClick={onReloadEngine}
            disabled={isReloading}
            className="min-h-[44px] inline-flex items-center justify-center gap-2 rounded-lg border border-[#CBD5E1] bg-[#F8F9FA] px-4 py-2 text-xs sm:text-sm font-semibold text-[#0F172A] hover:bg-[#F1F5F9] disabled:opacity-50 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors shadow-xs"
            aria-label={t.reloadEngine}
            data-testid="reload-engine-button"
          >
            {isReloading && <span className="inline-block animate-spin">⟳</span>}
            <span>{isReloading ? t.reloadingEngine : t.reloadEngine}</span>
          </button>

          <button
            type="button"
            onClick={onOpenLogs}
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
                        <span className="font-semibold">
                          {model.name || model.filename}
                        </span>
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
                            onClick={() => onSwitchModel(model.filename)}
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
                          title={
                            model.is_active ? "Cannot delete active model" : t.deleteModel
                          }
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
          <p className="mt-1 text-xs text-[#64748B]">{t.downloadManagerSubtitle}</p>
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
        <form onSubmit={onStartDownload} className="mt-4 flex flex-col gap-4">
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

                  {(item.status === "downloading" || item.status === "verifying") && (
                    <button
                      type="button"
                      onClick={() => onCancelDownload(item.id)}
                      className="text-red-600 hover:text-red-800 font-semibold cursor-pointer p-1"
                      aria-label={`${t.cancelDownload} ${item.filename}`}
                    >
                      {t.cancelDownload}
                    </button>
                  )}
                </div>

                {item.error && (
                  <div className="mt-2 text-red-600 font-medium">{item.error}</div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
