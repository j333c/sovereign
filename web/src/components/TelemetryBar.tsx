"use client";

import React from "react";
import { TelemetryUpdate } from "@/lib/api";
import { Language, translations } from "@/lib/i18n";

export interface TelemetryBarProps {
  telemetry: TelemetryUpdate | null;
  currentLanguage: Language;
}

export function formatBytesToTB(bytes: number): string {
  if (bytes === 0) return "0.0 TB";
  const tb = bytes / (1024 * 1024 * 1024 * 1024);
  return `${tb.toFixed(1)} TB`;
}

export function formatBytesToGB(bytes: number): string {
  if (bytes === 0) return "0.0 GB";
  const gb = bytes / (1024 * 1024 * 1024);
  return `${gb.toFixed(1)} GB`;
}

export function TelemetryBar({ telemetry, currentLanguage }: TelemetryBarProps) {
  const t = translations[currentLanguage];

  // Default fallback baseline when waiting for initial SSE
  const cpuPercent = typeof telemetry?.cpu_percent === "number" ? telemetry.cpu_percent : 0;
  const memUsedGB = telemetry ? formatBytesToGB(telemetry.memory_used_bytes) : "0.0 GB";
  const memTotalGB = telemetry ? formatBytesToGB(telemetry.memory_total_bytes) : "0.0 GB";

  const storageUsedTB = telemetry ? formatBytesToTB(telemetry.storage_used_bytes) : "0.0 TB";
  const storageTotalTB = telemetry ? formatBytesToTB(telemetry.storage_total_bytes) : "0.0 TB";
  const storagePercent = telemetry ? Math.min(100, Math.max(0, telemetry.storage_percent)) : 0;

  // Amber threshold: > 85%
  const isStorageWarning = storagePercent > 85;
  const progressColor = isStorageWarning ? "bg-amber-500" : "bg-emerald-500";

  // NAS status
  const nasRaw = telemetry?.nas_status ?? "not_configured";
  let nasLabel = t.nasNotConfigured;
  let nasColor = "bg-slate-400";
  if (nasRaw === "connected") {
    nasLabel = t.nasConnected;
    nasColor = "bg-emerald-500";
  } else if (nasRaw === "degraded") {
    nasLabel = t.nasDegraded;
    nasColor = "bg-amber-500";
  } else if (nasRaw === "disconnected" || nasRaw === "error") {
    nasLabel = t.nasDisconnected;
    nasColor = "bg-red-500";
  }

  let opticalStatus = t.opticalIdle;
  if (telemetry?.optical_status && telemetry.optical_status !== "Idle · Ready for disc") {
    opticalStatus = telemetry.optical_status;
  }

  let gpuStatus = t.gpuActive;
  if (telemetry?.gpu_status && telemetry.gpu_status !== "Daytime (AI Priority)") {
    gpuStatus = telemetry.gpu_status;
  }

  return (
    <footer
      className="fixed bottom-0 left-0 right-0 z-30 flex h-14 w-full items-center border-t border-[#E5E7EB] bg-white px-4 sm:px-8 text-xs text-[#0F172A] shadow-xs"
      data-testid="telemetry-bar"
      aria-live="polite"
      aria-atomic="false"
    >
      <div className="flex w-full items-center justify-between gap-4 overflow-x-auto no-scrollbar py-1">
        {/* Storage Pool with Progress Bar */}
        <div className="flex items-center gap-2.5 shrink-0">
          <span className="font-medium text-[#64748B]">{t.storage}:</span>
          <span className="font-semibold tabular-nums" data-testid="storage-metric">
            {storageUsedTB} / {storageTotalTB}
          </span>
          <div
            className="w-16 h-1.5 rounded-full bg-slate-100 overflow-hidden border border-slate-200"
            role="progressbar"
            aria-label={t.storage}
            aria-valuenow={Math.round(storagePercent)}
            aria-valuemin={0}
            aria-valuemax={100}
            data-testid="storage-progressbar"
          >
            <div
              className={`h-full transition-all duration-300 ${progressColor}`}
              style={{ width: `${storagePercent}%` }}
            />
          </div>
          {isStorageWarning && (
            <span
              className="text-[10px] font-semibold text-amber-700 bg-amber-50 border border-amber-200 rounded px-1"
              data-testid="storage-warning"
              aria-live="polite"
            >
              &gt;85%
            </span>
          )}
        </div>

        {/* Separator */}
        <span className="h-4 w-px bg-[#E5E7EB] shrink-0 hidden md:block" />

        {/* CPU & Memory metrics */}
        <div className="flex items-center gap-4 shrink-0">
          <div className="flex items-center gap-1.5" data-testid="cpu-metric">
            <span className="text-[#64748B]">{t.cpu}:</span>
            <span className="font-semibold tabular-nums">{cpuPercent.toFixed(1)}%</span>
          </div>

          <div className="flex items-center gap-1.5" data-testid="ram-metric">
            <span className="text-[#64748B]">{t.ram}:</span>
            <span className="font-semibold tabular-nums">
              {memUsedGB} / {memTotalGB}
            </span>
          </div>
        </div>

        {/* Separator */}
        <span className="h-4 w-px bg-[#E5E7EB] shrink-0 hidden md:block" />

        {/* NAS connection status */}
        <div className="flex items-center gap-1.5 shrink-0" data-testid="nas-status">
          <span className="text-[#64748B]">{t.nas}:</span>
          <span className="inline-flex items-center gap-1">
            <span className={`h-2 w-2 rounded-full ${nasColor}`} aria-hidden="true" />
            <span className="font-medium">{nasLabel}</span>
          </span>
        </div>

        {/* Separator */}
        <span className="h-4 w-px bg-[#E5E7EB] shrink-0 hidden lg:block" />

        {/* Optical Drive status */}
        <div className="items-center gap-1.5 shrink-0 hidden lg:flex" data-testid="optical-status">
          <span className="text-[#64748B]">{t.opticalDrive}:</span>
          <span className="font-medium text-[#475569]">{opticalStatus}</span>
        </div>

        {/* Separator */}
        <span className="h-4 w-px bg-[#E5E7EB] shrink-0 hidden xl:block" />

        {/* Smart Hardware Sharing */}
        <div className="items-center gap-1.5 shrink-0 hidden xl:flex" data-testid="gpu-status">
          <span className="text-[#64748B]">{t.gpuSharing}:</span>
          <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-[#0F172A]">
            {gpuStatus}
          </span>
        </div>
      </div>
    </footer>
  );
}
