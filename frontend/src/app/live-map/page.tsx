"use client"

import { useProfile } from "@/lib/ProfileContext"
import { LiveMapDashboard } from "@/components/live-map/live-map-dashboard"

export default function LiveMapPage() {
  const { currentProfile } = useProfile()
  return <LiveMapDashboard userRole={currentProfile?.role} />
}
