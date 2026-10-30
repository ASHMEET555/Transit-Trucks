import { DotPill } from "./chrome"
import type { TripStatus } from "./types"

const CONFIG: Record<TripStatus, { dot: string; pill: string }> = {
  Draft: { dot: "bg-amber-600", pill: "bg-sand text-foreground" },
  Dispatched: { dot: "bg-chart-5", pill: "bg-sage text-foreground" },
  Completed: { dot: "bg-primary", pill: "bg-mist text-foreground" },
  Cancelled: { dot: "bg-destructive", pill: "bg-destructive/12 text-destructive" },
}

export function StatusBadge({ status }: { status: string }) {
  const config = (CONFIG as Record<string, (typeof CONFIG)[TripStatus]>)[status] ?? {
    dot: "bg-muted-foreground",
    pill: "bg-muted text-muted-foreground",
  }
  return (
    <DotPill dot={config.dot} pill={config.pill}>
      {status}
    </DotPill>
  )
}
