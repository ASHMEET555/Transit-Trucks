"use client"

import { CircleCheck, Clock, MapPin, Phone, UserRound, Weight } from "lucide-react"
import {
  etaMinutes,
  fleets,
  getRoute,
  nextStop,
  viewerProfiles,
  type ViewerRole,
} from "@/lib/data/transit-data"
import type { TruckStatus, LiveTruck, TripLogEntry } from "@/lib/data/use-fleet-simulation"
import { cn } from "@/lib/utils"

export type StatusFilter = "all" | "active" | "completed"

type RoutePanelProps = {
  role: ViewerRole
  trucks: LiveTruck[]
  selectedTruckId: string | null
  onSelectTruck: (id: string) => void
  statusFilter: StatusFilter
  onStatusFilterChange: (s: StatusFilter) => void
  fleetFilter: string
  onFleetFilterChange: (f: string) => void
  log: TripLogEntry[]
}

const titles: Record<ViewerRole, { title: string; hint: string }> = {
  operator: { title: "Live Freight Fleet", hint: "Active trucks across all depots" },
  driver: { title: "My Dispatched Trip", hint: "Assigned vehicle and corridor stops" },
}

export function RoutePanel(props: RoutePanelProps) {
  const { role, trucks, selectedTruckId, onSelectTruck, log } = props
  const { title, hint } = titles[role]

  return (
    <section
      aria-label={title}
      className="flex max-h-full flex-col overflow-hidden rounded-3xl border border-border bg-card/95 shadow-xl backdrop-blur"
    >
      <div className="flex flex-col gap-3 border-b border-border p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold">{title}</h2>
            <p className="text-xs text-muted-foreground">{hint}</p>
          </div>
          <span className="rounded-full border border-border px-2.5 py-1 text-xs font-medium tabular-nums">
            {trucks.length} {trucks.length === 1 ? "truck" : "trucks"}
          </span>
        </div>
        {role !== "driver" && <PanelFilters {...props} />}
      </div>

      <div className="flex-1 overflow-y-auto p-3">
        {role === "driver" && trucks[0] ? (
          <DriverTrip truck={trucks[0]} />
        ) : trucks.length === 0 ? (
          <p className="px-2 py-8 text-center text-sm text-muted-foreground">No trucks match these filters.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {trucks.map((truck) => (
              <li key={truck.id}>
                <TruckCard truck={truck} role={role} selected={truck.id === selectedTruckId} onSelect={() => onSelectTruck(truck.id)} />
              </li>
            ))}
          </ul>
        )}

        {role === "operator" && log.length > 0 && (
          <div className="mt-4 border-t border-border px-1 pt-4">
            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">Recently completed</h3>
            <ul className="flex flex-col gap-2" aria-live="polite">
              {log.slice(0, 5).map((entry) => (
                <li key={entry.id} className="flex items-center gap-2 text-sm">
                  <CircleCheck className="size-4 text-chart-5" aria-hidden="true" />
                  <span className="font-medium">{entry.truckCode}</span>
                  <span className="text-muted-foreground">finished corridor {getRoute(entry.routeId).number}</span>
                  <time className="ml-auto text-xs tabular-nums text-muted-foreground">
                    {entry.at.toLocaleTimeString([], { minute: "2-digit", second: "2-digit" })}
                  </time>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </section>
  )
}

function PanelFilters({
  statusFilter,
  onStatusFilterChange,
  fleetFilter,
  onFleetFilterChange,
}: RoutePanelProps) {
  const statuses: { value: StatusFilter; label: string }[] = [
    { value: "all", label: "All" },
    { value: "active", label: "Active" },
    { value: "completed", label: "Done" },
  ]

  return (
    <div className="flex items-center gap-2">
      <div role="group" aria-label="Filter by status" className="flex flex-1 rounded-xl bg-muted p-1">
        {statuses.map(({ value, label }) => {
          const active = statusFilter === value
          return (
            <button
              key={value}
              type="button"
              aria-pressed={active}
              onClick={() => onStatusFilterChange(value)}
              className={cn(
                "flex-1 rounded-lg py-1 text-xs font-medium transition-colors",
                active ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {label}
            </button>
          )
        })}
      </div>

      <select
        value={fleetFilter}
        onChange={(e) => onFleetFilterChange(e.target.value)}
        aria-label="Filter by depot"
        className="rounded-xl border border-border bg-card px-2.5 py-1.5 text-xs font-medium text-foreground outline-none"
      >
        <option value="all">All depots</option>
        {fleets.map((f) => (
          <option key={f.id} value={f.id}>
            {f.name}
          </option>
        ))}
      </select>
    </div>
  )
}

function TruckCard({
  truck,
  selected,
  onSelect,
}: {
  truck: LiveTruck
  role: ViewerRole
  selected: boolean
  onSelect: () => void
}) {
  const route = getRoute(truck.routeId)
  const upcoming = nextStop(route, truck.progress)
  const eta = etaMinutes(route.id, truck.progress, upcoming.at)
  const isCompleted = truck.status === "completed"

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        "group flex w-full flex-col gap-2 rounded-2xl border p-3 text-left transition-all",
        selected
          ? "border-primary bg-primary/5 shadow-md ring-1 ring-primary"
          : "border-border bg-card hover:border-primary/40 hover:bg-muted/40",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="font-semibold text-foreground">{truck.code}</span>
        <span className="rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
          {route.name}
        </span>
      </div>

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <MapPin className="size-3.5 shrink-0" aria-hidden="true" />
        <span className="truncate">Next: {upcoming.name}</span>
        <span className="ml-auto font-medium text-foreground">
          {isCompleted ? "Completed" : `${eta}m`}
        </span>
      </div>
    </button>
  )
}

function DriverTrip({ truck }: { truck: LiveTruck }) {
  const route = getRoute(truck.routeId)

  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-2xl border border-border bg-muted/30 p-4">
        <p className="text-xs uppercase tracking-wider text-muted-foreground">Assigned Truck</p>
        <p className="mt-1 text-lg font-bold">{truck.code} ({truck.plate})</p>
        <p className="text-xs text-muted-foreground">Route: {route.name}</p>
        <div className="mt-3 flex items-center gap-4 text-xs">
          <span className="flex items-center gap-1.5">
            <Weight className="size-3.5 text-muted-foreground" />
            {(truck.cargoWeight / 1000).toFixed(1)}t cargo
          </span>
          <span className="flex items-center gap-1.5">
            <Clock className="size-3.5 text-muted-foreground" />
            {truck.status === "completed" ? "Layover" : "In transit"}
          </span>
        </div>
      </div>

      <div>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Corridor Stops</h3>
        <ol className="flex flex-col gap-2">
          {route.stops.map((s, idx) => {
            const passed = truck.progress >= s.at
            return (
              <li key={s.name} className="flex items-center gap-2 text-sm">
                <span className={cn(
                  "grid size-5 place-items-center rounded-full text-[10px] font-bold",
                  passed ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground"
                )}>
                  {idx + 1}
                </span>
                <span className={passed ? "line-through text-muted-foreground" : "font-medium"}>
                  {s.name}
                </span>
                <span className="ml-auto text-xs text-muted-foreground">
                  {passed ? "Passed" : `${etaMinutes(route.id, truck.progress, s.at)} min`}
                </span>
              </li>
            )
          })}
        </ol>
      </div>
    </div>
  )
}
