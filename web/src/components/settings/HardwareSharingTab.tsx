"use client";

import React from "react";
import { DashboardTranslations } from "@/lib/i18n";

export interface HardwareSharingTabProps {
  t: DashboardTranslations;
}

export function HardwareSharingTab({ t }: HardwareSharingTabProps) {
  return (
    <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
      <h1 className="text-lg font-semibold text-[#0F172A]">{t.gpuSharing}</h1>
      <p className="mt-1 text-xs text-[#64748B]">{t.balanceHardwareSharing}</p>

      <div className="mt-6 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] p-4">
        <div className="flex justify-between text-xs font-semibold text-[#0F172A]">
          <span>{t.daytimeSchedule}</span>
          <span>{t.nighttimeSchedule}</span>
        </div>

        <div className="mt-3 flex h-8 overflow-hidden rounded-md text-xs font-bold text-white">
          <div className="flex items-center justify-center bg-blue-600 w-3/4">
            {t.interactiveAILabel}
          </div>
          <div className="flex items-center justify-center bg-[#0F172A] w-1/4">
            {t.batchTranscodeLabel}
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
  );
}
