"use client"

import { cn } from "@/lib/utils"
import { statusPillLabel } from "@/lib/live/liveUtils.mjs"

export function StatusPill({
  source,
  ageSeconds,
  hasPosition,
  paused,
  className,
}: {
  source: string | null
  ageSeconds: number | null
  hasPosition: boolean
  paused: boolean
  className?: string
}) {
  const label = statusPillLabel({ source, ageSeconds, hasPosition, paused })
  const live = !paused && hasPosition && source !== "simulator" && (ageSeconds ?? 9999) < 60
  return (
    <p
      role="status"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset",
        live
          ? "bg-signal/15 text-signal-foreground ring-signal/30"
          : "bg-muted text-muted-foreground ring-foreground/10",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "size-1.5 rounded-full",
          !hasPosition || paused ? "bg-muted-foreground" : live ? "bg-signal" : "bg-chart-3",
        )}
      />
      {label}
    </p>
  )
}
