"use client"

import Link from "next/link"
import {
  CircleCheck,
  FileText,
  Gauge,
  ReceiptText,
  Route,
} from "lucide-react"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { CardShell } from "@/components/fleet/card-shell"
import { StatPills } from "@/components/live-map/stat-pills"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import type { RealtimeStatus } from "@/lib/realtime/useFleetEvents"
import { EmptyNote, IconBadge } from "./states"

export type DriverDashboard = {
  driver_license: string
  trips_by_status: Record<string, number>
  total_completed_trips: number
  total_distance: number
  recent_trips: Array<{
    id: number
    source_name: string
    destination_name: string
    cargo_weight: number
    planned_distance: number
    distance: number
    completed_at: string
  }>
}

export function DriverHome({
  data,
  realtimeStatus,
}: {
  data: DriverDashboard
  realtimeStatus: RealtimeStatus
}) {
  const byStatus = data.trips_by_status ?? {}

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="flex items-center gap-3">
            <p className="text-sm text-muted-foreground">Driver portal</p>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">
            Driver dashboard
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Overview for license {data.driver_license}
          </p>
        </div>
        <Link
          href="/trips"
          className="inline-flex h-9 items-center rounded-lg border border-border bg-card px-4 text-sm font-medium hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          View my trips
        </Link>
      </div>

      <section aria-label="Key figures">
        <StatPills
          items={[
            {
              label: "Dispatched trips",
              value: byStatus["Dispatched"] ?? 0,
              icon: Route,
              tone: "primary",
            },
            {
              label: "Completed trips",
              value: data.total_completed_trips,
              icon: CircleCheck,
              tone: "sage",
            },
            {
              label: "Total distance",
              value: `${Number(data.total_distance).toLocaleString()} km`,
              icon: Gauge,
              tone: "signal",
            },
            {
              label: "Draft trips",
              value: byStatus["Draft"] ?? 0,
              icon: FileText,
              tone: "mist",
            },
          ]}
        />
      </section>

      <CardShell
        as="section"
        aria-label="Recent completed trips"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <div className="flex items-center gap-3">
          <IconBadge icon={ReceiptText} />
          <div>
            <h2 className="text-[15px] font-medium tracking-tight">
              Recent completed trips
            </h2>
            <p className="mt-0.5 text-sm text-muted-foreground">
              Your most recent finished work.
            </p>
          </div>
        </div>
        {data.recent_trips.length > 0 ? (
          <div className="mt-4 overflow-x-auto">
            <Table>
              <TableHead>
                <TableRow>
                  <TableHeaderCell>Trip</TableHeaderCell>
                  <TableHeaderCell>Route</TableHeaderCell>
                  <TableHeaderCell>Cargo</TableHeaderCell>
                  <TableHeaderCell>Distance</TableHeaderCell>
                  <TableHeaderCell>Completed</TableHeaderCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {data.recent_trips.map((trip) => (
                  <TableRow key={trip.id}>
                    <TableCell>#{trip.id}</TableCell>
                    <TableCell>
                      {trip.source_name} → {trip.destination_name}
                    </TableCell>
                    <TableCell>{trip.cargo_weight.toFixed(1)} kg</TableCell>
                    <TableCell>{trip.distance.toFixed(1)} km</TableCell>
                    <TableCell>
                      {new Date(trip.completed_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        ) : (
          <EmptyNote message="No completed trips yet." />
        )}
      </CardShell>
    </div>
  )
}
