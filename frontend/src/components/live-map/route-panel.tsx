"use client"

import React, { useMemo, useState } from "react"
import { AlertCircle, Clock, Fuel, Gauge, Snowflake, Thermometer, Truck, X } from "lucide-react"
import { DEPOTS } from "@/lib/data/transit-data"
import type { LiveTruckItem } from "./fleet-map"
import { cn } from "@/lib/utils"

export type StatusFilter = "all" | "on_route" | "delayed" | "alert" | "offline"
export type CargoFilter = "all" | "reefer" | "dry_van"

type RoutePanelProps = {
  trucks: LiveTruckItem[]
  selectedTruckId: string | null
  onSelectTruck: (id: string | null) => void
}

export function RoutePanel({
  trucks,
  selectedTruckId,
  onSelectTruck,
}: RoutePanelProps) {
  const [query, setQuery] = useState("")
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all")
  const [depotFilter, setDepotFilter] = useState("all")
  const [cargoFilter, setCargoFilter] = useState<CargoFilter>("all")

  const selectedTruck = useMemo(
    () => trucks.find((t) => t.vehicle_reg === selectedTruckId || t.id === selectedTruckId) ?? null,
    [trucks, selectedTruckId],
  )

  const filteredTrucks = useMemo(() => {
    const q = query.trim().toLowerCase()
    return trucks.filter((t) => {
      // Status filter
      if (statusFilter === "on_route" && t.status !== "on_route") return false
      if (statusFilter === "delayed" && t.status !== "delayed") return false
      if (statusFilter === "alert" && (!t.alerts || t.alerts.length === 0)) return false
      if (statusFilter === "offline" && t.status !== "no_signal" && t.status !== "offline") return false

      // Depot filter
      if (depotFilter !== "all") {
        const matchesOrigin = t.origin?.name?.toLowerCase().includes(depotFilter.toLowerCase())
        const matchesDest = t.destination?.name?.toLowerCase().includes(depotFilter.toLowerCase())
        if (!matchesOrigin && !matchesDest) return false
      }

      // Cargo filter
      const isReefer =
        t.cargo_type === "Refrigerated" || t.cargo_type?.toLowerCase().includes("reefer")
      if (cargoFilter === "reefer" && !isReefer) return false
      if (cargoFilter === "dry_van" && isReefer) return false

      // Search query
      if (!q) return true
      return (
        t.vehicle_reg.toLowerCase().includes(q) ||
        (t.driver_name && t.driver_name.toLowerCase().includes(q)) ||
        (t.driver?.name && t.driver.name.toLowerCase().includes(q)) ||
        (t.origin?.name && t.origin.name.toLowerCase().includes(q)) ||
        (t.destination?.name && t.destination.name.toLowerCase().includes(q))
      )
    })
  }, [trucks, query, statusFilter, depotFilter, cargoFilter])

  return (
    <aside
      aria-label="Fleet Route Panel"
      className="flex max-h-full flex-col overflow-hidden rounded-3xl border border-border bg-card/95 shadow-xl backdrop-blur"
    >
      {/* If a truck is selected, show the Dark Forest Selected-Truck Card at top */}
      {selectedTruck ? (
        <SelectedTruckCard
          truck={selectedTruck}
          onClose={() => onSelectTruck(null)}
        />
      ) : (
        <div className="flex flex-col gap-3 border-b border-border p-4">
          <div className="flex items-center justify-between gap-2">
            <div>
              <h2 className="text-base font-semibold text-foreground">Active Fleet</h2>
              <p className="text-xs text-muted-foreground">Real-time simulated telemetry</p>
            </div>
            <span className="rounded-full bg-sand px-2.5 py-0.5 text-xs font-semibold text-foreground">
              {filteredTrucks.length} trucks
            </span>
          </div>

          {/* Search Box */}
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search truck, driver, depot…"
            className="w-full rounded-xl border border-border bg-background px-3 py-1.5 text-xs text-foreground outline-none placeholder:text-muted-foreground focus:border-primary"
          />

          {/* Status Filter Tabs */}
          <div className="flex rounded-xl bg-muted p-0.5">
            {[
              { id: "all", label: "All" },
              { id: "on_route", label: "On Route" },
              { id: "delayed", label: "Delayed" },
              { id: "alert", label: "Alert" },
              { id: "offline", label: "Offline" },
            ].map((f) => (
              <button
                key={f.id}
                type="button"
                onClick={() => setStatusFilter(f.id as StatusFilter)}
                className={cn(
                  "flex-1 rounded-lg py-1 text-[11px] font-medium transition-colors",
                  statusFilter === f.id
                    ? "bg-card font-semibold text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {f.label}
              </button>
            ))}
          </div>

          {/* Dropdown Filters: Depot & Cargo Type */}
          <div className="grid grid-cols-2 gap-2 text-xs">
            <select
              value={depotFilter}
              onChange={(e) => setDepotFilter(e.target.value)}
              className="rounded-xl border border-border bg-background px-2.5 py-1 text-xs text-foreground outline-none"
            >
              <option value="all">All Depots</option>
              {DEPOTS.map((d) => (
                <option key={d.id} value={d.city}>
                  {d.city}
                </option>
              ))}
            </select>

            <select
              value={cargoFilter}
              onChange={(e) => setCargoFilter(e.target.value as CargoFilter)}
              className="rounded-xl border border-border bg-background px-2.5 py-1 text-xs text-foreground outline-none"
            >
              <option value="all">All Cargo</option>
              <option value="reefer">Reefer Only</option>
              <option value="dry_van">Dry Van</option>
            </select>
          </div>
        </div>
      )}

      {/* Truck List */}
      <div className="flex-1 overflow-y-auto p-3">
        {filteredTrucks.length === 0 ? (
          <p className="px-2 py-8 text-center text-xs text-muted-foreground">
            No trucks match these filters.
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {filteredTrucks.map((truck) => {
              const isSelected =
                truck.vehicle_reg === selectedTruckId || truck.id === selectedTruckId
              const isReefer =
                truck.cargo_type === "Refrigerated" ||
                truck.cargo_type?.toLowerCase().includes("reefer")
              const driver = truck.driver_name || truck.driver?.name || "Driver Unassigned"
              const loadKg = Math.round(truck.cargo_weight ?? 16500)
              const milesRem =
                truck.miles_remaining != null ? Math.round(truck.miles_remaining) : null
              const delay = truck.delay_minutes ?? 0

              return (
                <li key={truck.vehicle_reg}>
                  <button
                    type="button"
                    onClick={() => onSelectTruck(truck.vehicle_reg)}
                    className={cn(
                      "group flex w-full flex-col gap-1.5 rounded-2xl border p-3 text-left transition-all",
                      isSelected
                        ? "border-primary bg-primary/5 shadow-md ring-1 ring-primary"
                        : "border-border bg-card hover:border-primary/40 hover:bg-muted/30",
                    )}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-1.5 font-semibold text-foreground">
                        <span>{truck.vehicle_reg}</span>
                        {isReefer && (
                          <span
                            className="inline-flex items-center gap-0.5 rounded-full bg-sky-100 px-1.5 py-0.2 text-[10px] font-medium text-sky-800"
                            title="Refrigerated"
                          >
                            <Snowflake className="size-2.5" /> Reefer
                          </span>
                        )}
                      </div>
                      <span
                        className={cn(
                          "rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider",
                          truck.alerts && truck.alerts.length > 0
                            ? "bg-destructive/15 text-destructive"
                            : truck.status === "delayed"
                              ? "bg-signal/20 text-signal-foreground"
                              : truck.status === "on_route"
                                ? "bg-primary/10 text-primary"
                                : "bg-muted text-muted-foreground",
                        )}
                      >
                        {truck.alerts && truck.alerts.length > 0
                          ? truck.alerts[0].replace(/_/g, " ")
                          : truck.status.replace(/_/g, " ")}
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-xs text-muted-foreground">
                      <span className="truncate">{driver}</span>
                      <span className="tabular-nums font-medium text-foreground">
                        {loadKg.toLocaleString()} kg
                      </span>
                    </div>

                    <div className="flex items-center justify-between pt-0.5 text-[11px] text-muted-foreground">
                      <span>{truck.eta ? `est. ${truck.eta}` : "ETA —"}</span>
                      {milesRem != null && <span>{milesRem} mi remaining</span>}
                      {delay > 0 && (
                        <span className="font-semibold text-destructive">+{delay} min delay</span>
                      )}
                    </div>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </aside>
  )
}

/**
 * Dark Forest Selected Truck Card (as specified in rule 6):
 * Origin and destination in serif italic, progress bar at progress_pct,
 * ETA with "est.", delay, fuel %, speed, and trailer temperature if reefer.
 */
function SelectedTruckCard({
  truck,
  onClose,
}: {
  truck: LiveTruckItem
  onClose: () => void
}) {
  const isReefer =
    truck.cargo_type === "Refrigerated" || truck.cargo_type?.toLowerCase().includes("reefer")
  const pct = Math.min(100, Math.max(0, truck.progress_pct ?? 0))
  const orig = truck.origin?.name?.replace(" Logistics Hub", "").replace(" Intermodal Hub", "").replace(" Gateway Hub", "").replace(" Terminal", "").replace(" Freight Depot", "").replace(" Distribution Hub", "") || "Origin"
  const dest = truck.destination?.name?.replace(" Logistics Hub", "").replace(" Intermodal Hub", "").replace(" Gateway Hub", "").replace(" Terminal", "").replace(" Freight Depot", "").replace(" Distribution Hub", "") || "Destination"

  return (
    <div className="relative overflow-hidden rounded-2xl bg-[#1b4332] p-4 text-[#e8f5e9] shadow-2xl">
      <div className="flex items-center justify-between border-b border-white/10 pb-2">
        <div className="flex items-center gap-2">
          <Truck className="size-4 text-[#74c69d]" />
          <span className="font-semibold tracking-wide text-white">{truck.vehicle_reg}</span>
          <span className="rounded-full bg-white/15 px-2 py-0.5 text-[10px] font-medium tracking-wide text-[#b7e4c7]">
            Simulated
          </span>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close truck card"
          className="grid size-7 place-items-center rounded-full bg-white/10 text-white/80 transition-colors hover:bg-white/20 hover:text-white"
        >
          <X className="size-4" />
        </button>
      </div>

      {/* Origin & Destination in Serif Italic */}
      <div className="mt-3 flex items-center justify-between gap-2">
        <div>
          <span className="block text-[10px] uppercase tracking-wider text-[#b7e4c7]/80">From</span>
          <span className="font-serif text-base italic font-normal text-white">{orig}</span>
        </div>
        <span className="text-[#74c69d]">→</span>
        <div className="text-right">
          <span className="block text-[10px] uppercase tracking-wider text-[#b7e4c7]/80">To</span>
          <span className="font-serif text-base italic font-normal text-white">{dest}</span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="mt-3">
        <div className="flex items-center justify-between text-xs text-[#b7e4c7]">
          <span>Progress</span>
          <span className="font-semibold tabular-nums text-white">{Math.round(pct)}%</span>
        </div>
        <div className="relative mt-1 h-2 w-full overflow-hidden rounded-full bg-black/30">
          <div
            className="h-full rounded-full bg-[#74c69d] transition-all duration-500 ease-out"
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>

      {/* Telemetry Stats: ETA, Delay, Fuel, Speed, Temp */}
      <div className="mt-3 grid grid-cols-2 gap-2 text-xs">
        <div className="flex items-center gap-1.5 rounded-lg bg-black/20 p-2">
          <Clock className="size-3.5 text-[#74c69d]" />
          <div>
            <span className="block text-[10px] text-[#b7e4c7]">ETA</span>
            <span className="font-semibold text-white">
              {truck.eta ? `est. ${truck.eta}` : "est. —"}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-1.5 rounded-lg bg-black/20 p-2">
          <AlertCircle className="size-3.5 text-[#ffb703]" />
          <div>
            <span className="block text-[10px] text-[#b7e4c7]">Delay</span>
            <span className="font-semibold text-white">
              {truck.delay_minutes ? `+${truck.delay_minutes} min` : "On time"}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-1.5 rounded-lg bg-black/20 p-2">
          <Fuel className="size-3.5 text-[#74c69d]" />
          <div>
            <span className="block text-[10px] text-[#b7e4c7]">Fuel</span>
            <span className="font-semibold text-white">
              {truck.fuel_pct != null ? `${Math.round(truck.fuel_pct)}%` : "78%"}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-1.5 rounded-lg bg-black/20 p-2">
          <Gauge className="size-3.5 text-[#74c69d]" />
          <div>
            <span className="block text-[10px] text-[#b7e4c7]">Speed</span>
            <span className="font-semibold text-white">
              {truck.speed_kmh != null ? `${Math.round(truck.speed_kmh)} km/h` : "84 km/h"}
            </span>
          </div>
        </div>

        {isReefer && (
          <div className="col-span-2 flex items-center gap-2 rounded-lg bg-sky-950/60 p-2 text-sky-200">
            <Thermometer className="size-4 text-sky-300" />
            <div className="flex flex-1 items-center justify-between">
              <span className="text-[11px]">Trailer Temperature</span>
              <span className="font-bold text-white">
                {truck.reefer_temp_c != null ? `${truck.reefer_temp_c.toFixed(1)}°C` : "-20.0°C"}
              </span>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
