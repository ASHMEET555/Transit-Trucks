"use client"

import { useEffect, useMemo, useRef, useState, Fragment } from "react"
import {
  MapContainer,
  TileLayer,
  Polyline,
  CircleMarker,
  useMap,
} from "react-leaflet"
import L from "leaflet"
import {
  DEMO_ROUTES,
  estimateAnimationDurationMs,
  fetchRoadRoute,
  getDestinationCoords,
  interpolateAlongRoute,
  type LatLng,
} from "@/lib/geo"
import type { Destination, Trip } from "@/lib/tripTypes"
import "leaflet/dist/leaflet.css"

type ActiveTrip = {
  id: number
  label: string
  path: LatLng[]
  color: string
  startedAt: number
  durationMs: number
  status: string
}

const TRIP_COLORS = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)", "var(--signal)"]

function createTruckIcon(color: string) {
  return L.divIcon({
    className: "truck-marker-icon",
    html: `<div class="truck-marker" style="--bearing:0deg;--accent:${color}">
      <svg viewBox="0 0 24 24" width="28" height="28" fill="currentColor">
        <path d="M3 6h11v8H3V6zm11 0h3l3 4v4h-6V6zM6 18a2 2 0 1 0 0-4 2 2 0 0 0 0 4zm9 0a2 2 0 1 0 0-4 2 2 0 0 0 0 4z"/>
      </svg>
    </div>`,
    iconSize: [28, 28],
    iconAnchor: [14, 14],
  })
}

function smoothBearing(current: number, target: number, factor = 0.12): number {
  let diff = target - current
  while (diff > 180) diff -= 360
  while (diff < -180) diff += 360
  return current + diff * factor
}

function updateMarkerVisual(
  marker: L.Marker,
  bearing: number,
  color: string,
) {
  const el = marker.getElement()
  if (!el) return
  const inner = el.querySelector(".truck-marker") as HTMLElement | null
  if (!inner) return
  inner.style.setProperty("--bearing", `${bearing}deg`)
  inner.style.setProperty("--accent", color)
}

function FitBounds({ points }: { points: LatLng[] }) {
  const map = useMap()
  useEffect(() => {
    if (points.length < 2) return
    const bounds = L.latLngBounds(points.map((p) => [p.lat, p.lng]))
    map.fitBounds(bounds, { padding: [48, 48], maxZoom: 8 })
  }, [map, points])
  return null
}

function AnimatedTrucks({ trips }: { trips: ActiveTrip[] }) {
  const map = useMap()
  const markersRef = useRef<Map<number, L.Marker>>(new Map())
  const bearingsRef = useRef<Map<number, number>>(new Map())

  useEffect(() => {
    let frame = 0
    const tick = () => {
      const now = Date.now()
      for (const trip of trips) {
        const elapsed = now - trip.startedAt
        const progress = Math.min(1, elapsed / trip.durationMs)
        const { position, bearing: targetBearing } = interpolateAlongRoute(
          trip.path,
          progress,
        )

        let marker = markersRef.current.get(trip.id)
        if (!marker) {
          marker = L.marker([position.lat, position.lng], {
            icon: createTruckIcon(trip.color),
            zIndexOffset: 1000,
          })
          marker.bindTooltip(trip.label, { direction: "top", offset: [0, -12] })
          marker.addTo(map)
          markersRef.current.set(trip.id, marker)
          bearingsRef.current.set(trip.id, targetBearing)
        }

        marker.setLatLng([position.lat, position.lng])
        const prevBearing = bearingsRef.current.get(trip.id) ?? targetBearing
        const bearing = smoothBearing(prevBearing, targetBearing)
        bearingsRef.current.set(trip.id, bearing)
        updateMarkerVisual(marker, bearing, trip.color)
      }

      for (const [id, marker] of markersRef.current) {
        if (!trips.find((t) => t.id === id)) {
          marker.remove()
          markersRef.current.delete(id)
          bearingsRef.current.delete(id)
        }
      }

      frame = requestAnimationFrame(tick)
    }

    frame = requestAnimationFrame(tick)
    const markers = markersRef.current
    const bearings = bearingsRef.current
    return () => {
      cancelAnimationFrame(frame)
      for (const marker of markers.values()) marker.remove()
      markers.clear()
      bearings.clear()
    }
  }, [map, trips])

  return null
}

export type FleetMapInnerProps = {
  trips?: Trip[]
  destinations?: Destination[]
  demoMode?: boolean
  className?: string
  height?: string
}

export default function FleetMapInner({
  trips = [],
  destinations = [],
  demoMode = false,
  className = "",
  height = "100%",
}: FleetMapInnerProps) {
  const [activeTrips, setActiveTrips] = useState<ActiveTrip[]>([])
  const tripStartTimesRef = useRef<Map<number, number>>(new Map())
  const destMap = useMemo(() => {
    const m = new Map<string, string>()
    for (const d of destinations) m.set(d.name, d.region)
    return m
  }, [destinations])

  useEffect(() => {
    let cancelled = false

    async function loadRoutes() {
      if (demoMode) {
        const demoActive: ActiveTrip[] = []
        for (const route of DEMO_ROUTES) {
          const path = await fetchRoadRoute(route.from, route.to)
          if (cancelled) return
          demoActive.push({
            id: route.id,
            label: route.label,
            path,
            color: route.color,
            startedAt: Date.now(),
            durationMs: estimateAnimationDurationMs(path),
            status: "Dispatched",
          })
        }
        setActiveTrips(demoActive)
        return
      }

      const dispatched = trips.filter((t) => t.status === "Dispatched")
      const loaded: ActiveTrip[] = []

      for (let i = 0; i < dispatched.length; i += 1) {
        const trip = dispatched[i]
        const from = getDestinationCoords(
          trip.source_name,
          destMap.get(trip.source_name),
        )
        const to = getDestinationCoords(
          trip.destination_name,
          destMap.get(trip.destination_name),
        )
        const path = await fetchRoadRoute(from, to)
        if (cancelled) return
        let startedAt = tripStartTimesRef.current.get(trip.id)
        if (!startedAt) {
          startedAt = Date.now()
          tripStartTimesRef.current.set(trip.id, startedAt)
        }
        loaded.push({
          id: trip.id,
          label: `${trip.vehicle_reg} → ${trip.destination_name}`,
          path,
          color: TRIP_COLORS[i % TRIP_COLORS.length],
          startedAt,
          durationMs: estimateAnimationDurationMs(path, trip.planned_distance),
          status: trip.status,
        })
      }

      const activeIds = new Set(dispatched.map((t) => t.id))
      for (const id of tripStartTimesRef.current.keys()) {
        if (!activeIds.has(id)) tripStartTimesRef.current.delete(id)
      }
      setActiveTrips(loaded)
    }

    loadRoutes()
    return () => {
      cancelled = true
    }
  }, [trips, destinations, demoMode, destMap])

  const hubPoints = useMemo(() => {
    if (demoMode) {
      return DEMO_ROUTES.flatMap((r) => [r.from, r.to])
    }
    const names = new Set<string>()
    for (const t of trips) {
      names.add(t.source_name)
      names.add(t.destination_name)
    }
    return Array.from(names).map((name) =>
      getDestinationCoords(name, destMap.get(name)),
    )
  }, [trips, demoMode, destMap])

  const allPoints = useMemo(
    () => [...hubPoints, ...activeTrips.flatMap((t) => t.path)],
    [hubPoints, activeTrips],
  )

  return (
    <div className={`fleet-map-shell ${className}`} style={{ height }}>
      <MapContainer
        center={[39.5, -98.35]}
        zoom={4}
        className="fleet-map-canvas"
        zoomControl={false}
        attributionControl={false}
      >
        <TileLayer
          url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>'
        />
        <FitBounds points={allPoints} />
        {activeTrips.map((trip) => (
          <Polyline
            key={`route-${trip.id}`}
            positions={trip.path.map((p) => [p.lat, p.lng] as [number, number])}
            pathOptions={{
              color: trip.color,
              weight: 5,
              opacity: 0.85,
              lineCap: "round",
              lineJoin: "round",
            }}
          />
        ))}
        {activeTrips.map((trip) => {
          const start = trip.path[0]
          const end = trip.path[trip.path.length - 1]
          return (
            <Fragment key={`hubs-${trip.id}`}>
              <CircleMarker
                center={[start.lat, start.lng]}
                radius={6}
                pathOptions={{
                  color: "var(--card)",
                  fillColor: trip.color,
                  fillOpacity: 1,
                  weight: 2,
                }}
              />
              <CircleMarker
                center={[end.lat, end.lng]}
                radius={6}
                pathOptions={{
                  color: "var(--card)",
                  fillColor: "var(--signal)",
                  fillOpacity: 1,
                  weight: 2,
                }}
              />
            </Fragment>
          )
        })}
        <AnimatedTrucks trips={activeTrips} />
      </MapContainer>
      <div className="fleet-map-overlay">
        <div className="fleet-map-pill">
          <span className="fleet-map-live-dot" />
          Live fleet tracking
        </div>
        <div className="fleet-map-pill muted">
          {activeTrips.length} active route{activeTrips.length === 1 ? "" : "s"}
        </div>
      </div>
    </div>
  )
}
