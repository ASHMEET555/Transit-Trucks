"use client"

import type { ReactNode } from "react"
import { RiInboxLine } from "@remixicon/react"
import { CardShell } from "@/components/fleet/card-shell"
import { Button } from "@/components/Button"
import { Skeleton } from "@/components/Skeleton"
import { StatusBadge } from "./StatusBadge"
import { TRIP_STATUSES, formatNumber, type Trip } from "./types"

interface TripBoardProps {
  trips: Trip[]
  loading: boolean
  visibleStatuses: string[]
  onOpen: (trip: Trip) => void
  renderActions: (trip: Trip) => ReactNode
  draftEmptyAction: ReactNode
}

function TripCard({
  trip,
  onOpen,
  actions,
}: {
  trip: Trip
  onOpen: () => void
  actions: ReactNode
}) {
  return (
    <CardShell
      as="article"
      aria-label={`Trip ${trip.id} from ${trip.source_name} to ${trip.destination_name}`}
      className="rounded-3xl p-4 sm:p-4"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="font-mono text-sm font-semibold tabular-nums text-foreground">
          #{trip.id}
        </span>
        <StatusBadge status={trip.status} />
      </div>
      <p className="mt-2 font-serif text-[15px] font-medium text-foreground">
        {trip.source_name} <span aria-hidden="true">→</span> {trip.destination_name}
      </p>
      <dl className="mt-2 space-y-1 text-xs text-muted-foreground">
        <div className="flex justify-between gap-2">
          <dt>Vehicle</dt>
          <dd className="font-serif text-foreground">
            {trip.vehicle_reg}
          </dd>
        </div>
        <div className="flex justify-between gap-2">
          <dt>Driver</dt>
          <dd className="font-mono tabular-nums text-foreground">
            {trip.driver_license}
          </dd>
        </div>
        <div className="flex justify-between gap-2">
          <dt>Cargo</dt>
          <dd className="tabular-nums text-foreground">
            {formatNumber(trip.cargo_weight)} kg
          </dd>
        </div>
        <div className="flex justify-between gap-2">
          <dt>Revenue</dt>
          <dd className="tabular-nums text-foreground">
            ${formatNumber(trip.revenue)}
          </dd>
        </div>
      </dl>
      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onOpen}>
          View
        </Button>
        {actions}
      </div>
    </CardShell>
  )
}

function ColumnSkeleton() {
  return (
    <div aria-hidden="true" className="space-y-3">
      {[0, 1, 2].map((key) => (
        <div
          key={key}
          className="rounded-2xl border border-border bg-card p-3"
        >
          <Skeleton className="h-5 w-1/3 motion-reduce:animate-none" />
          <Skeleton className="mt-2 h-4 w-3/4 motion-reduce:animate-none" />
          <Skeleton className="mt-2 h-4 w-1/2 motion-reduce:animate-none" />
        </div>
      ))}
    </div>
  )
}

export function TripBoard({
  trips,
  loading,
  visibleStatuses,
  onOpen,
  renderActions,
  draftEmptyAction,
}: TripBoardProps) {
  return (
    <div
      className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4"
      role="list"
      aria-label="Trips by status"
    >
      {TRIP_STATUSES.filter((status) => visibleStatuses.includes(status)).map(
        (status) => {
          const column = trips.filter((trip) => trip.status === status)
          return (
            <section
              key={status}
              role="listitem"
              aria-label={`${status} trips, ${column.length} items`}
              className="flex min-h-40 flex-col rounded-2xl border border-border bg-muted p-3"
            >
              <header className="mb-3 flex items-center justify-between gap-2">
                <StatusBadge status={status} />
                <span className="text-xs font-medium tabular-nums text-muted-foreground">
                  {loading ? "…" : column.length}
                </span>
              </header>
              {loading ? (
                <ColumnSkeleton />
              ) : column.length === 0 ? (
                <div className="flex flex-1 flex-col items-center justify-center gap-2 rounded-2xl border border-dashed border-border bg-card px-3 py-8 text-center">
                  <RiInboxLine
                    className="size-6 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <p className="text-sm text-muted-foreground">
                    {status === "Draft"
                      ? "No draft trips. Start one to plan the next dispatch."
                      : `No ${status.toLowerCase()} trips right now.`}
                  </p>
                  {status === "Draft" && draftEmptyAction}
                </div>
              ) : (
                <div className="space-y-3">
                  {column.map((trip) => (
                    <TripCard
                      key={trip.id}
                      trip={trip}
                      onOpen={() => onOpen(trip)}
                      actions={renderActions(trip)}
                    />
                  ))}
                </div>
              )}
            </section>
          )
        },
      )}
    </div>
  )
}
