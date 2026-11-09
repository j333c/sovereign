"use client";

import React from "react";
import Link from "next/link";
import { DashboardTranslations } from "@/lib/i18n";
import { DestructiveActionType } from "@/components/DestructiveModal";

export interface SystemSettingsTabProps {
  t: DashboardTranslations;
  onOpenDestructiveModal: (state: {
    isOpen: boolean;
    actionType: DestructiveActionType;
    serviceId?: string;
    serviceName?: string;
  }) => void;
}

export function SystemSettingsTab({ t, onOpenDestructiveModal }: SystemSettingsTabProps) {
  return (
    <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
      <h1 className="text-lg font-semibold text-[#0F172A]">{t.tabSystem}</h1>
      <p className="mt-1 text-xs text-[#64748B]">{t.systemMaintenanceDesc}</p>

      <div className="mt-6 divide-y divide-[#F1F5F9] text-xs">
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
