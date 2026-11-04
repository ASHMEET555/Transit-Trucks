"use client"

import "leaflet/dist/leaflet.css"
import L, { type Map as LeafletMap } from "leaflet"
import React, { useEffect, useMemo, useState } from "react"
import { CircleMarker, MapContainer, Marker, Polyline, Popup, Tooltip, TileLayer, useMap, useMapEvents } from "react-leaflet"
import { DEPOTS, MAP_CENTER } from "@/lib/data/transit-data"
import { createTruckDivIcon } from "@/components/map/truck-marker"
import { decodePolyline6, clusterMarkers } from "@/lib/live/liveUtils.mjs"

export type LiveTruckItem = {
  id: string
  vehicle_reg: string
  trip_id: number
  driver?: { name: string; phone?: string }
  driver_name?: string | null
  cargo_type?: string
  cargo_weight?: number
  speed_kmh?: number | null
  fuel_pct?: number | null
  reefer_temp_c?: number | null
  heading_deg?: number | null
  latitude?: number | null
  longitude?: number | null
  progress_pct?: number | null
  miles_remaining?: number | null
  eta?: string | null
  delay_minutes?: number | null
  status: string
  freshness?: string
  alerts?: string[]
  source?: string | null
  route_polyline?: string | null
  route_provider?: string | null
  origin?: { name: string; latitude?: number | null; longitude?: number | null }
  destination?: { name: string; latitude?: number | null; longitude?: number | null }
}

type FleetMapProps = {
  fleet: LiveTruckItem[]
  selectedTruckId: string | null
  showRoutes: boolean
  onSelectTruck: (id: string) => void
  onReady: (map: LeafletMap) => void
}

export default function FleetMap({
  fleet,
  selectedTruckId,
  showRoutes,
  onSelectTruck,
  onReady,
}: FleetMapProps) {
  const selectedTruck = fleet.find((b) => b.vehicle_reg === selectedTruckId || b.id === selectedTruckId) ?? null

  return (
    <MapContainer
      center={MAP_CENTER}
      zoom={4}
      minZoom={3}
      maxZoom={12}
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

      {/* Depots / Hubs with permanent labels at all zoom levels */}
      {DEPOTS.map((depot) => (
        <DepotMarker key={depot.id} depot={depot} />
      ))}

      {/* Road Routes */}
      {showRoutes && <RoadRoutesLayer fleet={fleet} selectedTruck={selectedTruck} />}

      {/* Cluster-aware Truck Markers */}
      <ClusterAwareTrucksLayer
        fleet={fleet}
        selectedTruckId={selectedTruckId}
        onSelectTruck={onSelectTruck}
      />
    </MapContainer>
  )
}

function DepotMarker({ depot }: { depot: (typeof DEPOTS)[0] }) {
  const icon = useMemo(
    () =>
      L.divIcon({
        className: "depot-hub-icon-wrap",
        html: `
          <div style="position:relative; display:flex; flex-direction:column; align-items:center;">
            <div style="width:14px; height:14px; border-radius:9999px; background:#1b4332; border:2.5px solid #ffffff; box-shadow:0 2px 6px rgba(0,0,0,0.35);"></div>
            <span style="margin-top:2px; font-size:10px; font-weight:700; color:#1b4332; background:rgba(255,255,255,0.92); padding:1px 5px; border-radius:9999px; border:1px solid rgba(27,67,50,0.2); white-space:nowrap; box-shadow:0 1px 3px rgba(0,0,0,0.12); pointer-events:none;">
              ${depot.city}
            </span>
          </div>
        `,
        iconSize: [60, 36],
        iconAnchor: [30, 7],
      }),
    [depot.city],
  )

  return (
    <Marker position={depot.coords} icon={icon} interactive={false} zIndexOffset={50}>
      <Tooltip direction="bottom" offset={[0, 10]}>
        <div className="font-semibold text-foreground">{depot.name}</div>
        <div className="text-xs text-muted-foreground">{depot.city} Depot</div>
      </Tooltip>
    </Marker>
  )
}

function RoadRoutesLayer({
  fleet,
  selectedTruck,
}: {
  fleet: LiveTruckItem[]
  selectedTruck: LiveTruckItem | null
}) {
  const renderedRoutes = useMemo(() => {
    const map = new Map<string, { pts: [number, number][]; provider: string; active: boolean }>()
    for (const t of fleet) {
      if (!t.route_polyline) continue
      const key = `${t.origin?.name ?? ""}->${t.destination?.name ?? ""}`
      if (!map.has(key)) {
        try {
          const pts = decodePolyline6(t.route_polyline)
          const active = selectedTruck?.route_polyline === t.route_polyline
          map.set(key, { pts, provider: t.route_provider ?? "osrm", active })
        } catch {
          // ignore corrupted polyline
        }
      }
    }
    return Array.from(map.entries())
  }, [fleet, selectedTruck])

  return (
    <>
      {renderedRoutes.map(([key, item]) => {
        const isApprox = item.provider === "approximate"
        const isSelected = item.active
        return (
          <React.Fragment key={key}>
            <Polyline
              positions={item.pts}
              pathOptions={{
                color: isSelected ? "#1b4332" : "#94a3b8",
                weight: isSelected ? 4 : 2.5,
                opacity: isSelected ? 0.95 : 0.45,
                dashArray: isApprox ? "6, 8" : undefined,
                lineJoin: "round",
                lineCap: "round",
              }}
              interactive={false}
            />
          </React.Fragment>
        )
      })}
    </>
  )
}

function ClusterAwareTrucksLayer({
  fleet,
  selectedTruckId,
  onSelectTruck,
}: {
  fleet: LiveTruckItem[]
  selectedTruckId: string | null
  onSelectTruck: (id: string) => void
}) {
  const map = useMap()
  const [, setZoom] = useState(map.getZoom())

  useMapEvents({
    zoomend() {
      setZoom(map.getZoom())
    },
    moveend() {
      setZoom(map.getZoom())
    },
  })

  // Valid trucks with coordinates
  const placedTrucks = useMemo(
    () =>
      fleet.filter(
        (t) =>
          typeof t.latitude === "number" &&
          typeof t.longitude === "number" &&
          !Number.isNaN(t.latitude) &&
          !Number.isNaN(t.longitude),
      ),
    [fleet],
  )

  // 40px cluster algorithm
  const clusters = useMemo(() => {
    return clusterMarkers(
      placedTrucks,
      (lat: number, lon: number) => map.latLngToLayerPoint([lat, lon]),
      40,
    )
  }, [placedTrucks, map])

  return (
    <>
      {clusters.map((c) => {
        if (c.members.length === 1) {
          const truck = c.members[0] as LiveTruckItem
          const isSelected = truck.vehicle_reg === selectedTruckId || truck.id === selectedTruckId
          const isReefer =
            truck.cargo_type === "Refrigerated" ||
            truck.cargo_type?.toLowerCase().includes("reefer")
          const heading = truck.heading_deg ?? 0

          const icon = createTruckDivIcon({
            vehicleReg: truck.vehicle_reg,
            headingDeg: heading,
            status: truck.status,
            alerts: truck.alerts,
            selected: isSelected,
            isReefer: isReefer,
          })

          return (
            <Marker
              key={truck.vehicle_reg}
              position={[truck.latitude!, truck.longitude!]}
              icon={icon}
              zIndexOffset={isSelected ? 1000 : 100}
              eventHandlers={{
                click: () => onSelectTruck(truck.vehicle_reg),
              }}
            >
              <Popup className="simulated-truck-popup" offset={[0, -18]}>
                <div className="p-1 text-xs">
                  <div className="flex items-center justify-between gap-3 border-b border-border pb-1 font-semibold text-foreground">
                    <span>{truck.vehicle_reg}</span>
                    <span className="rounded-full bg-sand px-2 py-0.5 text-[10px] font-medium text-foreground">
                      Simulated
                    </span>
                  </div>
                  <div className="mt-1.5 space-y-0.5 text-muted-foreground">
                    <div>
                      <span className="font-medium text-foreground">Status:</span> {truck.status}
                    </div>
                    {truck.speed_kmh != null && (
                      <div>
                        <span className="font-medium text-foreground">Speed:</span>{" "}
                        {Math.round(truck.speed_kmh)} km/h
                      </div>
                    )}
                    {truck.fuel_pct != null && (
                      <div>
                        <span className="font-medium text-foreground">Fuel:</span>{" "}
                        {Math.round(truck.fuel_pct)}%
                      </div>
                    )}
                    {isReefer && truck.reefer_temp_c != null && (
                      <div>
                        <span className="font-medium text-foreground">Trailer Temp:</span>{" "}
                        {truck.reefer_temp_c.toFixed(1)}°C
                      </div>
                    )}
                  </div>
                </div>
              </Popup>
            </Marker>
          )
        }

        // Multiple trucks: render cluster bubble
        const containsSelected = c.members.some(
          (m: LiveTruckItem) => m.vehicle_reg === selectedTruckId || m.id === selectedTruckId,
        )
        const clusterIcon = L.divIcon({
          className: "truck-cluster-bubble-wrap",
          html: `
            <div style="
              width:34px;
              height:34px;
              border-radius:9999px;
              background:${containsSelected ? "#1b4332" : "#ffffff"};
              color:${containsSelected ? "#ffffff" : "#1b4332"};
              border:2.5px solid ${containsSelected ? "#ffffff" : "#1b4332"};
              box-shadow:0 3px 10px rgba(0,0,0,0.2);
              display:grid;
              place-items:center;
              font-size:12px;
              font-weight:700;
              cursor:pointer;
            ">
              ${c.members.length}
            </div>
          `,
          iconSize: [34, 34],
          iconAnchor: [17, 17],
        })

        return (
          <Marker
            key={c.key}
            position={[c.lat, c.lon]}
            icon={clusterIcon}
            zIndexOffset={containsSelected ? 900 : 200}
            eventHandlers={{
              click: () => {
                const first = c.members[0] as LiveTruckItem
                onSelectTruck(first.vehicle_reg)
              },
            }}
          />
        )
      })}
    </>
  )
}
