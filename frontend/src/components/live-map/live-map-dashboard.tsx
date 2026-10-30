"use client"

import dynamic from "next/dynamic"
import { useCallback, useEffect, useMemo, useState } from "react"
import type { Map as LeafletMap } from "leaflet"
import { CircleCheck, Clock, MapPinned, Truck } from "lucide-react"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import type { Destination, Trip } from "@/lib/tripTypes"
import { isActiveTrip } from "@/components/map/FleetMapInner"
import { LiveBadge, MapControls } from "./map-controls"
import { RoleFilter, type ViewerRole } from "./role-filter"
import { RoutePanel, type StatusFilter } from "./route-panel"
import { StatPills, type StatPill } from "./stat-pills"
import { TopBar } from "./top-bar"

const FleetMap = dynamic(() => import("./fleet-map"), {
  ssr: false,
  loading: () => (
    <div className="grid size-full place-items-center bg-muted/40" role="status">
      <p className="text-sm text-muted-foreground">Loading live freight map…</p>
    </div>
  ),
})

export function LiveMapDashboard({ userRole = "Fleet Manager" }: { userRole?: string }) {
  const { currentProfile } = useProfile()
  const initialViewer: ViewerRole = userRole === "Driver" ? "driver" : "operator"
  const [role, setRole] = useState<ViewerRole>(initialViewer)
  const [paused, setPaused] = useState(false)
  const [query, setQuery] = useState("")
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all")
  const [selectedTripId, setSelectedTripId] = useState<number | null>(null)
  const [showRoutes, setShowRoutes] = useState(true)
  const [, setRecenterCount] = useState(0)
  const [map, setMap] = useState<LeafletMap | null>(null)
  const [trips, setTrips] = useState<Trip[]>([])
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setError(null)
      const [t, d] = await Promise.all([
        apiRequest("/trips/"),
        apiRequest("/destinations/"),
      ])
      setTrips(extractItems<Trip>(t))
      setDestinations(extractItems<Destination>(d))
      setLastSyncAt(new Date().toISOString())
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load live trips")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const { status: wsStatus } = useFleetEvents({
    enabled: !paused,
    onEvent: () => {
      load()
    },
    onReconnect: load,
  })

  function changeRole(nextRole: ViewerRole) {
    setRole(nextRole)
    setSelectedTripId(null)
    setStatusFilter("all")
  }

  const scopedTrips = useMemo(() => {
    if (role === "driver" && currentProfile?.driver_license) {
      return trips.filter((t) => t.driver_license === currentProfile.driver_license)
    }
    return trips
  }, [trips, role, currentProfile?.driver_license])

  const q = query.trim().toLowerCase()
  const visibleTrips = useMemo(
    () =>
      scopedTrips.filter((trip) => {
        if (statusFilter === "active" && !isActiveTrip(trip.status)) return false
        if (statusFilter === "completed" && trip.status !== "Completed") return false
        if (!q) return true
        return (
          (trip.vehicle_reg || "").toLowerCase().includes(q) ||
          trip.source_name.toLowerCase().includes(q) ||
          trip.destination_name.toLowerCase().includes(q) ||
          trip.driver_license.toLowerCase().includes(q) ||
          String(trip.id).includes(q)
        )
      }),
    [scopedTrips, statusFilter, q],
  )

  const effectiveSelectedId =
    selectedTripId !== null && visibleTrips.some((t) => t.id === selectedTripId)
      ? selectedTripId
      : role === "driver"
        ? (visibleTrips[0]?.id ?? null)
        : null

  const activeCount = scopedTrips.filter((t) => isActiveTrip(t.status)).length
  const completedCount = scopedTrips.filter((t) => t.status === "Completed").length
  const stats: StatPill[] = [
    { label: "Active trips", value: activeCount, icon: Truck, tone: "primary" },
    { label: "Completed trips", value: completedCount, icon: CircleCheck, tone: "sage" },
    { label: "Places tracked", value: destinations.length, icon: MapPinned, tone: "signal" },
    {
      label: "Last sync",
      value: lastSyncAt ? new Date(lastSyncAt).toLocaleTimeString() : "—",
      icon: Clock,
      tone: "mist",
    },
  ]

  const focusKey = `${role}-${effectiveSelectedId ?? "all"}-${visibleTrips.map((t) => t.id).join(",")}`

  return (
    <div className="flex flex-col gap-4">
      <nav aria-label="Breadcrumb" className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <ol className="flex items-center gap-1.5">
          <li>Overview</li>
          <li aria-hidden="true">&gt;</li>
          <li className="font-medium text-foreground" aria-current="page">
            Live map
          </li>
        </ol>
        <RealtimeIndicator status={wsStatus} />
      </nav>

      <p className="rounded-xl border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
        Dispatch status from the operations API. Lines join trip origin to destination and
        markers show estimated positions — no GPS source is connected.
        {loading
          ? " Loading trips…"
          : ` Tracking ${trips.length} ${trips.length === 1 ? "trip" : "trips"}.`}
        {lastSyncAt && !loading
          ? ` Last sync ${new Date(lastSyncAt).toLocaleTimeString()}.`
          : ""}
        {error ? ` Last refresh issue: ${error}.` : ""}
      </p>

      {error && !loading && trips.length === 0 ? (
        <div className="rounded-2xl border border-destructive/30 bg-card p-8 text-center" role="alert">
          <p className="font-medium text-foreground">Could not load live trips</p>
          <p className="mt-1 text-sm text-muted-foreground">{error}</p>
          <button
            type="button"
            onClick={load}
            className="mt-4 rounded-md border border-border bg-muted px-3 py-2 text-sm font-medium text-foreground hover:bg-muted/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            Retry
          </button>
        </div>
      ) : (
        <>
          <TopBar query={query} onQueryChange={setQuery} />

          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <StatPills items={stats} />
            <RoleFilter value={role} onChange={changeRole} />
          </div>

          {loading ? (
            <div className="grid min-h-[420px] place-items-center rounded-3xl border border-border bg-card" role="status">
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <span className="size-2 animate-pulse rounded-full bg-signal motion-reduce:animate-none" aria-hidden="true" />
                Loading trips and places…
              </p>
            </div>
          ) : visibleTrips.length === 0 && scopedTrips.length === 0 ? (
            <div className="rounded-3xl border border-border bg-card p-8 text-center">
              <p className="font-medium text-foreground">No trips to show</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {role === "driver"
                  ? "No trips are assigned to your driver license right now."
                  : "Dispatch a trip to see it on the live map."}
              </p>
            </div>
          ) : (
            <div className="relative flex flex-1 flex-col gap-4 md:block md:min-h-[560px]">
              <div className="relative h-[60vh] overflow-hidden rounded-3xl border border-border md:absolute md:inset-0 md:h-auto">
                <FleetMap
                  trips={visibleTrips}
                  destinations={destinations}
                  selectedTripId={effectiveSelectedId}
                  showRoutes={showRoutes}
                  focusKey={focusKey}
                  onSelectTrip={setSelectedTripId}
                  onReady={setMap}
                />
                <LiveBadge paused={paused} onToggle={() => setPaused((p) => !p)} />
                <MapControls
                  showRoutes={showRoutes}
                  onShowRoutesChange={setShowRoutes}
                  onZoomIn={() => map?.zoomIn()}
                  onZoomOut={() => map?.zoomOut()}
                  onRecenter={() => {
                    setSelectedTripId(null)
                    setRecenterCount((c) => c + 1)
                  }}
                />
              </div>

              <div className="md:absolute md:bottom-4 md:left-4 md:top-4 md:z-[500] md:w-[340px]">
                <RoutePanel
                  role={role}
                  trips={visibleTrips}
                  selectedTripId={effectiveSelectedId}
                  onSelectTrip={setSelectedTripId}
                  statusFilter={statusFilter}
                  onStatusFilterChange={setStatusFilter}
                />
              </div>
            </div>
          )}

          {error && trips.length > 0 && (
            <p role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm">
              Showing last loaded data. Refresh issue: {error}{" "}
              <button type="button" onClick={load} className="font-medium underline underline-offset-2">
                Retry
              </button>
            </p>
          )}
        </>
      )}
    </div>
  )
}
