"use client";

import React from "react";
import { DashboardTranslations } from "@/lib/i18n";
import { DiskInfo, StorageStatusResponse } from "@/lib/api";
import { formatBytes } from "@/lib/format";

export interface StorageSettingsTabProps {
  t: DashboardTranslations;
  disks: DiskInfo[];
  storageStatus: StorageStatusResponse | null;
  nasHost: string;
  setNasHost: (val: string) => void;
  nasShare: string;
  setNasShare: (val: string) => void;
  nasUsername: string;
  setNasUsername: (val: string) => void;
  nasPassword: string;
  setNasPassword: (val: string) => void;
  nasTestStatus: "idle" | "testing" | "success" | "error";
  nasTestMessage: string | null;
  nasMountStatus: "idle" | "mounting" | "success" | "error";
  nasMountMessage: string | null;
  onTestNAS: () => void;
  onMountNAS: () => void;
}

export function StorageSettingsTab({
  t,
  disks,
  storageStatus,
  nasHost,
  setNasHost,
  nasShare,
  setNasShare,
  nasUsername,
  setNasUsername,
  nasPassword,
  setNasPassword,
  nasTestStatus,
  nasTestMessage,
  nasMountStatus,
  nasMountMessage,
  onTestNAS,
  onMountNAS,
}: StorageSettingsTabProps) {
  return (
    <div className="flex flex-col gap-6" data-testid="storage-settings-panel">
      {/* Storage Pool Status Card */}
      <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs" data-testid="storage-pool-card">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-4 border-b border-[#F1F5F9]">
          <div>
            <h1 className="text-lg font-semibold text-[#0F172A]">{t.storagePoolStatus}</h1>
            <p className="mt-1 text-xs text-[#64748B]">
              {storageStatus?.mount_point ? `Local Pool: ${storageStatus.mount_point}` : "Local Pool: /data"}
            </p>
          </div>
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold ${
              storageStatus?.status === "degraded"
                ? "bg-amber-100 text-amber-800 border border-amber-300"
                : "bg-emerald-100 text-emerald-800 border border-emerald-300"
            }`}
          >
            <span aria-hidden="true">{storageStatus?.status === "degraded" ? "◆" : "●"}</span>
            <span>{storageStatus?.status === "degraded" ? t.storageDegraded : t.storageHealthy}</span>
          </span>
        </div>

        <div className="mt-4 grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
          <div className="rounded-lg bg-[#F8FAFC] border border-[#E2E8F0] p-3.5">
            <span className="text-[#64748B] block font-medium">{t.totalCapacity}</span>
            <span className="text-base font-bold text-[#0F172A] mt-1 block">
              {storageStatus?.total_bytes != null ? formatBytes(storageStatus.total_bytes) : "—"}
            </span>
          </div>
          <div className="rounded-lg bg-[#F8FAFC] border border-[#E2E8F0] p-3.5">
            <span className="text-[#64748B] block font-medium">{t.usedSpace}</span>
            <span className="text-base font-bold text-[#0F172A] mt-1 block">
              {storageStatus?.used_bytes != null ? formatBytes(storageStatus.used_bytes) : "—"}
            </span>
          </div>
          <div className="rounded-lg bg-[#F8FAFC] border border-[#E2E8F0] p-3.5">
            <span className="text-[#64748B] block font-medium">{t.freeSpace}</span>
            <span className="text-base font-bold text-[#0F172A] mt-1 block">
              {storageStatus?.available_bytes != null ? formatBytes(storageStatus.available_bytes) : "—"}
            </span>
          </div>
        </div>
      </section>

      {/* Detected Physical Disks Table */}
      <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs" data-testid="detected-disks-card">
        <h2 className="text-base font-semibold text-[#0F172A]">{t.detectedDisksTitle}</h2>
        <div className="overflow-x-auto mt-4">
          <table className="w-full text-left text-xs" data-testid="detected-disks-table">
            <thead>
              <tr className="border-b border-[#E5E7EB] text-[#64748B] font-semibold">
                <th className="pb-2.5 pr-4">{t.diskDeviceCol}</th>
                <th className="pb-2.5 px-4">{t.diskModelCol}</th>
                <th className="pb-2.5 px-4">{t.diskCapacityCol}</th>
                <th className="pb-2.5 pl-4">{t.diskTypeCol}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#F1F5F9]">
              {disks.length === 0 ? (
                <tr>
                  <td colSpan={4} className="py-6 text-center text-[#94A3B8]">
                    {t.noDisksDetected}
                  </td>
                </tr>
              ) : (
                disks.map((d, i) => (
                  <tr key={d.path || d.name || i} className="hover:bg-[#F8FAFC]">
                    <td className="py-3 pr-4 font-mono font-medium text-[#0F172A]">{d.path || d.name}</td>
                    <td className="py-3 px-4 text-[#334155]">{d.model || "Standard Storage Drive"}</td>
                    <td className="py-3 px-4 font-medium text-[#0F172A]">{formatBytes(d.size_bytes)}</td>
                    <td className="py-3 pl-4 text-[#64748B] uppercase">{d.type || "SSD"}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* External NAS Configuration Card */}
      <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs" data-testid="nas-config-card">
        <h2 className="text-base font-semibold text-[#0F172A]">{t.nasConfigTitle}</h2>
        <p className="mt-1 text-xs text-[#64748B]">{t.nasConfigDesc}</p>

        <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
          <div>
            <label htmlFor="nas-host" className="block font-medium text-[#334155] mb-1">
              {t.nasHostLabel}
            </label>
            <input
              id="nas-host"
              type="text"
              value={nasHost}
              onChange={(e) => setNasHost(e.target.value)}
              placeholder="192.168.1.100"
              className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] px-3.5 py-2 text-sm text-[#0F172A] focus:outline-none focus:ring-2 focus:ring-blue-600"
              data-testid="nas-host-input"
            />
          </div>
          <div>
            <label htmlFor="nas-share" className="block font-medium text-[#334155] mb-1">
              {t.nasShareLabel}
            </label>
            <input
              id="nas-share"
              type="text"
              value={nasShare}
              onChange={(e) => setNasShare(e.target.value)}
              placeholder="media"
              className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] px-3.5 py-2 text-sm text-[#0F172A] focus:outline-none focus:ring-2 focus:ring-blue-600"
              data-testid="nas-share-input"
            />
          </div>
          <div>
            <label htmlFor="nas-username" className="block font-medium text-[#334155] mb-1">
              {t.nasUsernameLabel}
            </label>
            <input
              id="nas-username"
              type="text"
              value={nasUsername}
              onChange={(e) => setNasUsername(e.target.value)}
              placeholder="admin"
              className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] px-3.5 py-2 text-sm text-[#0F172A] focus:outline-none focus:ring-2 focus:ring-blue-600"
              data-testid="nas-user-input"
            />
          </div>
          <div>
            <label htmlFor="nas-password" className="block font-medium text-[#334155] mb-1">
              {t.nasPasswordLabel}
            </label>
            <input
              id="nas-password"
              type="password"
              value={nasPassword}
              onChange={(e) => setNasPassword(e.target.value)}
              placeholder="••••••••"
              className="w-full min-h-[44px] rounded-lg border border-[#CBD5E1] px-3.5 py-2 text-sm text-[#0F172A] focus:outline-none focus:ring-2 focus:ring-blue-600"
              data-testid="nas-pass-input"
            />
          </div>
        </div>

        {/* NAS Action Feedback */}
        {nasTestMessage && (
          <div
            className={`mt-4 rounded-xl border p-3.5 text-xs ${
              nasTestStatus === "success"
                ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                : "border-rose-200 bg-rose-50 text-rose-800"
            }`}
            role="alert"
            data-testid="nas-test-feedback"
          >
            {nasTestMessage}
          </div>
        )}
        {nasMountMessage && (
          <div
            className={`mt-4 rounded-xl border p-3.5 text-xs ${
              nasMountStatus === "success"
                ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                : "border-rose-200 bg-rose-50 text-rose-800"
            }`}
            role="alert"
            data-testid="nas-mount-feedback"
          >
            {nasMountMessage}
          </div>
        )}

        <div className="mt-5 flex flex-wrap gap-3">
          <button
            type="button"
            onClick={onTestNAS}
            disabled={nasTestStatus === "testing" || !nasHost || !nasShare}
            className="min-h-[44px] px-4 py-2.5 rounded-xl border border-[#CBD5E1] bg-white text-xs font-semibold text-[#0F172A] hover:bg-[#F8FAFC] transition-colors cursor-pointer disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
            data-testid="nas-test-btn"
          >
            {nasTestStatus === "testing" ? t.nasTesting : t.nasTestConnection}
          </button>
          <button
            type="button"
            onClick={onMountNAS}
            disabled={nasMountStatus === "mounting" || !nasHost || !nasShare}
            className="min-h-[44px] px-4 py-2.5 rounded-xl bg-blue-600 text-xs font-semibold text-white hover:bg-blue-700 transition-colors cursor-pointer disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
            data-testid="nas-mount-btn"
          >
            {nasMountStatus === "mounting" ? t.nasMounting : t.nasMountButton}
          </button>
        </div>
      </section>
    </div>
  );
}
