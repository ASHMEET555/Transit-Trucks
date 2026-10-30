"use client"

import { useEffect, useMemo } from "react"
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
import { getDestinationCoords, type LatLng } from "@/lib/geo"
import type { Destination, Trip } from "@/lib/tripTypes"
import "leaflet/dist/leaflet.css"

export const ESTIMATED_POSITION_LABEL = "Estimated position, no GPS"

export const ACTIVE_STATUSES = ["Dispatched"]

export function isActiveTrip(status: string): boolean {
  return status !== "Completed" && status !== "Cancelled"
}

function midpoint(a: LatLng, b: LatLng): LatLng {
  return { lat: (a.lat + b.lat) / 2, lng: (a.lng + b.lng) / 2 }
}

function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return true
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches
}

function estimatedIcon(label: string, selected: boolean): L.DivIcon {
  return L.divIcon({
    className: "",
    html: `<div class="bus-marker${selected ? " bus-marker--selected" : ""}"><span class="bus-marker__label">${label}</span></div>`,
    iconSize: [34, 34],
    iconAnchor: [17, 17],
  })
}

export type FleetMapInnerProps = {
  trips: Trip[]
  destinations: Destination[]
  selectedTripId: number | null
  showRoutes: boolean
  focusKey: string
  onSelectTrip: (id: number | null) => void
  onReady: (map: LeafletMap) => void
}

type PlottedTrip = {
  trip: Trip
  from: LatLng
  to: LatLng
  at: LatLng
}

function FitController({ points, focusKey }: { points: LatLng[]; focusKey: string }) {
  const map = useMap()
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") return
    const container = map.getContainer()
    const observer = new ResizeObserver(() => {
      map.invalidateSize()
    })
    observer.observe(container)
    return () => observer.disconnect()
  }, [map])
  useEffect(() => {
    map.invalidateSize()
    if (points.length === 0) return
    const bounds = L.latLngBounds(points.map((p) => [p.lat, p.lng] as [number, number]))
    if (prefersReducedMotion()) {
      map.fitBounds(bounds, { padding: [48, 48], animate: false, maxZoom: 8 })
    } else {
      map.flyToBounds(bounds, { padding: [48, 48], duration: 0.8, maxZoom: 8 })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusKey])
  return null
}

export function FleetMapInner({
  trips,
  destinations,
  selectedTripId,
  showRoutes,
  focusKey,
  onSelectTrip,
  onReady,
}: FleetMapInnerProps) {
  const regionByName = useMemo(() => {
    const m = new Map<string, string>()
    for (const d of destinations) m.set(d.name, d.region)
    return m
  }, [destinations])

  const plotted = useMemo<PlottedTrip[]>(
    () =>
      trips.filter((t) => isActiveTrip(t.status)).map((trip) => {
        const from = getDestinationCoords(trip.source_name, regionByName.get(trip.source_name))
        const to = getDestinationCoords(
          trip.destination_name,
          regionByName.get(trip.destination_name),
        )
        return { trip, from, to, at: midpoint(from, to) }
      }),
    [trips, regionByName],
  )

  const hubPoints = useMemo<LatLng[]>(() => {
    const seen = new Set<string>()
    const pts: LatLng[] = []
    for (const t of trips) {
      for (const name of [t.source_name, t.destination_name]) {
        if (seen.has(name)) continue
        seen.add(name)
        pts.push(getDestinationCoords(name, regionByName.get(name)))
      }
    }
    return pts
  }, [trips, regionByName])

  const fitPoints = useMemo(
    () => [...hubPoints, ...plotted.flatMap((p) => [p.from, p.to, p.at])],
    [hubPoints, plotted],
  )

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

        {showRoutes &&
          plotted.map(({ trip, from, to }) => {
            const selected = trip.id === selectedTripId
            return (
              <Polyline
                key={`route-${trip.id}`}
                positions={[
                  [from.lat, from.lng],
                  [to.lat, to.lng],
                ]}
                pathOptions={{
                  color: selected ? "var(--primary)" : "var(--chart-5)",
                  weight: selected ? 5 : 3,
                  opacity: selected ? 1 : 0.65,
                  lineCap: "round",
                }}
                interactive={false}
              />
            )
          })}

        {plotted.map(({ trip, from, to }) => (
          <CircleMarker
            key={`origin-${trip.id}`}
            center={[from.lat, from.lng]}
            radius={6}
            pathOptions={{
              color: "var(--card)",
              fillColor: "var(--chart-5)",
              fillOpacity: 1,
              weight: 2,
            }}
          >
            <Tooltip direction="top" offset={[0, -4]}>
              Origin: {trip.source_name}
            </Tooltip>
          </CircleMarker>
        ))}

        {plotted.map(({ trip, from, to }) => (
          <CircleMarker
            key={`destination-${trip.id}`}
            center={[to.lat, to.lng]}
            radius={6}
            pathOptions={{
              color: "var(--card)",
              fillColor: "var(--signal)",
              fillOpacity: 1,
              weight: 2,
            }}
          >
            <Tooltip direction="top" offset={[0, -4]}>
              Destination: {trip.destination_name}
            </Tooltip>
          </CircleMarker>
        ))}

        {plotted.map(({ trip, at }) => (
          <Marker
            key={`estimated-${trip.id}`}
            position={[at.lat, at.lng]}
            icon={estimatedIcon(trip.vehicle_reg || `Trip ${trip.id}`, trip.id === selectedTripId)}
            zIndexOffset={trip.id === selectedTripId ? 1000 : 0}
            eventHandlers={{
              click: () =>
                onSelectTrip(trip.id === selectedTripId ? null : trip.id),
            }}
            title={`${trip.vehicle_reg || `Trip ${trip.id}`}. ${ESTIMATED_POSITION_LABEL}.`}
            keyboard
          >
            <Tooltip direction="top" offset={[0, -12]}>
              {trip.vehicle_reg || `Trip ${trip.id}`}. {ESTIMATED_POSITION_LABEL}.
            </Tooltip>
          </Marker>
        ))}
      </MapContainer>

      <ul className="sr-only" aria-label="Map markers as a list">
        {plotted.map(({ trip }) => (
          <li key={`marker-text-${trip.id}`}>
            Trip {trip.id} ({trip.vehicle_reg || "no vehicle"}): origin {trip.source_name},
            destination {trip.destination_name}. {ESTIMATED_POSITION_LABEL}.
          </li>
        ))}
      </ul>
    </>
  )
}
