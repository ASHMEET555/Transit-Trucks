"use client"

import React from "react"
import { RealtimeStatus } from "@/lib/realtime/useFleetEvents"

interface RealtimeIndicatorProps {
  status: RealtimeStatus
  className?: string
}

export function RealtimeIndicator({ status, className = "" }: RealtimeIndicatorProps) {
  const getStatusConfig = () => {
    switch (status) {
      case "live":
        return {
          label: "Live Fleet Sync",
          badgeClass: "bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-800",
          dotClass: "bg-emerald-500 animate-pulse",
        }
      case "reconnecting":
        return {
          label: "Reconnecting...",
          badgeClass: "bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-400 dark:border-amber-800",
          dotClass: "bg-amber-500 animate-ping",
        }
      case "connecting":
        return {
          label: "Connecting...",
          badgeClass: "bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/40 dark:text-blue-400 dark:border-blue-800",
          dotClass: "bg-blue-500",
        }
      case "offline":
      default:
        return {
          label: "Offline",
          badgeClass: "bg-gray-100 text-gray-600 border-gray-300 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700",
          dotClass: "bg-gray-400",
        }
    }
  }

  const config = getStatusConfig()

  return (
    <div
      role="status"
      aria-live="polite"
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${config.badgeClass} ${className}`}
    >
      <span className={`w-2 h-2 rounded-full shrink-0 ${config.dotClass}`} aria-hidden="true" />
      <span>{config.label}</span>
    </div>
  )
}
