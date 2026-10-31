"use client"

import "leaflet/dist/leaflet.css"
import L, { type Map as LeafletMap } from "leaflet"
import { useEffect, useMemo } from "react"
import { CircleMarker, MapContainer, Marker, Polyline, Tooltip, TileLayer, useMap } from "react-leaflet"
import { MAP_CENTER, positionAlong, routes, traveledPath, type Route } from "@/lib/data/transit-data"
import type { LiveTruck } from "@/lib/data/use-fleet-simulation"

const TRUCK_SVG =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14 18V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v11a1 1 0 0 0 1 1h2"/><circle cx="7" cy="18" r="2"/><path d="M9 18h6"/><circle cx="17" cy="18" r="2"/><path d="M19 18h2a1 1 0 0 0 1-1v-3.65a1 1 0 0 0-.22-.624l-3.48-4.35A1 1 0 0 0 17.52 8H14"/><circle cx="17" cy="18" r="2"/></svg>'
const CHECK_SVG =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>'

type FleetMapProps = {
  fleet: LiveTruck[]
  visibleRouteIds: string[]
  selectedTruckId: string | null
  showRoutes: boolean
  focusKey: string
  onSelectTruck: (id: string) => void
  onReady: (map: LeafletMap) => void
}

export default function FleetMap({
  fleet,
  visibleRouteIds,
  selectedTruckId,
  showRoutes,
  focusKey,
  onSelectTruck,
  onReady,
}: FleetMapProps) {
  const selectedTruck = fleet.find((b) => b.id === selectedTruckId) ?? null
  const activeRouteId = selectedTruck?.routeId ?? null
  const visibleRoutes = routes.filter((r) => visibleRouteIds.includes(r.id))
  const focusRoutes = activeRouteId ? visibleRoutes.filter((r) => r.id === activeRouteId) : visibleRoutes

  return (
    <MapContainer
      center={MAP_CENTER}
      zoom={14}
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
      <FocusController focusKey={focusKey} routes={focusRoutes} />

      {showRoutes &&
        visibleRoutes.map((route) => (
          <RouteLayer
            key={route.id}
            route={route}
            active={route.id === activeRouteId}
            dimmed={activeRouteId !== null && route.id !== activeRouteId}
          />
        ))}

      {selectedTruck && showRoutes && selectedTruck.status !== "completed" && (
        <Polyline
          positions={traveledPath(routes.find((r) => r.id === selectedTruck.routeId)!, selectedTruck.progress)}
          pathOptions={{ className: "route-line route-line--progress", weight: 5, lineCap: "round" }}
          interactive={false}
        />
      )}

      {fleet.map((truck) => (
        <TruckMarker
          key={truck.id}
          truck={truck}
          selected={truck.id === selectedTruckId}
          onSelect={() => onSelectTruck(truck.id)}
        />
      ))}
    </MapContainer>
  )
}

function RouteLayer({ route, active, dimmed }: { route: Route; active: boolean; dimmed: boolean }) {
  const badgePosition = positionAlong(route, 0.5).latlng
  const badgeIcon = useMemo(
    () =>
      L.divIcon({
        className: "",
        html: `<span class="route-badge ${active ? "route-badge--active" : ""}">${route.number}</span>`,
        iconSize: [22, 22],
        iconAnchor: [11, 11],
      }),
    [route.number, active],
  )
  const lineClass = `route-line ${active ? "route-line--active" : ""} ${dimmed ? "route-line--dim" : ""}`

  return (
    <>
      <Polyline
        positions={route.path}
        pathOptions={{ className: lineClass, weight: active ? 4 : 3, lineJoin: "round", lineCap: "round" }}
        interactive={false}
      />
      {(!dimmed || active) &&
        route.stops.map((stop, i) => (
          <CircleMarker
            key={`${stop.name}-${i}`}
            center={positionAlong(route, stop.at).latlng}
            radius={active ? 5 : 4}
            pathOptions={{ className: `stop-dot ${active ? "stop-dot--active" : ""}`, weight: 2, fillOpacity: 1 }}
          >
            <Tooltip direction="top" offset={[0, -4]}>
              {stop.name}
            </Tooltip>
          </CircleMarker>
        ))}
      <Marker position={badgePosition} icon={badgeIcon} interactive={false} zIndexOffset={active ? 500 : 0} />
    </>
  )
}

function TruckMarker({ truck, selected, onSelect }: { truck: LiveTruck; selected: boolean; onSelect: () => void }) {
  const route = routes.find((r) => r.id === truck.routeId)!
  const { latlng, heading } = positionAlong(route, truck.progress)
  const roundedHeading = Math.round(heading / 15) * 15
  const completed = truck.status === "completed"

  const icon = useMemo(() => {
    const classes = [
      "bus-marker",
      completed && "bus-marker--completed",
      !completed && truck.status === "delayed" && "bus-marker--delayed",
      selected && "bus-marker--selected",
    ]
      .filter(Boolean)
      .join(" ")
    const headingEl = completed
      ? ""
      : `<span class="bus-marker__heading" style="rotate:${roundedHeading}deg"></span>`
    return L.divIcon({
      className: "",
      html: `<div class="${classes}">${headingEl}${completed ? CHECK_SVG : TRUCK_SVG}<span class="bus-marker__label">${truck.code}</span></div>`,
      iconSize: [34, 34],
      iconAnchor: [17, 17],
    })
  }, [truck.code, truck.status, completed, selected, roundedHeading])

  return (
    <Marker
      position={latlng}
      icon={icon}
      zIndexOffset={selected ? 1000 : completed ? -100 : 0}
      eventHandlers={{ click: onSelect }}
      title={`${truck.code} on corridor ${route.number}`}
      keyboard
    />
  )
}

function FocusController({ focusKey, routes: focusRoutes }: { focusKey: string; routes: Route[] }) {
  const map = useMap()

  useEffect(() => {
    if (focusRoutes.length === 0) return
    map.invalidateSize()
    const bounds = L.latLngBounds(focusRoutes.flatMap((r) => r.path))
    const wide = map.getContainer().clientWidth >= 768
    map.flyToBounds(bounds, {
      paddingTopLeft: [wide ? 380 : 24, 72],
      paddingBottomRight: [wide ? 80 : 24, 40],
      duration: 0.8,
      maxZoom: 16,
    })
  }, [focusKey, map, focusRoutes])

  return null
}
