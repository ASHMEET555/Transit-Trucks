"use client"

import { useEffect, useState, useCallback, useMemo } from "react"
import dynamic from "next/dynamic"
import {
  Truck,
  MapPin,
  Clock,
  AlertTriangle,
  Info,
  Compass,
  RefreshCw,
  Search,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import { useProfile } from "@/lib/ProfileContext"
import { getLiveTrips, type LiveTripsResponse, type LiveTrip } from "@/lib/api/client"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { TableSkeleton, ErrorState } from "@/components/ui/data-states"

const LeafletMap = dynamic(
  () =>
    import("react-leaflet").then((mod) => {
      const { MapContainer, TileLayer, Marker, Popup, Polyline } = mod
      return function MapView({
        trips,
        selectedTrip,
        onSelect,
      }: {
        trips: LiveTrip[]
        selectedTrip: LiveTrip | null
        onSelect: (trip: LiveTrip) => void
      }) {
        const L = require("leaflet")

        const truckIcon = new L.DivIcon({
          className: "custom-truck-marker",
          html: `<div style="background-color: oklch(0.3 0.05 160); color: white; border-radius: 9999px; width: 32px; height: 32px; display: flex; align-items: center; justify-content: center; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.2); border: 2px solid white; cursor: pointer;">
            <svg style="width: 16px; height: 16px;" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M14 18V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v11a1 1 0 0 0 1 1h2"/><path d="M15 18H9"/><path d="M19 18h2a1 1 0 0 0 1-1v-3.65a1 1 0 0 0-.22-.62l-3.48-4.35A1 1 0 0 0 17.52 8H14"/><circle cx="7" cy="18" r="2"/><circle cx="17" cy="18" r="2"/></svg>
          </div>`,
          iconSize: [32, 32],
          iconAnchor: [16, 16],
        })

        const depotIcon = new L.DivIcon({
          className: "custom-depot-marker",
          html: `<div style="background-color: oklch(0.68 0.17 48); color: white; border-radius: 9999px; width: 22px; height: 22px; display: flex; align-items: center; justify-content: center; box-shadow: 0 2px 4px rgba(0,0,0,0.2); border: 2px solid white;">
            <div style="width: 6px; height: 6px; background-color: white; border-radius: 9999px;"></div>
          </div>`,
          iconSize: [22, 22],
          iconAnchor: [11, 11],
        })

        return (
          <MapContainer
            center={[38.5, -96.0]}
            zoom={4}
            style={{ width: "100%", height: "100%", borderRadius: "0.75rem" }}
            scrollWheelZoom={true}
          >
            <TileLayer
              attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            />
            {trips.map((t) => {
              const oLat = t.origin?.latitude ?? 0
              const oLng = t.origin?.longitude ?? 0
              const dLat = t.destination?.latitude ?? 0
              const dLng = t.destination?.longitude ?? 0

              // Client-side linear interpolation for estimated position
              const progress = Math.min(Math.max(t.estimated_progress ?? 0, 0), 1)
              const curLat = oLat + (dLat - oLat) * progress
              const curLng = oLng + (dLng - oLng) * progress

              return (
                <div key={t.id}>
                  {/* Route corridor */}
                  <Polyline
                    positions={[
                      [oLat, oLng],
                      [dLat, dLng],
                    ]}
                    color="oklch(0.3 0.05 160)"
                    weight={3}
                    opacity={selectedTrip?.id === t.id ? 0.9 : 0.4}
                    dashArray="6, 8"
                  />
                  {/* Origin */}
                  <Marker position={[oLat, oLng]} icon={depotIcon}>
                    <Popup>
                      <div className="text-xs p-1">
                        <strong>Origin:</strong> {t.source_name}
                      </div>
                    </Popup>
                  </Marker>
                  {/* Destination */}
                  <Marker position={[dLat, dLng]} icon={depotIcon}>
                    <Popup>
                      <div className="text-xs p-1">
                        <strong>Destination:</strong> {t.destination_name}
                      </div>
                    </Popup>
                  </Marker>
                  {/* Vehicle marker */}
                  <Marker
                    position={[curLat, curLng]}
                    icon={truckIcon}
                    eventHandlers={{ click: () => onSelect(t) }}
                  >
                    <Popup>
                      <div className="text-xs space-y-1 p-1">
                        <div className="font-bold text-sm">Trip #{t.id}</div>
                        <div>Vehicle: {t.vehicle_reg}</div>
                        {t.driver_license && <div>Driver: {t.driver_license}</div>}
                        <div>Route: {t.source_name} → {t.destination_name}</div>
                        <div>
                          Est. Progress: {((t.estimated_progress ?? 0) * 100).toFixed(1)}%
                        </div>
                        <div className="text-[10px] text-amber-600 font-medium pt-1">
                          Estimated position, no GPS
                        </div>
                      </div>
                    </Popup>
                  </Marker>
                </div>
              )
            })}
          </MapContainer>
        )
      }
    }),
  { ssr: false }
)

export default function LiveMapPage() {
  const { currentProfile } = useProfile()
  const role = currentProfile?.role || "Fleet Manager"

  const [data, setData] = useState<LiveTripsResponse | null>(null)
  const [selectedTrip, setSelectedTrip] = useState<LiveTrip | null>(null)
  const [search, setSearch] = useState("")
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadLiveTrips = useCallback(async () => {
    try {
      setError(null)
      const res = await getLiveTrips()
      setData(res)
      const items = res.items || []
      if (items.length > 0 && !selectedTrip) {
        setSelectedTrip(items[0])
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load live dispatched trips.")
    } finally {
      setLoading(false)
    }
  }, [selectedTrip])

  useEffect(() => {
    loadLiveTrips()
  }, [loadLiveTrips])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: (event) => {
      if (event.type.startsWith("trip.")) {
        loadLiveTrips()
      }
    },
  })

  const trips = data?.items || []

  const filteredTrips = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return trips
    return trips.filter(
      (t) =>
        (t.vehicle_reg || "").toLowerCase().includes(q) ||
        (t.source_name || "").toLowerCase().includes(q) ||
        (t.destination_name || "").toLowerCase().includes(q) ||
        (t.driver_license && t.driver_license.toLowerCase().includes(q))
    )
  }, [trips, search])

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto flex flex-col h-[calc(100vh-4rem)]">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border/60 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">Live Telematics Map</h1>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1.5">
            <Info className="size-3.5 text-signal" />
            <span>
              {data?.estimated_warning || "Estimated position, no GPS"} • Assumed transit speed: {data?.assumed_speed_kmh || 80} km/h
            </span>
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={loadLiveTrips} className="gap-2 text-xs">
            <RefreshCw className="size-3.5" /> Refresh
          </Button>
        </div>
      </div>

      {error && <ErrorState message={error} onRetry={loadLiveTrips} />}

      {/* Main Split Grid: Map + Trip Ledger */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-0">
        {/* Left: Map Container */}
        <div className="lg:col-span-2 h-[420px] lg:h-full rounded-xl border border-border/80 bg-card overflow-hidden shadow-xs relative">
          {loading ? (
            <div className="p-6">
              <TableSkeleton rows={6} cols={3} />
            </div>
          ) : (
            <LeafletMap
              trips={filteredTrips}
              selectedTrip={selectedTrip}
              onSelect={(t) => setSelectedTrip(t)}
            />
          )}

          {/* Map floating disclosure badge */}
          <div className="absolute bottom-3 left-3 z-[1000] bg-white/90 dark:bg-neutral-900/90 backdrop-blur-xs px-3 py-1.5 rounded-lg border border-border/80 text-[11px] font-medium text-muted-foreground shadow-sm flex items-center gap-2">
            <span className="size-2 rounded-full bg-signal" />
            <span>Estimated coordinates computed server-side via time-elapsed formula</span>
          </div>
        </div>

        {/* Right: Active Dispatched Trips List (A11y & Quick Jump) */}
        <div className="rounded-xl border border-border/80 bg-card p-5 shadow-xs flex flex-col justify-between overflow-hidden">
          <div className="flex flex-col h-full min-h-0">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
                <Truck className="size-4 text-primary" /> Active En Route Trips
              </h2>
              <span className="text-[11px] font-mono px-2 py-0.5 rounded-full bg-neutral-100 text-neutral-700">
                {filteredTrips.length} active
              </span>
            </div>

            {/* Filter Search */}
            <div className="relative mb-3">
              <Search className="size-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search vehicle, driver or hub..."
                className="w-full text-xs pl-8 pr-3 py-1.5 rounded-lg border border-border bg-background focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>

            {/* Scrollable Trips */}
            <div className="flex-1 overflow-y-auto space-y-2 pr-1" tabIndex={0} aria-label="Active Trips List">
              {filteredTrips.length === 0 ? (
                <div className="py-12 text-center text-xs text-muted-foreground">
                  No active dispatched trips matching coordinates criteria.
                </div>
              ) : (
                filteredTrips.map((t) => {
                  const isSel = selectedTrip?.id === t.id
                  const progressPct = ((t.estimated_progress ?? 0) * 100).toFixed(0)

                  return (
                    <div
                      key={t.id}
                      onClick={() => setSelectedTrip(t)}
                      className={`p-3 rounded-lg border text-left cursor-pointer transition-all ${
                        isSel
                          ? "border-primary bg-primary/5 shadow-xs"
                          : "border-border/60 hover:border-border hover:bg-neutral-50"
                      }`}
                    >
                      <div className="flex items-center justify-between mb-1.5">
                        <span className="text-xs font-bold text-foreground">Trip #{t.id}</span>
                        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-signal/10 text-signal font-semibold">
                          {progressPct}% Progress
                        </span>
                      </div>

                      <div className="text-xs text-foreground font-medium mb-1">
                        {t.source_name} → {t.destination_name}
                      </div>

                      <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                        <span>Vehicle: <strong className="text-foreground">{t.vehicle_reg}</strong></span>
                        {t.driver_license && (
                          <span>License: <strong className="text-foreground">{t.driver_license}</strong></span>
                        )}
                      </div>

                      {/* Progress bar */}
                      <div className="w-full bg-neutral-200/60 rounded-full h-1.5 mt-2.5 overflow-hidden">
                        <div
                          style={{ width: `${progressPct}%` }}
                          className="bg-primary h-full rounded-full transition-all duration-300"
                        />
                      </div>
                    </div>
                  )
                })
              )}
            </div>
          </div>

          <div className="pt-3 border-t border-border/60 mt-3 text-[11px] text-muted-foreground text-center">
            Role scope: <span className="font-semibold text-foreground">{role}</span> (server-enforced)
          </div>
        </div>
      </div>
    </div>
  )
}
