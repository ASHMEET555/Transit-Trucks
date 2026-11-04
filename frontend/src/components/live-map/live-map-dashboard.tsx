"use client"

import dynamic from "next/dynamic"
import React, { useMemo, useState } from "react"
import type { Map as LeafletMap } from "leaflet"
import { AlertTriangle, Clock, Gauge, Route as RouteIcon, Truck } from "lucide-react"
import { MAP_CENTER, trucks as fallbackTrucks, viewerProfiles, type ViewerRole } from "@/lib/data/transit-data"
import { useLiveVehicles } from "@/lib/live/useLiveVehicles"
import { AppSidebar } from "./app-sidebar"
import { LiveBadge, MapControls } from "./map-controls"
import { RoleFilter } from "./role-filter"
import { RoutePanel } from "./route-panel"
import { StatPills, type StatPill } from "./stat-pills"
import { TopBar } from "./top-bar"
import type { LiveTruckItem } from "./fleet-map"

const FleetMap = dynamic(() => import("./fleet-map"), {
  ssr: false,
  loading: () => (
    <div className="grid size-full place-items-center bg-muted/40 text-sm text-muted-foreground">
      Loading freight telemetry map…
    </div>
  ),
})

export function LiveMapDashboard({ userRole = "Fleet Manager" }: { userRole?: string }) {
  const initialViewer: ViewerRole = userRole === "Driver" ? "driver" : "operator"
  const [role, setRole] = useState<ViewerRole>(initialViewer)
  const [paused, setPaused] = useState(false)
  const [query, setQuery] = useState("")
  const [selectedTruckId, setSelectedTruckId] = useState<string | null>(null)
  const [showRoutes, setShowRoutes] = useState(true)
  const [map, setMap] = useState<LeafletMap | null>(null)

  const { vehicles: liveData } = useLiveVehicles({ enabled: !paused })

  // Map live backend telemetry to LiveTruckItem array
  const trucks: LiveTruckItem[] = useMemo(() => {
    if (liveData && liveData.length > 0) {
      return liveData.map((v) => ({
        id: v.vehicle_reg,
        vehicle_reg: v.vehicle_reg,
        trip_id: v.trip_id,
        driver_name: (v as any).driver_name || null,
        cargo_type: (v as any).cargo_type || "Semi",
        cargo_weight: (v as any).cargo_weight || 18000,
        speed_kmh: v.speed_kmh,
        fuel_pct: (v as any).fuel_pct,
        reefer_temp_c: (v as any).reefer_temp_c,
        heading_deg: v.heading_deg,
        latitude: v.latitude,
        longitude: v.longitude,
        progress_pct: v.progress_pct,
        miles_remaining: (v as any).miles_remaining,
        eta: (v as any).eta,
        delay_minutes: (v as any).delay_minutes,
        status: (v as any).status || "on_route",
        alerts: (v as any).alerts || [],
        source: v.source || "simulator",
        route_polyline: v.route_polyline,
        route_provider: v.route_provider,
        origin: v.origin,
        destination: v.destination,
      }))
    }
    // Initial fallback if backend is still starting
    return fallbackTrucks.map((t) => ({
      id: t.id,
      vehicle_reg: t.code,
      trip_id: 1001,
      driver_name: t.driver.name,
      cargo_type: t.cargoType || "Semi",
      cargo_weight: t.cargoWeight,
      speed_kmh: 84,
      fuel_pct: t.fuelPct ?? 78,
      reefer_temp_c: t.reeferTempC,
      heading_deg: 90,
      latitude: 39.5296,
      longitude: -119.8138,
      progress_pct: t.startProgress * 100,
      miles_remaining: 320,
      eta: "14:20",
      delay_minutes: t.delayed ? 22 : 0,
      status: t.delayed ? "delayed" : "on_route",
      alerts: t.delayed ? ["delayed"] : [],
      source: "simulator",
      origin: { name: "Reno Hub", latitude: 39.5296, longitude: -119.8138 },
      destination: { name: "Salt Lake City Hub", latitude: 40.7608, longitude: -111.891 },
    }))
  }, [liveData])

  function changeRole(nextRole: ViewerRole) {
    setRole(nextRole)
    if (nextRole === "driver") {
      setSelectedTruckId(trucks[0]?.vehicle_reg ?? null)
    } else {
      setSelectedTruckId(null)
    }
  }

  const roleScopedTrucks = useMemo(() => {
    if (role === "driver") {
      return trucks.slice(0, 1)
    }
    return trucks
  }, [role, trucks])

  const profile = viewerProfiles[role]
  const stats = buildStats(roleScopedTrucks)

  return (
    <div className="flex min-h-dvh bg-canvas p-0 md:p-6">
      <div className="flex w-full overflow-hidden bg-background md:rounded-[2rem] md:border md:border-border md:shadow-2xl">
        <AppSidebar role={userRole} />
        <div className="flex min-w-0 flex-1 flex-col">
          <TopBar
            query={query}
            onQueryChange={setQuery}
            viewerName={profile.name}
            viewerSubtitle={profile.subtitle}
          />

          <main className="flex flex-1 flex-col gap-4 p-4 md:p-6">
            <nav aria-label="Breadcrumb" className="text-xs text-muted-foreground">
              <ol className="flex items-center gap-1.5">
                <li>Overview</li>
                <li aria-hidden="true">&gt;</li>
                <li className="font-medium text-foreground" aria-current="page">
                  Live freight map
                </li>
              </ol>
            </nav>

            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <StatPills items={stats} />
              <RoleFilter value={role} onChange={changeRole} />
            </div>

            <div className="relative flex flex-1 flex-col gap-4 md:block md:min-h-[560px]">
              <div className="relative h-[65vh] overflow-hidden rounded-3xl border border-border md:absolute md:inset-0 md:h-auto">
                <FleetMap
                  fleet={roleScopedTrucks}
                  selectedTruckId={selectedTruckId}
                  showRoutes={showRoutes}
                  onSelectTruck={setSelectedTruckId}
                  onReady={setMap}
                />
                <LiveBadge paused={paused} onToggle={() => setPaused((p) => !p)} />
                <MapControls
                  showRoutes={showRoutes}
                  onShowRoutesChange={setShowRoutes}
                  onZoomIn={() => map?.zoomIn()}
                  onZoomOut={() => map?.zoomOut()}
                  onRecenter={() => {
                    setSelectedTruckId(null)
                    map?.flyTo(MAP_CENTER, 4, { duration: 1.0 })
                  }}
                />
              </div>

              {/* Panel on Left Side */}
              <div className="md:absolute md:bottom-4 md:left-4 md:top-4 md:z-[500] md:w-[350px]">
                <RoutePanel
                  trucks={roleScopedTrucks}
                  selectedTruckId={selectedTruckId}
                  onSelectTruck={setSelectedTruckId}
                />
              </div>
            </div>
          </main>
        </div>
      </div>
    </div>
  )
}

function buildStats(trucksList: LiveTruckItem[]): StatPill[] {
  const onRoute = trucksList.filter((b) => b.status === "on_route").length
  const delayed = trucksList.filter((b) => b.status === "delayed" || (b.delay_minutes ?? 0) > 0).length
  const alerts = trucksList.filter((b) => b.alerts && b.alerts.length > 0).length

  return [
    { label: "Active trucks", value: trucksList.length, icon: Truck, tone: "primary" },
    { label: "On route", value: onRoute, icon: Gauge, tone: "sage" },
    { label: "Delayed", value: delayed, icon: Clock, tone: "signal" },
    { label: "Alerts", value: alerts, icon: AlertTriangle, tone: alerts > 0 ? "signal" : "mist" },
  ]
}
