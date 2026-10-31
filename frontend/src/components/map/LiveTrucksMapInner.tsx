"use client"

import { useEffect, useMemo, useState } from "react"
import L, { type Map as LeafletMap } from "leaflet"
import {
  CircleMarker,
  MapContainer,
  Marker,
  Polyline,
  TileLayer,
  Tooltip,
  useMap,
} from "react-leaflet"
import "leaflet/dist/leaflet.css"
import { clusterMarkers, decodePolyline6 } from "@/lib/live/liveUtils.mjs"
import type { LiveVehicle } from "@/lib/live/liveTypes"

const TRUCK_SVG =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14 18V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v11a1 1 0 0 0 1 1h2"/><path d="M15 18H9"/><path d="M19 18h2a1 1 0 0 0 1-1v-3.65a1 1 0 0 0-.22-.62l-3.48-4.35A1 1 0 0 0 17.52 8H14"/><circle cx="17" cy="18" r="2"/><circle cx="7" cy="18" r="2"/></svg>'

function truckIcon(vehicle: LiveVehicle, selected: boolean): L.DivIcon {
  const heading = vehicle.heading_deg ?? 0
  return L.divIcon({
    className: "",
    html:
      `<div class="truck-marker truck-marker--${vehicle.status}${selected ? " truck-marker--selected" : ""}" role="img" aria-label="Truck ${vehicle.vehicle_reg}">` +
      `<span class="truck-marker__glyph" style="transform: rotate(${heading}deg)">${TRUCK_SVG}</span>` +
      (selected ? `<span class="truck-marker__tag">${vehicle.vehicle_reg}</span>` : "") +
      `</div>`,
    iconSize: selected ? [44, 44] : [36, 36],
    iconAnchor: selected ? [22, 22] : [18, 18],
  })
}

function clusterIcon(count: number): L.DivIcon {
  return L.divIcon({
    className: "",
    html: `<div class="truck-cluster" role="img" aria-label="${count} trucks grouped">${count}</div>`,
    iconSize: [36, 36],
    iconAnchor: [18, 18],
  })
}

function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return true
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches
}

type PlottedRoute = {
  reg: string
  points: Array<[number, number]>
  provider: string | null
  selected: boolean
}

function FitController({ points, focusKey }: { points: Array<[number, number]>; focusKey: string }) {
  const map = useMap()
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") return
    const observer = new ResizeObserver(() => map.invalidateSize())
    observer.observe(map.getContainer())
    return () => observer.disconnect()
  }, [map])
  useEffect(() => {
    map.invalidateSize()
    if (points.length === 0) return
    const bounds = L.latLngBounds(points)
    if (prefersReducedMotion()) {
      map.fitBounds(bounds, { padding: [48, 48], animate: false, maxZoom: 8 })
    } else {
      map.flyToBounds(bounds, { padding: [48, 48], duration: 0.8, maxZoom: 8 })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusKey])
  return null
}

function ClusteredMarkers({
  vehicles,
  selectedReg,
  onSelect,
}: {
  vehicles: LiveVehicle[]
  selectedReg: string | null
  onSelect: (reg: string | null) => void
}) {
  const map = useMap()
  const [, setZoomTick] = useState(0)
  useEffect(() => {
    const bump = () => setZoomTick((t) => t + 1)
    map.on("zoomend moveend", bump)
    return () => {
      map.off("zoomend moveend", bump)
    }
  }, [map])

  const clusters = useMemo(() => {
    const positioned = vehicles.filter((v) => v.latitude != null && v.longitude != null)
    const project = (lat: number, lon: number) => {
      const p = map.latLngToContainerPoint([lat, lon])
      return { x: p.x, y: p.y }
    }
    return clusterMarkers(
      positioned.map((v) => ({ vehicle_reg: v.vehicle_reg, latitude: v.latitude as number, longitude: v.longitude as number })),
      project,
      40,
    )
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [map, vehicles, selectedReg])

  const byReg = useMemo(() => new Map(vehicles.map((v) => [v.vehicle_reg, v])), [vehicles])

  return (
    <>
      {clusters.map((c) =>
        c.members.length === 1 ? (
          <Marker
            key={c.key}
            position={[c.lat, c.lon]}
            icon={truckIcon(byReg.get(c.key) as LiveVehicle, c.key === selectedReg)}
            zIndexOffset={c.key === selectedReg ? 1000 : 0}
            eventHandlers={{ click: () => onSelect(c.key === selectedReg ? null : c.key) }}
            title={(byReg.get(c.key) as LiveVehicle).vehicle_reg}
            keyboard
          >
            <Tooltip direction="top" offset={[0, -12]}>
              {(byReg.get(c.key) as LiveVehicle).vehicle_reg}
            </Tooltip>
          </Marker>
        ) : (
          <Marker key={`cluster-${c.key}`} position={[c.lat, c.lon]} icon={clusterIcon(c.members.length)} keyboard={false}>
            <Tooltip direction="top" offset={[0, -12]}>
              {c.members.map((m) => m.vehicle_reg).join(", ")}
            </Tooltip>
          </Marker>
        ),
      )}
    </>
  )
}

export type LiveTrucksMapProps = {
  vehicles: LiveVehicle[]
  selectedReg: string | null
  focusKey: string
  onSelect: (reg: string | null) => void
  onReady: (map: LeafletMap) => void
}

export function LiveTrucksMapInner({ vehicles, selectedReg, focusKey, onSelect, onReady }: LiveTrucksMapProps) {
  const routes = useMemo<PlottedRoute[]>(
    () =>
      vehicles.flatMap((v) => {
        if (!v.route_polyline) return []
        try {
          const decoded = decodePolyline6(v.route_polyline)
          if (decoded.length < 2) return []
          return [{ reg: v.vehicle_reg, points: decoded, provider: v.route_provider, selected: v.vehicle_reg === selectedReg }]
        } catch {
          return []
        }
      }),
    [vehicles, selectedReg],
  )

  const selected = vehicles.find((v) => v.vehicle_reg === selectedReg) ?? null

  const fitPoints = useMemo<Array<[number, number]>>(() => {
    if (selected?.route_polyline) {
      try {
        const pts = decodePolyline6(selected.route_polyline)
        if (pts.length > 1) return pts
      } catch {
        /* fall through */
      }
    }
    const pts: Array<[number, number]> = []
    for (const r of routes) pts.push(...r.points)
    for (const v of vehicles) {
      if (v.latitude != null && v.longitude != null) pts.push([v.latitude, v.longitude])
      if (v.origin.latitude != null && v.origin.longitude != null) pts.push([v.origin.latitude, v.origin.longitude])
      if (v.destination.latitude != null && v.destination.longitude != null) pts.push([v.destination.latitude, v.destination.longitude])
    }
    return pts
  }, [routes, selected, vehicles])

  const hubDots = useMemo(() => {
    if (!selected) return []
    const dots: Array<{ key: string; lat: number; lon: number; label: string; tone: "origin" | "destination" }> = []
    if (selected.origin.latitude != null && selected.origin.longitude != null) {
      dots.push({ key: "origin", lat: selected.origin.latitude, lon: selected.origin.longitude, label: `Origin: ${selected.origin.name}`, tone: "origin" })
    }
    if (selected.destination.latitude != null && selected.destination.longitude != null) {
      dots.push({ key: "destination", lat: selected.destination.latitude, lon: selected.destination.longitude, label: `Destination: ${selected.destination.name}`, tone: "destination" })
    }
    return dots
  }, [selected])

  return (
    <>
      <MapContainer
        center={[39.5, -98.35]}
        zoom={4}
        zoomControl={false}
        scrollWheelZoom
        ref={(map) => {
          if (map) onReady(map)
        }}
        className="size-full"
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://tile.openstreetmap.org/{z}/{x}/{y}.png"
          className="map-tiles"
        />
        <FitController points={fitPoints} focusKey={focusKey} />

        {routes.map((r) => (
          <Polyline
            key={`route-${r.reg}`}
            positions={r.points}
            pathOptions={{
              className: `route-line${r.selected ? " route-line--active" : " route-line--dim"}`,
              weight: r.selected ? 5 : 3,
              lineCap: "round",
              dashArray: r.provider === "approximate" ? "6 6" : undefined,
            }}
            interactive={false}
          />
        ))}

        {hubDots.map((h) => (
          <CircleMarker
            key={h.key}
            center={[h.lat, h.lon]}
            radius={6}
            pathOptions={{
              color: "var(--card)",
              fillColor: h.tone === "origin" ? "var(--chart-5)" : "var(--signal)",
              fillOpacity: 1,
              weight: 2,
            }}
          >
            <Tooltip direction="top" offset={[0, -4]}>
              {h.label}
            </Tooltip>
          </CircleMarker>
        ))}

        <ClusteredMarkers vehicles={vehicles} selectedReg={selectedReg} onSelect={onSelect} />
      </MapContainer>

      <ul className="sr-only" aria-label="Map markers as a list">
        {vehicles.map((v) => (
          <li key={`marker-text-${v.vehicle_reg}`}>
            Truck {v.vehicle_reg} on trip {v.trip_id}: {v.origin.name} to {v.destination.name}.{" "}
            {v.latitude != null ? `Position ${v.latitude}, ${v.longitude}.` : "No position yet."}
          </li>
        ))}
      </ul>
    </>
  )
}
