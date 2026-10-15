import React from "react";

export type StatusLevel = "healthy" | "operational" | "amber" | "degraded" | "red" | "error" | "stopped";

export interface StatusBadgeProps {
  status: StatusLevel;
  label?: string;
  className?: string;
}

export function StatusBadge({ status, label, className = "" }: StatusBadgeProps) {
  const isGreen = status === "healthy" || status === "operational";
  const isAmber = status === "amber" || status === "degraded";
  const isRed = status === "red" || status === "error" || status === "stopped";

  let bgClasses = "bg-emerald-50 text-emerald-800 border-emerald-200";
  let symbol = "●";
  let defaultLabel = "Operational";

  if (isAmber) {
    bgClasses = "bg-amber-50 text-amber-800 border-amber-200";
    symbol = "◆";
    defaultLabel = "Attention Required";
  } else if (isRed) {
    bgClasses = "bg-red-50 text-red-800 border-red-200";
    symbol = "▲";
    defaultLabel = "Stopped / Error";
  }

  const displayText = label || defaultLabel;

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium border ${bgClasses} ${className}`}
      data-testid="status-badge"
    >
      <span aria-hidden="true" className="text-[10px] leading-none">
        {symbol}
      </span>
      <span>{displayText}</span>
    </span>
  );
}
