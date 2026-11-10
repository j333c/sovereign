"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { DashboardTranslations } from "@/lib/i18n";
import { DestructiveActionType } from "@/components/DestructiveModal";

export interface SystemSettingsTabProps {
  t: DashboardTranslations;
  currentChannel?: string;
  currentVersion?: string;
  onChannelChange?: (channel: string, confirmation?: string) => Promise<void> | void;
  onOpenDestructiveModal: (state: {
    isOpen: boolean;
    actionType: DestructiveActionType;
    serviceId?: string;
    serviceName?: string;
  }) => void;
}

export function SystemSettingsTab({
  t,
  currentChannel = "stable",
  currentVersion,
  onChannelChange,
  onOpenDestructiveModal,
}: SystemSettingsTabProps) {
  const [selectedChannel, setSelectedChannel] = useState<string>(currentChannel);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  useEffect(() => {
    setSelectedChannel(currentChannel);
  }, [currentChannel]);

  const handleChannelSelect = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const nextChannel = e.target.value;
    if (nextChannel === "preview") {
      // Trigger amber DestructiveModal with actionType="preview-channel"
      onOpenDestructiveModal({
        isOpen: true,
        actionType: "preview-channel",
      });
      return;
    }

    // Stable or security-only updates immediately
    setSelectedChannel(nextChannel);
    if (onChannelChange) {
      try {
        await onChannelChange(nextChannel);
        setToastMessage(t.channelToastSuccess);
        setTimeout(() => setToastMessage(null), 4000);
      } catch {
        setSelectedChannel(currentChannel);
      }
    }
  };

  const isRunningPreview = currentVersion ? currentVersion.includes("-rc.") : false;
  const isGraduatingHold = isRunningPreview && (selectedChannel === "stable" || selectedChannel === "security-only");

  return (
    <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
      <h1 className="text-lg font-semibold text-[#0F172A]">{t.tabSystem}</h1>
      <p className="mt-1 text-xs text-[#64748B]">{t.systemMaintenanceDesc}</p>

      {/* Toast Confirmation */}
      {toastMessage && (
        <div
          data-testid="channel-toast"
          className="mt-4 rounded-xl border border-emerald-300 bg-emerald-50 p-3.5 text-xs text-emerald-900 shadow-xs flex items-center gap-2"
          role="status"
        >
          <span aria-hidden="true">✓</span>
          <span>{toastMessage}</span>
        </div>
      )}

      <div className="mt-6 divide-y divide-[#F1F5F9] text-xs">
        {/* Update Channel Selector */}
        <div className="py-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <label htmlFor="update-channel-select" className="text-sm font-medium text-[#0F172A] block">
              {t.updateChannelLabel}
            </label>
            <p className="text-xs text-[#64748B]">
              {selectedChannel === "stable"
                ? t.channelStable
                : selectedChannel === "security-only"
                ? t.channelSecurityOnly
                : t.channelPreview}
            </p>
          </div>
          <select
            id="update-channel-select"
            data-testid="update-channel-select"
            value={selectedChannel}
            onChange={handleChannelSelect}
            className="min-h-[44px] rounded-lg border border-[#CBD5E1] bg-white px-3 py-2 text-xs font-semibold text-[#0F172A] focus:border-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-100 cursor-pointer shadow-xs"
          >
            <option value="stable">{t.channelStable}</option>
            <option value="security-only">{t.channelSecurityOnly}</option>
            <option value="preview">{t.channelPreview}</option>
          </select>
        </div>

        {/* Graduating to Stable hold state notice */}
        {isGraduatingHold && (
          <div
            data-testid="graduating-to-stable-notice"
            className="py-4 my-2 rounded-xl border border-amber-300 bg-amber-50/80 p-4 text-xs text-amber-950"
          >
            <div className="flex items-center gap-2 font-bold text-sm text-amber-900">
              <span aria-hidden="true">⏳</span>
              <span>{t.graduatingToStableTitle}</span>
            </div>
            <p className="mt-1.5 leading-relaxed text-[#78350F]">
              {t.graduatingToStableDesc}
            </p>
            <div className="mt-3">
              <Link
                href="/settings?tab=services"
                className="font-semibold text-blue-700 underline hover:text-blue-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
                data-testid="graduating-rollback-link"
              >
                {t.manualRollbackLink} →
              </Link>
            </div>
          </div>
        )}

        <div className="py-3 flex justify-between">
          <span className="text-[#64748B]">{t.dailyUpdateWindow}</span>
          <span className="font-semibold text-[#0F172A]">03:00 AM</span>
        </div>
        <div className="py-3 flex justify-between">
          <span className="text-[#64748B]">{t.rollbackProtectionLabel}</span>
          <span className="font-semibold text-emerald-600">{t.rollbackProtectionActive}</span>
        </div>
        <div className="py-3 flex justify-between">
          <span className="text-[#64748B]">{t.applianceHostname}</span>
          <span className="font-mono font-semibold text-[#0F172A]">sovereign.local</span>
        </div>
        <div className="py-3 flex justify-between items-center">
          <div>
            <span className="text-sm font-medium text-[#0F172A]">{t.hardwareTitle}</span>
            <p className="text-xs text-[#64748B]">{t.hardwareSubtitle}</p>
          </div>
          <Link
            href="/hardware"
            className="flex min-h-[44px] items-center gap-1.5 rounded-lg border border-[#CBD5E1] bg-white px-3.5 py-2 text-xs font-semibold text-blue-600 hover:text-blue-800 hover:bg-[#F8FAFC] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer shadow-xs"
            data-testid="hardware-settings-link"
          >
            <span>💻</span>
            <span>{t.hardwareTitle}</span>
            <span aria-hidden="true">→</span>
          </Link>
        </div>
      </div>

      {/* Destructive Controls */}
      <div className="mt-8 pt-6 border-t border-[#F1F5F9]">
        <h2 className="text-sm font-bold text-rose-700 flex items-center gap-1.5">
          <span aria-hidden="true">⚠️</span>
          <span>{t.destructiveControlsTitle}</span>
        </h2>
        <p className="mt-1 text-xs text-[#64748B]">{t.destructiveControlsDesc}</p>

        <div className="mt-4 flex flex-wrap gap-4">
          <button
            type="button"
            onClick={() =>
              onOpenDestructiveModal({
                isOpen: true,
                actionType: "reboot",
              })
            }
            className="flex min-h-[44px] items-center gap-2 rounded-xl border border-amber-300 bg-amber-50 px-4 py-2.5 text-xs font-semibold text-amber-900 shadow-xs hover:bg-amber-100 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500"
            data-testid="reboot-appliance-btn"
          >
            <span aria-hidden="true">🔄</span>
            <span>{t.rebootTitle}</span>
          </button>

          <button
            type="button"
            onClick={() =>
              onOpenDestructiveModal({
                isOpen: true,
                actionType: "factory-reset",
              })
            }
            className="flex min-h-[44px] items-center gap-2 rounded-xl border border-rose-300 bg-rose-50 px-4 py-2.5 text-xs font-semibold text-rose-900 shadow-xs hover:bg-rose-100 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rose-500"
            data-testid="factory-reset-btn"
          >
            <span aria-hidden="true">⚠️</span>
            <span>{t.factoryResetTitle}</span>
          </button>
        </div>
      </div>
    </section>
  );
}
