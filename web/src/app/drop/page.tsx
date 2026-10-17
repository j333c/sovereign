"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Language, translations, getInitialLanguage, persistLanguage } from "@/lib/i18n";
import { apiClient, DropHistoryItem, ProblemDetailsError } from "@/lib/api/client";

export type MediaCategory = "movies" | "tv" | "music" | "audiobooks";

interface ActiveUpload {
  id: string;
  name: string;
  size: number;
  category: MediaCategory;
  progress: number;
  status: "uploading" | "success" | "error";
  errorMessage?: string;
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export interface DropzonePageProps {
  initialLanguage?: Language;
}

export default function DropzonePage({ initialLanguage }: DropzonePageProps = {}) {
  const [lang, setLang] = useState<Language>(() => {
    return getInitialLanguage(initialLanguage);
  });
  const t = translations[lang];

  const handleLanguageChange = (newLang: Language) => {
    setLang(newLang);
    persistLanguage(newLang);
  };

  const [category, setCategory] = useState<MediaCategory>("movies");
  const [isDragging, setIsDragging] = useState(false);
  const [activeUploads, setActiveUploads] = useState<ActiveUpload[]>([]);
  const [history, setHistory] = useState<DropHistoryItem[]>([]);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [alert, setAlert] = useState<{ type: "success" | "error"; message: string } | null>(null);
  const [copiedTarget, setCopiedTarget] = useState<string | null>(null);
  const [isMounted, setIsMounted] = useState(false);

  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setIsMounted(true);
    loadHistory();
  }, []);

  async function loadHistory() {
    try {
      setHistoryLoading(true);
      const items = await apiClient.getDropHistory();
      setHistory(items);
    } catch {
      // Non-fatal; history will be empty
    } finally {
      setHistoryLoading(false);
    }
  }

  const handleCopyPath = async (path: string, key: string) => {
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(path);
        setCopiedTarget(key);
        setTimeout(() => {
          setCopiedTarget(null);
        }, 2000);
      }
    } catch {
      // Clipboard write failed
    }
  };

  const handleFiles = async (files: FileList | null) => {
    if (!files || files.length === 0) return;

    setAlert(null);
    const newUploads: ActiveUpload[] = Array.from(files).map((f) => ({
      id: `${Date.now()}-${f.name}`,
      name: f.name,
      size: f.size,
      category,
      progress: 0,
      status: "uploading",
    }));

    setActiveUploads((prev) => [...newUploads, ...prev]);

    // Upload files with real-time byte progress reporting
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const uploadId = newUploads[i].id;

      try {
        await apiClient.uploadDropMedia(category, file, undefined, (progress) => {
          setActiveUploads((prev) =>
            prev.map((u) => (u.id === uploadId ? { ...u, progress } : u))
          );
        });

        setActiveUploads((prev) =>
          prev.map((u) => (u.id === uploadId ? { ...u, progress: 100, status: "success" } : u))
        );

        setAlert({
          type: "success",
          message: t.uploadSuccess,
        });

        // Refresh recent uploads history
        loadHistory();
      } catch (err) {
        let errorMsg = t.onlyMediaAccepted;
        if (err instanceof ProblemDetailsError) {
          if (err.status === 422) {
            errorMsg = t.onlyMediaAccepted;
          } else if (err.detail) {
            errorMsg = err.detail;
          }
        }

        setActiveUploads((prev) =>
          prev.map((u) =>
            u.id === uploadId ? { ...u, progress: 100, status: "error", errorMessage: errorMsg } : u
          )
        );

        setAlert({
          type: "error",
          message: errorMsg,
        });
      }
    }

    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const categoryLabels: Record<MediaCategory, string> = {
    movies: t.categoryMovies,
    tv: t.categoryTV,
    music: t.categoryMusic,
    audiobooks: t.categoryAudiobooks,
  };

  return (
    <div className="min-h-screen bg-[#F8F9FA] text-[#0F172A] flex flex-col font-sans">
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={handleLanguageChange}
        aggregateStatus="healthy"
      />

      <main className="flex-1 w-full max-w-4xl mx-auto px-4 py-8 sm:py-12 flex flex-col gap-8">
        {/* Page Title & Reassurance Header */}
        <div className="flex flex-col items-center text-center gap-2">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-[#ECFDF5] text-[#065F46] text-xs font-medium border border-[#A7F3D0]">
            <span aria-hidden="true">●</span>
            <span>{t.crestTitle} · {t.storageIngestionReady}</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-[#0F172A]">
            {t.dropzoneTitle}
          </h1>
          <p className="text-sm text-[#64748B] max-w-lg">
            {t.dropzoneSubtitle}
          </p>
        </div>

        {/* Dismissible Alert Banner */}
        {alert && (
          <div
            role="alert"
            className={`flex items-start justify-between p-4 rounded-xl border text-sm transition-all ${
              alert.type === "error"
                ? "bg-[#FEF2F2] border-[#FECACA] text-[#991B1B]"
                : "bg-[#ECFDF5] border-[#A7F3D0] text-[#065F46]"
            }`}
          >
            <div className="flex items-center gap-2 font-medium">
              <span aria-hidden="true" className="text-base">
                {alert.type === "error" ? "▲" : "✓"}
              </span>
              <span>{alert.message}</span>
            </div>
            <button
              type="button"
              onClick={() => setAlert(null)}
              className="min-h-[44px] min-w-[44px] flex items-center justify-center -mr-2 -my-2 text-current opacity-70 hover:opacity-100 rounded-md focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
              aria-label={t.close}
            >
              ✕
            </button>
          </div>
        )}

        {/* Central Dropzone Card */}
        <div className="w-full max-w-[680px] mx-auto bg-white rounded-xl border border-[#E5E7EB] p-6 sm:p-8 shadow-xs flex flex-col gap-6">
          {/* Category Selector Tabs */}
          <div className="flex flex-col gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-[#64748B]">
              {t.destinationLibrary}
            </span>
            <div
              role="tablist"
              aria-label={t.mediaCategoryTabs}
              className="grid grid-cols-2 sm:grid-cols-4 gap-1 p-1 bg-[#F8F9FA] rounded-xl border border-[#E5E7EB]"
            >
              {(["movies", "tv", "music", "audiobooks"] as MediaCategory[]).map((cat) => {
                const isSelected = category === cat;
                return (
                  <button
                    key={cat}
                    type="button"
                    role="tab"
                    id={`tab-${cat}`}
                    aria-selected={isSelected}
                    aria-controls={`panel-${cat}`}
                    onClick={() => setCategory(cat)}
                    className={`min-h-[44px] flex items-center justify-center rounded-lg text-xs sm:text-sm font-medium transition-all focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer ${
                      isSelected
                        ? "bg-white text-[#0F172A] shadow-xs font-semibold"
                        : "text-[#64748B] hover:text-[#0F172A]"
                    }`}
                  >
                    {categoryLabels[cat]}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Drag & Drop Target Area */}
          <div
            id={`panel-${category}`}
            role="button"
            tabIndex={0}
            aria-label={`${t.dragDropPrompt} for ${categoryLabels[category]}`}
            onClick={() => fileInputRef.current?.click()}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                fileInputRef.current?.click();
              }
            }}
            onDragOver={(e) => {
              e.preventDefault();
              setIsDragging(true);
            }}
            onDragLeave={(e) => {
              e.preventDefault();
              setIsDragging(false);
            }}
            onDrop={(e) => {
              e.preventDefault();
              setIsDragging(false);
              handleFiles(e.dataTransfer.files);
            }}
            className={`border-2 border-dashed rounded-xl p-8 sm:p-12 text-center transition-all cursor-pointer flex flex-col items-center justify-center gap-3 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none ${
              isDragging
                ? "border-[#0F172A] bg-[#F1F5F9]/80 ring-4 ring-[#0F172A]/10 scale-[1.01]"
                : "border-[#CBD5E1] hover:border-[#0F172A] bg-[#F8F9FA]/40 hover:bg-[#F8F9FA]"
            }`}
          >
            <input
              type="file"
              ref={fileInputRef}
              multiple
              className="hidden"
              onChange={(e) => handleFiles(e.target.files)}
              aria-hidden="true"
            />

            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[#0F172A] text-white text-xl shadow-xs">
              📥
            </div>

            <div className="flex flex-col gap-1">
              <span className="text-sm sm:text-base font-semibold text-[#0F172A]">
                {t.dragDropPrompt}
              </span>
              <span className="text-xs text-[#64748B]">
                {t.supportedFormats}
              </span>
            </div>
          </div>

          {/* Active Uploads Progress */}
          {activeUploads.length > 0 && (
            <div
              className="flex flex-col gap-3 pt-2 border-t border-[#E5E7EB]"
              aria-live="polite"
            >
              <span className="text-xs font-semibold uppercase tracking-wider text-[#64748B]">
                Active Uploads
              </span>
              <div className="flex flex-col gap-2">
                {activeUploads.map((upload) => (
                  <div
                    key={upload.id}
                    className="flex flex-col gap-1.5 p-3 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA]"
                  >
                    <div className="flex items-center justify-between text-xs">
                      <div className="flex items-center gap-2 truncate max-w-[70%]">
                        <span className="font-medium text-[#0F172A] truncate">
                          {upload.name}
                        </span>
                        <span className="text-[#64748B]">
                          ({formatBytes(upload.size)})
                        </span>
                      </div>
                      <span className="font-semibold text-xs text-[#0F172A]">
                        {upload.status === "uploading" && `${upload.progress}%`}
                        {upload.status === "success" && "✓ Done"}
                        {upload.status === "error" && "▲ Failed"}
                      </span>
                    </div>

                    <div
                      role="progressbar"
                      aria-valuenow={upload.progress}
                      aria-valuemin={0}
                      aria-valuemax={100}
                      aria-label={`Upload progress for ${upload.name}`}
                      className="w-full h-2 rounded-full bg-[#E5E7EB] overflow-hidden"
                    >
                      <div
                        className={`h-full transition-all duration-300 rounded-full ${
                          upload.status === "error"
                            ? "bg-[#EF4444]"
                            : upload.status === "success"
                            ? "bg-[#10B981]"
                            : "bg-[#0F172A]"
                        }`}
                        style={{ width: `${upload.progress}%` }}
                      />
                    </div>

                    {upload.errorMessage && (
                      <span className="text-[11px] text-[#991B1B] font-medium">
                        {upload.errorMessage}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Copyable Network Share Path Helper */}
          <div className="flex flex-col gap-3 pt-4 border-t border-[#E5E7EB]">
            <div className="flex flex-col">
              <span className="text-xs font-semibold uppercase tracking-wider text-[#64748B]">
                {t.networkShareTitle}
              </span>
              <p className="text-xs text-[#64748B] mt-0.5">
                {t.networkShareDesc}
              </p>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {/* Windows UNC Path */}
              <div className="flex items-center justify-between p-2.5 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] text-xs">
                <div className="flex flex-col truncate pr-2">
                  <span className="text-[10px] uppercase font-semibold text-[#64748B]">
                    Windows Share
                  </span>
                  <code className="font-mono text-[#0F172A] truncate">
                    \\sovereign\drop
                  </code>
                </div>
                <button
                  type="button"
                  onClick={() => handleCopyPath("\\\\sovereign\\drop", "win")}
                  className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-md px-3 text-xs font-medium text-[#0F172A] hover:bg-white border border-transparent hover:border-[#E5E7EB] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
                  aria-label="Copy Windows drop path \\sovereign\drop"
                >
                  {copiedTarget === "win" ? "✓ Copied" : t.copyPath}
                </button>
              </div>

              {/* macOS / Linux SMB Path */}
              <div className="flex items-center justify-between p-2.5 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] text-xs">
                <div className="flex flex-col truncate pr-2">
                  <span className="text-[10px] uppercase font-semibold text-[#64748B]">
                    macOS / Linux
                  </span>
                  <code className="font-mono text-[#0F172A] truncate">
                    smb://sovereign.local/drop
                  </code>
                </div>
                <button
                  type="button"
                  onClick={() => handleCopyPath("smb://sovereign.local/drop", "smb")}
                  className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-md px-3 text-xs font-medium text-[#0F172A] hover:bg-white border border-transparent hover:border-[#E5E7EB] transition-colors focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none cursor-pointer"
                  aria-label="Copy SMB drop path smb://sovereign.local/drop"
                >
                  {copiedTarget === "smb" ? "✓ Copied" : t.copyPath}
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* Recent Ingestion History */}
        <div className="w-full max-w-[680px] mx-auto bg-white rounded-xl border border-[#E5E7EB] p-6 shadow-xs flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-[#0F172A]">
              {t.recentUploads}
            </h2>
            <button
              type="button"
              onClick={loadHistory}
              className="text-xs text-[#64748B] hover:text-[#0F172A] rounded px-2 py-1 focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:outline-none"
              aria-label="Refresh upload history"
            >
              🔄 Refresh
            </button>
          </div>

          {historyLoading && history.length === 0 ? (
            <div className="py-6 text-center text-xs text-[#64748B]">
              Loading history...
            </div>
          ) : history.length === 0 ? (
            <div className="py-6 text-center text-xs text-[#64748B]">
              {t.noRecentUploads}
            </div>
          ) : (
            <div className="flex flex-col divide-y divide-[#E5E7EB]">
              {history.map((item) => {
                const isPromoted = item.status === "promoted";
                return (
                  <div
                    key={item.id}
                    className="flex items-center justify-between py-3 first:pt-0 last:pb-0 text-xs"
                  >
                    <div className="flex flex-col truncate max-w-[65%]">
                      <span className="font-medium text-[#0F172A] truncate">
                        {item.filename}
                      </span>
                      <div className="flex items-center gap-2 text-[11px] text-[#64748B] mt-0.5">
                        <span className="capitalize font-medium">
                          {item.category}
                        </span>
                        <span>•</span>
                        <span>{formatBytes(item.size)}</span>
                        {item.uploaded_at && isMounted && (
                          <>
                            <span>•</span>
                            <span suppressHydrationWarning>
                              {new Date(item.uploaded_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                            </span>
                          </>
                        )}
                      </div>
                    </div>

                    <div>
                      {isPromoted ? (
                        <StatusBadge status="healthy" label="Indexed" />
                      ) : (
                        <StatusBadge status="error" label="Rejected" />
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
