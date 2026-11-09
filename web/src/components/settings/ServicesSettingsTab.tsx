"use client";

import React from "react";
import { DashboardTranslations } from "@/lib/i18n";
import { DestructiveActionType } from "@/components/DestructiveModal";

export interface ServicesSettingsTabProps {
  t: DashboardTranslations;
  onSelectRollbackTarget: (target: { id: string; name: string; icon: string }) => void;
  onOpenDestructiveModal: (state: {
    isOpen: boolean;
    actionType: DestructiveActionType;
    serviceId?: string;
    serviceName?: string;
  }) => void;
}

export function ServicesSettingsTab({
  t,
  onSelectRollbackTarget,
  onOpenDestructiveModal,
}: ServicesSettingsTabProps) {
  return (
    <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 shadow-xs">
      <h1 className="text-lg font-semibold text-[#0F172A]">{t.tabServices}</h1>
      <p className="mt-1 text-xs text-[#64748B]">
        Enable or disable sovereign appliance modules without losing stored user data.
      </p>

      <div className="mt-6 divide-y divide-[#F1F5F9]">
        <div className="py-3.5 flex items-center justify-between gap-4">
          <div>
            <span className="text-sm font-medium text-[#0F172A]">
              {t.videoTitle} ({t.videoSubtitle})
            </span>
            <p className="text-xs text-[#64748B]">{t.videoDesc}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
            <button
              type="button"
              onClick={() => onSelectRollbackTarget({ id: "video", name: t.videoTitle, icon: "🎬" })}
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-[#CBD5E1] bg-white text-[#0F172A] hover:bg-[#F8FAFC] transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
              data-testid="rollback-btn-video"
            >
              <span aria-hidden="true">⏪</span>
              <span>{t.rollbackVersion}</span>
            </button>
            <button
              type="button"
              onClick={() =>
                onOpenDestructiveModal({
                  isOpen: true,
                  actionType: "deactivate",
                  serviceId: "video",
                  serviceName: t.videoTitle,
                })
              }
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-rose-200 bg-white text-rose-700 hover:bg-rose-50 transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-rose-500 focus-visible:outline-none"
              data-testid="deactivate-btn-video"
            >
              <span aria-hidden="true">⏸️</span>
              <span>{t.deactivateTitle}</span>
            </button>
          </div>
        </div>

        <div className="py-3.5 flex items-center justify-between gap-4">
          <div>
            <span className="text-sm font-medium text-[#0F172A]">
              {t.audioTitle} ({t.audioSubtitle})
            </span>
            <p className="text-xs text-[#64748B]">{t.audioDesc}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
            <button
              type="button"
              onClick={() => onSelectRollbackTarget({ id: "audio", name: t.audioTitle, icon: "🎵" })}
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-[#CBD5E1] bg-white text-[#0F172A] hover:bg-[#F8FAFC] transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
              data-testid="rollback-btn-audio"
            >
              <span aria-hidden="true">⏪</span>
              <span>{t.rollbackVersion}</span>
            </button>
            <button
              type="button"
              onClick={() =>
                onOpenDestructiveModal({
                  isOpen: true,
                  actionType: "deactivate",
                  serviceId: "audio",
                  serviceName: t.audioTitle,
                })
              }
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-rose-200 bg-white text-rose-700 hover:bg-rose-50 transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-rose-500 focus-visible:outline-none"
              data-testid="deactivate-btn-audio"
            >
              <span aria-hidden="true">⏸️</span>
              <span>{t.deactivateTitle}</span>
            </button>
          </div>
        </div>

        <div className="py-3.5 flex items-center justify-between gap-4">
          <div>
            <span className="text-sm font-medium text-[#0F172A]">
              {t.dmsTitle} ({t.dmsSubtitle})
            </span>
            <p className="text-xs text-[#64748B]">{t.dmsDesc}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
            <button
              type="button"
              onClick={() => onSelectRollbackTarget({ id: "dms", name: t.dmsTitle, icon: "📑" })}
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-[#CBD5E1] bg-white text-[#0F172A] hover:bg-[#F8FAFC] transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
              data-testid="rollback-btn-dms"
            >
              <span aria-hidden="true">⏪</span>
              <span>{t.rollbackVersion}</span>
            </button>
            <button
              type="button"
              onClick={() =>
                onOpenDestructiveModal({
                  isOpen: true,
                  actionType: "deactivate",
                  serviceId: "dms",
                  serviceName: t.dmsTitle,
                })
              }
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-rose-200 bg-white text-rose-700 hover:bg-rose-50 transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-rose-500 focus-visible:outline-none"
              data-testid="deactivate-btn-dms"
            >
              <span aria-hidden="true">⏸️</span>
              <span>{t.deactivateTitle}</span>
            </button>
          </div>
        </div>

        <div className="py-3.5 flex items-center justify-between gap-4">
          <div>
            <span className="text-sm font-medium text-[#0F172A]">
              {t.aiTitle} ({t.aiSubtitle})
            </span>
            <p className="text-xs text-[#64748B]">{t.aiDesc}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs font-semibold text-emerald-600">● {t.runningHealthy}</span>
            <button
              type="button"
              onClick={() => onSelectRollbackTarget({ id: "ai", name: t.aiTitle, icon: "⚡" })}
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-[#CBD5E1] bg-white text-[#0F172A] hover:bg-[#F8FAFC] transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
              data-testid="rollback-btn-ai"
            >
              <span aria-hidden="true">⏪</span>
              <span>{t.rollbackVersion}</span>
            </button>
            <button
              type="button"
              onClick={() =>
                onOpenDestructiveModal({
                  isOpen: true,
                  actionType: "deactivate",
                  serviceId: "ai",
                  serviceName: t.aiTitle,
                })
              }
              className="h-11 min-h-[44px] px-3.5 text-xs font-medium rounded-lg border border-rose-200 bg-white text-rose-700 hover:bg-rose-50 transition-colors cursor-pointer flex items-center gap-1.5 focus-visible:ring-2 focus-visible:ring-rose-500 focus-visible:outline-none"
              data-testid="deactivate-btn-ai"
            >
              <span aria-hidden="true">⏸️</span>
              <span>{t.deactivateTitle}</span>
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
