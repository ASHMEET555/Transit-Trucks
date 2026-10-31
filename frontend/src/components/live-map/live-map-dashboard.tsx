"use client"

import dynamic from "next/dynamic"
import { useState } from "react"
import type { Map as LeafletMap } from "leaflet"
import { CircleCheck, Clock, Gauge, Route as RouteIcon, Truck } from "lucide-react"
import { etaMinutes, getRoute, nextStop, routes, viewerProfiles, type ViewerRole } from "@/lib/data/transit-data"
import { useFleetSimulation } from "@/lib/data/use-fleet-simulation"
import { AppSidebar } from "./app-sidebar"
import { LiveBadge, MapControls } from "./map-controls"
import { RoleFilter } from "./role-filter"
import { RoutePanel, type StatusFilter } from "./route-panel"
import { StatPills, type StatPill } from "./stat-pills"
import { TopBar } from "./top-bar"

const FleetMap = dynamic(() => import("./fleet-map"), {
  ssr: false,
  loading: () => (
    <div className="grid size-full place-items-center bg-muted/40 text-sm text-muted-foreground">
      Loading live freight map…
    </div>
  ),
})

export function LiveMapDashboard({ userRole = "Fleet Manager" }: { userRole?: string }) {
  const initialViewer: ViewerRole = userRole === "Driver" ? "driver" : "operator"
  const [role, setRole] = useState<ViewerRole>(initialViewer)
  const [paused, setPaused] = useState(false)
  const [query, setQuery] = useState("")
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all")
  const [fleetFilter, setFleetFilter] = useState("all")
  const [selectedTruckId, setSelectedTruckId] = useState<string | null>(null)
  const [showRoutes, setShowRoutes] = useState(true)
  const [, setRecenterCount] = useState(0)
  const [map, setMap] = useState<LeafletMap | null>(null)

  const { fleet, log } = useFleetSimulation(paused)

  function changeRole(nextRole: ViewerRole) {
    setRole(nextRole)
    if (nextRole === "driver") {
      setSelectedTruckId(viewerProfiles.driver.truckId)
      setStatusFilter("all")
      setFleetFilter("all")
    } else {
      setSelectedTruckId(null)
    }
  }

  const roleScopedTrucks = role === "driver" ? fleet.filter((b) => b.id === viewerProfiles.driver.truckId) : fleet

  const q = query.trim().toLowerCase()
  const visibleTrucks = roleScopedTrucks.filter((truck) => {
    if (statusFilter === "active" && truck.status === "completed") return false
    if (statusFilter === "completed" && truck.status !== "completed") return false
    if (fleetFilter !== "all" && truck.fleetId !== fleetFilter) return false
    if (!q) return true
    const route = getRoute(truck.routeId)
    return (
      truck.code.toLowerCase().includes(q) ||
      truck.plate.toLowerCase().includes(q) ||
      truck.driver.name.toLowerCase().includes(q) ||
      route.name.toLowerCase().includes(q) ||
      String(route.number).includes(q)
    )
  })

  const effectiveSelectedId =
    selectedTruckId && visibleTrucks.some((b) => b.id === selectedTruckId)
      ? selectedTruckId
      : role === "driver"
        ? visibleTrucks[0]?.id ?? null
        : null

  const visibleRouteIds = Array.from(new Set(visibleTrucks.map((b) => b.routeId)))
  const completedCount = fleet.reduce((acc, b) => acc + b.tripsCompleted, 0)
  const stats = buildStats(role, visibleTrucks, completedCount)
  const profile = viewerProfiles[role]
  const focusKey = `${role}-${selectedTruckId ?? "all"}-${visibleRouteIds.join(",")}`

  return (
    <div className="flex min-h-dvh bg-canvas p-0 md:p-6">
      <div className="flex w-full overflow-hidden bg-background md:rounded-[2rem] md:border md:border-border md:shadow-2xl">
        <AppSidebar role={userRole} />
        <div className="flex min-w-0 flex-1 flex-col">
          <TopBar query={query} onQueryChange={setQuery} viewerName={profile.name} viewerSubtitle={profile.subtitle} />

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
              <div className="relative h-[60vh] overflow-hidden rounded-3xl border border-border md:absolute md:inset-0 md:h-auto">
                <FleetMap
                  fleet={visibleTrucks}
                  visibleRouteIds={visibleRouteIds}
                  selectedTruckId={effectiveSelectedId}
                  showRoutes={showRoutes}
                  focusKey={focusKey}
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
                    setRecenterCount((c) => c + 1)
                  }}
                />
              </div>

              <div className="md:absolute md:bottom-4 md:left-4 md:top-4 md:z-[500] md:w-[340px]">
                <RoutePanel
                  role={role}
                  trucks={visibleTrucks}
                  selectedTruckId={effectiveSelectedId}
                  onSelectTruck={(id) => setSelectedTruckId((cur) => (cur === id ? null : id))}
                  statusFilter={statusFilter}
                  onStatusFilterChange={setStatusFilter}
                  fleetFilter={fleetFilter}
                  onFleetFilterChange={setFleetFilter}
                  log={log}
                />
              </div>
            </div>
          </main>
        </div>
      </div>
    </div>
  )
}

function buildStats(role: ViewerRole, trucksList: ReturnType<typeof useFleetSimulation>["fleet"], completedTrips: number): StatPill[] {
  if (role === "driver") {
    const truck = trucksList[0]
    if (!truck) {
      return [
        { label: "Corridor", value: "-", icon: RouteIcon, tone: "signal" },
        { label: "Next stop", value: "-", icon: Clock, tone: "sage" },
        { label: "Cargo", value: "-", icon: Truck, tone: "primary" },
        { label: "Progress", value: "0%", icon: Gauge, tone: "mist" },
      ]
    }
    const route = getRoute(truck.routeId)
    const upcoming = nextStop(route, truck.progress)
    return [
      { label: "Corridor", value: route.number, icon: RouteIcon, tone: "signal" },
      {
        label: "Next stop",
        value: truck.status === "completed" ? "Layover" : `${etaMinutes(route.id, truck.progress, upcoming.at)} min`,
        icon: Clock,
        tone: "sage",
      },
      { label: "Cargo", value: `${(truck.cargoWeight / 1000).toFixed(1)}t`, icon: Truck, tone: "primary" },
      { label: "Progress", value: `${Math.round(truck.progress * 100)}%`, icon: Gauge, tone: "mist" },
    ]
  }

  const active = trucksList.filter((b) => b.status !== "completed").length
  const routeCount = new Set(trucksList.map((b) => b.routeId)).size

  return [
    { label: "Active trucks", value: trucksList.length, icon: Truck, tone: "primary" },
    { label: "In transit", value: active, icon: Gauge, tone: "sage" },
    { label: "Trips logged", value: completedTrips, icon: CircleCheck, tone: "mist" },
    { label: "Corridors", value: routeCount, icon: RouteIcon, tone: "signal" },
  ]
}
