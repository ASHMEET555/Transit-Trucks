"use client"

import { useProfile } from "@/lib/ProfileContext"
import { LiveMapDashboard } from "@/components/live-map/live-map-dashboard"

export default function LiveMapPage() {
  const { currentProfile, isLoaded } = useProfile()

  if (!isLoaded) {
    return (
      <div className="grid min-h-[420px] place-items-center rounded-3xl border border-border bg-card" role="status">
        <p className="text-sm text-muted-foreground">Loading live map…</p>
      </div>
    )
  }

  return <LiveMapDashboard userRole={currentProfile?.role} />
}
