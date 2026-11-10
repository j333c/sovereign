"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { RollbackModal } from "@/components/RollbackModal";
import { AdminLoginModal } from "@/components/AdminLoginModal";
import { DestructiveModal, DestructiveActionType } from "@/components/DestructiveModal";
import { AISettingsTab } from "@/components/settings/AISettingsTab";
import { ServicesSettingsTab } from "@/components/settings/ServicesSettingsTab";
import { StorageSettingsTab } from "@/components/settings/StorageSettingsTab";
import { HardwareSharingTab } from "@/components/settings/HardwareSharingTab";
import { SystemSettingsTab } from "@/components/settings/SystemSettingsTab";
import { Language } from "@/lib/i18n";
import { formatBytes, formatSpeed, formatDuration } from "@/lib/format";
import { useLanguage } from "@/lib/hooks";
import {
  apiClient as defaultApiClient,
  SovereignApiClient,
  AIModelInfo,
  AIModelDownloadProgress,
  AISettingsResponse,
  AIInferenceLogEntry,
  ProblemDetailsError,
  DiskInfo,
  StorageStatusResponse,
} from "@/lib/api";

export { formatBytes, formatSpeed, formatDuration };

export interface SettingsPageProps {
  apiClientOverride?: SovereignApiClient;
  initialLanguage?: Language;
  initialTab?: string;
  initialAuthenticated?: boolean;
  initialAISettings?: AISettingsResponse | null;
  initialModels?: AIModelInfo[];
  initialDownloads?: AIModelDownloadProgress[];
  initialRollbackServiceTarget?: { id: string; name: string; icon: string } | null;
  initialActionError?: { title: string; detail?: string } | null;
  initialRevisionsMap?: Record<string, { currentVersion?: string; previousVersion?: string; snapshotTimestamp?: string }>;
  initialDisks?: DiskInfo[];
  initialStorageStatus?: StorageStatusResponse | null;
}

export default function SettingsPage({
  apiClientOverride,
  initialLanguage,
  initialTab,
  initialAuthenticated,
  initialAISettings = null,
  initialModels = [],
  initialDownloads = [],
  initialRollbackServiceTarget = null,
  initialActionError = null,
  initialRevisionsMap = {},
  initialDisks = [],
  initialStorageStatus = null,
}: SettingsPageProps) {
  const client = apiClientOverride || defaultApiClient;
  const { lang, t, handleLanguageChange } = useLanguage(initialLanguage);

  // Authentication state
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(initialAuthenticated ?? true);
  const [isLoginModalOpen, setIsLoginModalOpen] = useState<boolean>(initialAuthenticated === false);

  useEffect(() => {
    if (initialAuthenticated === undefined) {
      client
        .getSession()
        .then(() => {
          setIsAuthenticated(true);
        })
        .catch(() => {
          setIsAuthenticated(false);
          setIsLoginModalOpen(true);
        });
    }
  }, [client, initialAuthenticated]);

  // Tab navigation state
  const [activeTab, setActiveTab] = useState<string>(initialTab || "ai");

  // Storage & NAS State
  const [disks, setDisks] = useState<DiskInfo[]>(initialDisks);
  const [storageStatus, setStorageStatus] = useState<StorageStatusResponse | null>(initialStorageStatus);
  const [nasHost, setNasHost] = useState("");
  const [nasShare, setNasShare] = useState("");
  const [nasUsername, setNasUsername] = useState("");
  const [nasPassword, setNasPassword] = useState("");
  const [nasTestStatus, setNasTestStatus] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [nasTestMessage, setNasTestMessage] = useState<string | null>(null);
  const [nasMountStatus, setNasMountStatus] = useState<"idle" | "mounting" | "success" | "error">("idle");
  const [nasMountMessage, setNasMountMessage] = useState<string | null>(null);

  const loadStorageState = useCallback(async () => {
    try {
      const [disksResp, statusResp] = await Promise.all([
        client.getDisks().catch(() => ({ disks: [] })),
        client.getStorageStatus().catch(() => null),
      ]);
      setDisks(disksResp.disks || []);
      setStorageStatus(statusResp);
    } catch {
      // Ignore
    }
  }, [client]);

  useEffect(() => {
    if (activeTab === "storage") {
      void loadStorageState();
    }
  }, [activeTab, loadStorageState]);

  const handleTestNAS = async () => {
    if (!nasHost || !nasShare) return;
    setNasTestStatus("testing");
    setNasTestMessage(null);
    try {
      const res = await client.testNAS({
        type: "smb",
        host: nasHost,
        share: nasShare,
        username: nasUsername,
        password: nasPassword,
      });
      if (res.success) {
        setNasTestStatus("success");
        setNasTestMessage(t.nasTestSuccess);
      } else {
        setNasTestStatus("error");
        setNasTestMessage(res.message || "Failed to verify NAS connection");
      }
    } catch (err: any) {
      setNasTestStatus("error");
      setNasTestMessage(err.detail || err.message || "Failed to test NAS connection");
    }
  };

  const handleMountNAS = async () => {
    if (!nasHost || !nasShare) return;
    setNasMountStatus("mounting");
    setNasMountMessage(null);
    try {
      await client.mountNAS({
        type: "smb",
        host: nasHost,
        share: nasShare,
        username: nasUsername,
        password: nasPassword,
      });
      setNasMountStatus("success");
      setNasMountMessage(t.nasMountSuccess);
      void loadStorageState();
    } catch (err: any) {
      setNasMountStatus("error");
      setNasMountMessage(err.detail || err.message || "Failed to mount NAS share");
    }
  };

  // Release Channel & Version State (Story 10.2)
  const [updateChannel, setUpdateChannel] = useState<string>("stable");
  const [currentVersion, setCurrentVersion] = useState<string>("v0.1.0");

  useEffect(() => {
    client
      .getSettings()
      .then((res: any) => {
        if (res?.update_channel) {
          setUpdateChannel(res.update_channel);
        }
      })
      .catch(() => {});
    client
      .getHealth()
      .then((h: any) => {
        if (h?.version) {
          setCurrentVersion(h.version);
        }
      })
      .catch(() => {});
  }, [client]);

  const handleChannelChange = async (newChannel: string) => {
    await client.updateSettings({ update_channel: newChannel });
    setUpdateChannel(newChannel);
  };

  // Destructive Actions State
  const [destructiveModalState, setDestructiveModalState] = useState<{
    isOpen: boolean;
    actionType: DestructiveActionType;
    serviceId?: string;
    serviceName?: string;
  }>({
    isOpen: false,
    actionType: "reboot",
  });
  const [isDestructiveProcessing, setIsDestructiveProcessing] = useState(false);
  const [destructiveError, setDestructiveError] = useState<string | null>(null);

  const handleConfirmDestructive = async () => {
    if (destructiveModalState.actionType === "deactivate" && !destructiveModalState.serviceId) {
      return;
    }
    setIsDestructiveProcessing(true);
    setDestructiveError(null);
    try {
      if (destructiveModalState.actionType === "reboot") {
        await client.rebootSystem();
        setDestructiveModalState({ isOpen: false, actionType: "reboot" });
      } else if (destructiveModalState.actionType === "factory-reset") {
        await client.factoryReset("RESET");
        setDestructiveModalState({ isOpen: false, actionType: "factory-reset" });
      } else if (destructiveModalState.actionType === "preview-channel") {
        await client.updateSettings({ update_channel: "preview", confirmation: "PREVIEW" });
        setUpdateChannel("preview");
        setDestructiveModalState({ isOpen: false, actionType: "preview-channel" });
      } else if (destructiveModalState.actionType === "deactivate" && destructiveModalState.serviceId) {
        await client.deactivateService(destructiveModalState.serviceId);
        setDestructiveModalState({ isOpen: false, actionType: "deactivate" });
      }
    } catch (err: any) {
      setDestructiveError(err.detail || err.message || "Operation failed");
    } finally {
      setIsDestructiveProcessing(false);
    }
  };

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
  const [actionError, setActionError] = useState<{ title: string; detail?: string } | null>(initialActionError);
  const [sliderGpuLayers, setSliderGpuLayers] = useState<number | null>(null);
  const [sliderIdleTimeout, setSliderIdleTimeout] = useState<number | null>(null);

  // Rollback state
  const [rollbackServiceTarget, setRollbackServiceTarget] = useState<{ id: string; name: string; icon: string } | null>(
    initialRollbackServiceTarget
  );
  const [revisionsMap, setRevisionsMap] = useState<
    Record<string, { currentVersion?: string; previousVersion?: string; snapshotTimestamp?: string }>
  >(initialRevisionsMap || {});
  const [isRollingBack, setIsRollingBack] = useState<boolean>(false);
  const [rollbackSuccess, setRollbackSuccess] = useState<string | null>(null);
  const rollbackToastTimerRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    return () => {
      if (rollbackToastTimerRef.current) {
        clearTimeout(rollbackToastTimerRef.current);
      }
    };
  }, []);

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
        } else if (hash === "storage") {
          setActiveTab("storage");
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
    const tabs = ["services", "storage", "ai", "gpu", "system"];
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

    if (!hasActiveDownloads || activeTab !== "ai") return;

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
  }, [downloads, client, activeTab]);

  useEffect(() => {
    if (!rollbackServiceTarget) return;
    const backendServiceMap: Record<string, string> = {
      video: "jellyfin",
      audio: "navidrome",
      dms: "paperless",
      ai: "llama-server",
    };
    const backendId = backendServiceMap[rollbackServiceTarget.id] || rollbackServiceTarget.id;
    void client
      .getRevisions()
      .then((res) => {
        if (!res?.revisions) return;
        const matching = res.revisions.find(
          (r) => (r.service_id === backendId || r.service_id === rollbackServiceTarget.id) && r.service_id !== ""
        );
        if (matching) {
          setRevisionsMap((prev) => ({
            ...prev,
            [rollbackServiceTarget.id]: {
              currentVersion: matching.new_image || undefined,
              previousVersion: matching.old_image || undefined,
              snapshotTimestamp: matching.created_at || undefined,
            },
          }));
        }
      })
      .catch(() => {});
  }, [rollbackServiceTarget, client]);

  const handleConfirmRollback = async () => {
    if (!rollbackServiceTarget) return;
    const targetServiceId = rollbackServiceTarget.id;
    const backendServiceMap: Record<string, string> = {
      video: "jellyfin",
      audio: "navidrome",
      dms: "paperless",
      ai: "llama-server",
    };
    const backendId = backendServiceMap[targetServiceId] || targetServiceId;
    setIsRollingBack(true);
    try {
      await client.rollbackService(backendId);
      setRollbackSuccess(t.rollbackSuccess);
      setRollbackServiceTarget(null);
      if (rollbackToastTimerRef.current) {
        clearTimeout(rollbackToastTimerRef.current);
      }
      rollbackToastTimerRef.current = setTimeout(() => setRollbackSuccess(null), 4000);
      void client
        .getRevisions()
        .then((res) => {
          if (!res?.revisions) return;
          const matching = res.revisions.find(
            (r) => (r.service_id === backendId || r.service_id === targetServiceId) && r.service_id !== ""
          );
          if (matching) {
            setRevisionsMap((prev) => ({
              ...prev,
              [targetServiceId]: {
                currentVersion: matching.new_image || undefined,
                previousVersion: matching.old_image || undefined,
                snapshotTimestamp: matching.created_at || undefined,
              },
            }));
          }
        })
        .catch(() => {});
    } catch (err: unknown) {
      setRollbackServiceTarget(null);
      let title = t.rollbackFailed;
      let detail = "An unexpected error occurred while rolling back the service.";
      if (err instanceof ProblemDetailsError) {
        title = err.title || title;
        detail = err.detail || detail;
      } else if (err instanceof Error) {
        detail = err.message;
      }
      setActionError({ title, detail });
    } finally {
      setIsRollingBack(false);
    }
  };

  // Model switching handler
  const handleSwitchModel = async (filename: string) => {
    try {
      setActionError(null);
      setActivatingModel(filename);
      await client.setActiveAIModel(filename);
      await loadAIState();
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
      } else if (err instanceof Error) {
        setActionError({ title: "Failed to switch model", detail: err.message });
      }
    } finally {
      setActivatingModel(null);
    }
  };

  // Model deletion handler
  const handleDeleteModel = async () => {
    if (!modelToDelete) return;
    try {
      setActionError(null);
      await client.deleteAIModel(modelToDelete);
      setModelToDelete(null);
      await loadAIState();
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
      } else if (err instanceof Error) {
        setActionError({ title: "Failed to delete model", detail: err.message });
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
      setActionError(null);
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
      setActionError(null);
      await client.cancelAIModelDownload(id);
      const activeDownloads = await client.getAIModelDownloads();
      setDownloads(activeDownloads?.downloads || []);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
      }
    }
  };

  // Parameter update handlers
  const handleContextLengthChange = async (length: number) => {
    if (!aiSettings || aiSettings.context_length === length) return;
    try {
      setActionError(null);
      const updated = await client.updateAISettings({ context_length: length });
      setAiSettings(updated);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
      }
    }
  };

  const handleGpuLayersChange = async (layers: number) => {
    if (!aiSettings) return;
    try {
      setActionError(null);
      const updated = await client.updateAISettings({ gpu_layers: layers });
      setAiSettings(updated);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
      }
    }
  };

  const handleIdleTimeoutChange = async (seconds: number) => {
    if (!aiSettings) return;
    try {
      setActionError(null);
      const updated = await client.updateAISettings({
        idle_timeout_seconds: seconds,
      });
      setAiSettings(updated);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
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
      setActionError(null);
      await client.reloadAIEngine();
      setReloadNotice(t.reloadSuccess);
      await loadAIState();
      setTimeout(() => setReloadNotice(null), 5000);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setActionError({ title: err.title, detail: err.detail });
      } else if (err instanceof Error) {
        setActionError({ title: "Reload failed", detail: err.message });
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
  const isCpuOnly = aiSettings ? !aiSettings.has_gpu : false;

  return (
    <div className="flex min-h-screen flex-col bg-[#F8F9FA] text-[#0F172A]">
      {/* 64px Header */}
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={handleLanguageChange}
        aggregateStatus="operational"
        updateChannel={updateChannel}
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
              onClick={() => selectTab("storage")}
              className={`flex min-h-[44px] items-center gap-2.5 rounded-lg px-3.5 py-2.5 text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                activeTab === "storage"
                  ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                  : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
              }`}
              aria-current={activeTab === "storage" ? "page" : undefined}
              data-testid="tab-storage-btn"
            >
              <span aria-hidden="true">💾</span>
              <span>{t.tabStorage}</span>
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
          {!isAuthenticated ? (
            <section className="rounded-xl border border-amber-200 bg-amber-50/50 p-8 text-center shadow-xs" data-testid="settings-auth-lock">
              <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 text-3xl">
                🔒
              </div>
              <h2 className="mt-4 text-base font-bold text-[#0F172A]">{t.adminAuthTitle}</h2>
              <p className="mt-1 text-xs text-[#64748B] max-w-md mx-auto">{t.adminAuthDesc}</p>
              <button
                type="button"
                onClick={() => setIsLoginModalOpen(true)}
                className="mt-5 inline-flex min-h-[44px] items-center justify-center rounded-xl bg-blue-600 px-6 py-2.5 text-sm font-semibold text-white shadow-xs hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 cursor-pointer"
                data-testid="unlock-settings-btn"
              >
                {t.unlock}
              </button>
            </section>
          ) : (
            <>
          {/* TAB: AI Backend & Models */}
          {activeTab === "ai" && (
            <AISettingsTab
              t={t}
              aiSettings={aiSettings}
              activeModelObj={activeModelObj}
              isCpuOnly={isCpuOnly}
              reloadNotice={reloadNotice}
              setReloadNotice={setReloadNotice}
              actionError={actionError}
              setActionError={setActionError}
              generationPreset={generationPreset}
              onPresetChange={handlePresetChange}
              onContextLengthChange={handleContextLengthChange}
              sliderGpuLayers={sliderGpuLayers}
              setSliderGpuLayers={setSliderGpuLayers}
              onGpuLayersChange={handleGpuLayersChange}
              sliderIdleTimeout={sliderIdleTimeout}
              setSliderIdleTimeout={setSliderIdleTimeout}
              onIdleTimeoutChange={handleIdleTimeoutChange}
              isReloading={isReloading}
              onReloadEngine={handleReloadEngine}
              onOpenLogs={handleOpenLogs}
              models={models}
              activatingModel={activatingModel}
              onSwitchModel={handleSwitchModel}
              setModelToDelete={setModelToDelete}
              downloadError={downloadError}
              setDownloadError={setDownloadError}
              downloadUrl={downloadUrl}
              setDownloadUrl={setDownloadUrl}
              downloadSha256={downloadSha256}
              setDownloadSha256={setDownloadSha256}
              downloadFilename={downloadFilename}
              setDownloadFilename={setDownloadFilename}
              isStartingDownload={isStartingDownload}
              onStartDownload={handleStartDownload}
              downloads={downloads}
              onCancelDownload={handleCancelDownload}
            />
          )}

          {/* TAB: Services & Add-ons */}
          {activeTab === "services" && (
            <ServicesSettingsTab
              t={t}
              onSelectRollbackTarget={setRollbackServiceTarget}
              onOpenDestructiveModal={setDestructiveModalState}
            />
          )}

          {/* TAB: Storage & Pools */}
          {activeTab === "storage" && (
            <StorageSettingsTab
              t={t}
              disks={disks}
              storageStatus={storageStatus}
              nasHost={nasHost}
              setNasHost={setNasHost}
              nasShare={nasShare}
              setNasShare={setNasShare}
              nasUsername={nasUsername}
              setNasUsername={setNasUsername}
              nasPassword={nasPassword}
              setNasPassword={setNasPassword}
              nasTestStatus={nasTestStatus}
              nasTestMessage={nasTestMessage}
              nasMountStatus={nasMountStatus}
              nasMountMessage={nasMountMessage}
              onTestNAS={handleTestNAS}
              onMountNAS={handleMountNAS}
            />
          )}

          {/* TAB: Smart Hardware Sharing */}
          {activeTab === "gpu" && <HardwareSharingTab t={t} />}

          {/* TAB: System & Hardware */}
          {activeTab === "system" && (
            <SystemSettingsTab
              t={t}
              currentChannel={updateChannel}
              currentVersion={currentVersion}
              onChannelChange={handleChannelChange}
              onOpenDestructiveModal={setDestructiveModalState}
            />
          )}
            </>
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

      {/* Service Rollback Confirmation Modal */}
      {rollbackServiceTarget && (
        <RollbackModal
          isOpen={true}
          serviceId={rollbackServiceTarget.id}
          serviceName={rollbackServiceTarget.name}
          serviceIcon={rollbackServiceTarget.icon}
          currentVersion={revisionsMap[rollbackServiceTarget.id]?.currentVersion}
          previousVersion={revisionsMap[rollbackServiceTarget.id]?.previousVersion}
          snapshotTimestamp={revisionsMap[rollbackServiceTarget.id]?.snapshotTimestamp}
          isRollingBack={isRollingBack}
          onConfirm={handleConfirmRollback}
          onClose={() => setRollbackServiceTarget(null)}
          currentLanguage={lang}
        />
      )}

      {/* Rollback Success Toast Notification */}
      {rollbackSuccess && (
        <div
          role="status"
          aria-live="polite"
          className="fixed bottom-6 right-6 z-50 flex items-center gap-2.5 rounded-xl border border-emerald-200 bg-white px-4 py-3 text-sm text-emerald-900 shadow-xl animate-in fade-in slide-in-from-bottom-2"
          data-testid="settings-rollback-success-toast"
        >
          <span className="text-base" aria-hidden="true">
            ✅
          </span>
          <span className="font-medium">{rollbackSuccess}</span>
          <button
            type="button"
            onClick={() => setRollbackSuccess(null)}
            className="ml-2 text-xs text-slate-400 hover:text-slate-600"
            aria-label={t.close}
          >
            ✕
          </button>
        </div>
      )}

      {/* RFC 7807 Problem Details Modal */}
      {actionError && (
        <div
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="settings-error-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          data-testid="problem-details-modal"
        >
          <div className="w-full max-w-md rounded-xl border border-red-200 bg-white p-6 shadow-xl text-[#0F172A]">
            <div className="flex items-center gap-3">
              <span className="text-2xl text-red-600" aria-hidden="true">
                ⚠️
              </span>
              <h3
                id="settings-error-title"
                className="text-base font-semibold text-[#0F172A]"
              >
                {actionError.title}
              </h3>
            </div>
            {actionError.detail && (
              <p className="mt-3 text-xs sm:text-sm text-[#475569] leading-relaxed">
                {actionError.detail}
              </p>
            )}
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                onClick={() => setActionError(null)}
                className="h-11 rounded-lg bg-[#0F172A] px-4 text-xs font-medium text-white hover:bg-[#1E293B] focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
                data-testid="close-problem-details"
              >
                {t.close}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Administrator Login Modal */}
      <AdminLoginModal
        isOpen={isLoginModalOpen}
        onClose={() => setIsLoginModalOpen(false)}
        onSuccess={() => {
          setIsAuthenticated(true);
          setIsLoginModalOpen(false);
        }}
        apiClient={client}
        currentLanguage={lang}
      />

      {/* Destructive Protection Modal */}
      <DestructiveModal
        isOpen={destructiveModalState.isOpen}
        actionType={destructiveModalState.actionType}
        serviceName={destructiveModalState.serviceName}
        isProcessing={isDestructiveProcessing}
        errorMessage={destructiveError}
        onConfirm={handleConfirmDestructive}
        onClose={() => setDestructiveModalState((prev) => ({ ...prev, isOpen: false }))}
        currentLanguage={lang}
      />
    </div>
  );
}
