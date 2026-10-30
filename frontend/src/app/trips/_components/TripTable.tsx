"use client"

import type { ReactNode } from "react"
import { RiInboxLine } from "@remixicon/react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
import { Skeleton } from "@/components/Skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { CardHead } from "./chrome"
import { StatusBadge } from "./StatusBadge"
import { formatDateTime, type Trip } from "./types"

interface TripTableProps {
  trips: Trip[]
  loading: boolean
  onOpen: (trip: Trip) => void
  renderActions: (trip: Trip) => ReactNode
  emptyAction: ReactNode
}

type TripDates = Trip & {
  dispatched_at?: string | null
  created_at?: string | null
  eta?: string | null
  expected_delivery?: string | null
  expected_at?: string | null
  planned_date?: string | null
  due_date?: string | null
}

function pickupValue(trip: Trip): string {
  const dates = trip as TripDates
  const raw =
    dates.dispatched_at ?? dates.created_at ?? trip.created_at ?? null
  return raw ? formatDateTime(raw) : "Not tracked"
}

function expectedValue(trip: Trip): string {
  const dates = trip as TripDates
  const raw =
    dates.eta ??
    dates.expected_delivery ??
    dates.expected_at ??
    dates.planned_date ??
    dates.due_date ??
    null
  return raw ? formatDateTime(raw) : "Not tracked"
}

export function TripTable({
  trips,
  loading,
  onOpen,
  renderActions,
  emptyAction,
}: TripTableProps) {
  return (
    <CardShell as="section" aria-label="Trip register">
      <CardHead
        title="Trip register"
        meta={loading ? "Loading…" : `${trips.length} ${trips.length === 1 ? "trip" : "trips"}`}
      />
      <div className="mt-4 overflow-x-auto">
        <Table aria-label="Trip register">
          <TableHead>
            <TableRow>
              <TableHeaderCell scope="col">Trip Number</TableHeaderCell>
              <TableHeaderCell scope="col">Origin</TableHeaderCell>
              <TableHeaderCell scope="col">Destination</TableHeaderCell>
              <TableHeaderCell scope="col">Vehicle</TableHeaderCell>
              <TableHeaderCell scope="col">Driver</TableHeaderCell>
              <TableHeaderCell scope="col">Pickup</TableHeaderCell>
              <TableHeaderCell scope="col">Expected delivery</TableHeaderCell>
              <TableHeaderCell scope="col">Status</TableHeaderCell>
              <TableHeaderCell scope="col">
                <span className="sr-only">Row actions</span>
                View
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              [0, 1, 2, 3, 4, 5].map((key) => (
                <TableRow key={key}>
                  <TableCell colSpan={9}>
                    <Skeleton
                      className="h-5 w-full motion-reduce:animate-none"
                      aria-label="Loading trips"
                    />
                  </TableCell>
                </TableRow>
              ))
            ) : trips.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9}>
                  <div className="flex flex-col items-center gap-2 py-10 text-center">
                    <RiInboxLine
                      className="size-8 text-muted-foreground"
                      aria-hidden="true"
                    />
                    <p className="text-sm font-medium text-foreground">
                      No trips match these filters
                    </p>
                    <p className="max-w-sm text-sm text-muted-foreground">
                      Try a different search or status, or create a trip to get
                      started.
                    </p>
                    {emptyAction}
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              trips.map((trip) => (
                <TableRow key={trip.id}>
                  <TableCell>
                    <button
                      type="button"
                      onClick={() => onOpen(trip)}
                      className="rounded font-mono font-semibold tabular-nums text-foreground underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                      aria-label={`Open details for trip ${trip.id}`}
                    >
                      #{trip.id}
                    </button>
                  </TableCell>
                  <TableCell className="font-serif">{trip.source_name}</TableCell>
                  <TableCell className="font-serif">
                    {trip.destination_name}
                  </TableCell>
                  <TableCell className="font-serif">
                    {trip.vehicle_reg}
                  </TableCell>
                  <TableCell className="font-mono tabular-nums">
                    {trip.driver_license}
                  </TableCell>
                  <TableCell className="tabular-nums">
                    {pickupValue(trip)}
                  </TableCell>
                  <TableCell className="tabular-nums">
                    {expectedValue(trip)}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={trip.status} />
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => onOpen(trip)}
                        aria-label={`View trip ${trip.id}`}
                      >
                        View
                      </Button>
                      {renderActions(trip)}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </CardShell>
  )
}
