"use client"

import { Truck } from "lucide-react"
import { ESTIMATED_POSITION_LABEL } from "@/components/map/FleetMapInner"
import type { Trip } from "@/lib/tripTypes"
import { cn } from "@/lib/utils"

export type StatusFilter = "all" | "active" | "completed"

type RoutePanelProps = {
  role: "operator" | "driver"
  trips: Trip[]
  selectedTripId: number | null
  onSelectTrip: (id: number | null) => void
  statusFilter: StatusFilter
  onStatusFilterChange: (s: StatusFilter) => void
}

const titles = {
  operator: { title: "Live trips", hint: "Dispatched trips across all places" },
  driver: { title: "My dispatched trip", hint: "Assigned vehicle and trip ends" },
} as const

function statusTone(status: string): string {
  if (status === "Completed") return "bg-sage text-primary"
  if (status === "Cancelled") return "bg-muted text-muted-foreground"
  return "bg-signal/15 text-signal-foreground"
}

export function RoutePanel(props: RoutePanelProps) {
  const { role, trips, selectedTripId, onSelectTrip } = props
  const { title, hint } = titles[role]

  return (
    <section
      aria-label={title}
      className="flex max-h-full flex-col overflow-hidden rounded-3xl border border-border bg-card/95 shadow-xl backdrop-blur"
    >
      <div className="flex flex-col gap-3 border-b border-border p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-foreground">{title}</h2>
            <p className="text-xs text-muted-foreground">{hint}</p>
          </div>
          <span className="rounded-full border border-border px-2.5 py-1 text-xs font-medium tabular-nums text-foreground">
            {trips.length} {trips.length === 1 ? "trip" : "trips"}
          </span>
        </div>
        {role !== "driver" && <PanelFilters {...props} />}
      </div>

      <div className="flex-1 overflow-y-auto p-3">
        {role === "driver" && trips[0] ? (
          <DriverTrip trip={trips[0]} />
        ) : trips.length === 0 ? (
          <p className="px-2 py-8 text-center text-sm text-muted-foreground">
            No trips match these filters.
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {trips.map((trip) => (
              <li key={trip.id}>
                <TripCard
                  trip={trip}
                  selected={trip.id === selectedTripId}
                  onSelect={() =>
                    onSelectTrip(trip.id === selectedTripId ? null : trip.id)
                  }
                />
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}

function PanelFilters({
  statusFilter,
  onStatusFilterChange,
}: RoutePanelProps) {
  const statuses: { value: StatusFilter; label: string }[] = [
    { value: "all", label: "All" },
    { value: "active", label: "Active" },
    { value: "completed", label: "Completed" },
  ]

  return (
    <div
      role="group"
      aria-label="Filter by status"
      className="flex flex-1 rounded-xl bg-muted p-1"
    >
      {statuses.map(({ value, label }) => {
        const active = statusFilter === value
        return (
          <button
            key={value}
            type="button"
            aria-pressed={active}
            onClick={() => onStatusFilterChange(value)}
            className={cn(
              "flex-1 rounded-lg py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
              active
                ? "bg-card text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {label}
          </button>
        )
      })}
    </div>
  )
}

function TripCard({
  trip,
  selected,
  onSelect,
}: {
  trip: Trip
  selected: boolean
  onSelect: () => void
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      aria-label={`Trip ${trip.id}, ${trip.vehicle_reg}, from ${trip.source_name} to ${trip.destination_name}, status ${trip.status}`}
      className={cn(
        "group flex w-full flex-col gap-2 rounded-2xl border p-3 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        selected
          ? "border-primary bg-primary/5 shadow-md ring-1 ring-primary"
          : "border-border bg-card hover:border-primary/40 hover:bg-muted/40",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="font-semibold text-foreground">
          {trip.vehicle_reg || `Trip ${trip.id}`}
        </span>
        <span
          className={cn(
            "rounded-md px-1.5 py-0.5 text-[11px] font-medium",
            statusTone(trip.status),
          )}
        >
          {trip.status}
        </span>
      </div>

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span className="size-2 shrink-0 rounded-full bg-chart-5" aria-hidden="true" />
        <span className="truncate font-serif text-base text-foreground">
          {trip.source_name} to <span className="italic">{trip.destination_name}</span>
        </span>
        <span className="size-2 shrink-0 rounded-full bg-signal" aria-hidden="true" />
      </div>

      <div aria-hidden="true" className="flex items-center gap-1.5">
        <span className="h-0.5 flex-1 rounded-full bg-muted" />
        <span className="grid size-6 place-items-center rounded-full border border-border bg-card shadow-sm">
          <Truck className="size-3.5 text-signal-foreground" />
        </span>
        <span className="h-0.5 flex-1 rounded-full bg-muted" />
      </div>

      <p className="text-xs text-muted-foreground">
        Driver {trip.driver_license} · Cargo {trip.cargo_weight} kg
      </p>
      <p className="text-[11px] text-muted-foreground">{ESTIMATED_POSITION_LABEL}</p>
    </button>
  )
}

function DriverTrip({ trip }: { trip: Trip }) {
  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-2xl border border-border bg-muted/30 p-4">
        <p className="text-xs font-medium text-muted-foreground">Assigned vehicle</p>
        <p className="mt-1 text-lg font-bold text-foreground">
          {trip.vehicle_reg || `Trip ${trip.id}`}
        </p>
        <p className="mt-1 font-serif text-lg text-foreground">
          {trip.source_name} to <span className="italic">{trip.destination_name}</span>
        </p>
        <div aria-hidden="true" className="mt-3 flex items-center gap-1.5">
          <span className="size-2 shrink-0 rounded-full bg-chart-5" />
          <span className="h-0.5 flex-1 rounded-full bg-border" />
          <span className="grid size-6 shrink-0 place-items-center rounded-full bg-card shadow-sm">
            <Truck className="size-3.5 text-signal-foreground" />
          </span>
          <span className="h-0.5 flex-1 rounded-full bg-border" />
          <span className="size-2 shrink-0 rounded-full bg-signal" />
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          Driver {trip.driver_license} · Cargo {trip.cargo_weight} kg · {trip.status}
        </p>
        <p className="mt-2 text-[11px] text-muted-foreground">{ESTIMATED_POSITION_LABEL}</p>
      </div>
    </div>
  )
}
