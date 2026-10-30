"use client"

import { CalendarDays, Fuel, Route } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { ComboChart } from "@/components/ComboChart"
import { DateRangePicker, type DateRange } from "@/components/DateRangePicker"
import { Pagination } from "@/components/Pagination"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { CardShell } from "@/components/fleet/card-shell"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { hasRouteAccess } from "@/lib/rbacConfig"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import {
  EmptyNote,
  ErrorBlock,
  ForbiddenBlock,
  IconBadge,
  LoadingBlock,
  SectionHeading,
} from "../dashboard/_components/states"
import {
  formatDateTime,
  formatNumber,
  type ExpenseRow,
  type Trip,
} from "../trips/_components/types"

type TripExtra = Trip & {
  dispatched_at?: string | null
  completed_at?: string | null
  created_at?: string | null
  updated_at?: string | null
}

type VehicleRow = {
  registration_number: string
  model: string
  type: string
  status: string
}

const PAGE_SIZE = 10

function asDate(value: unknown): string | null {
  if (typeof value !== "string" || !value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : value
}

function tripStart(trip: TripExtra): string | null {
  return (
    asDate(trip.dispatched_at) ?? asDate(trip.created_at) ?? asDate(trip.updated_at)
  )
}

function tripEnd(trip: TripExtra): string | null {
  return asDate(trip.completed_at) ?? asDate(trip.updated_at)
}

function tripDistance(trip: TripExtra): number | null {
  if (
    trip.start_odometer != null &&
    trip.final_odometer != null &&
    trip.final_odometer >= trip.start_odometer
  ) {
    return trip.final_odometer - trip.start_odometer
  }
  return typeof trip.planned_distance === "number" ? trip.planned_distance : null
}

function dayKey(value: string): string {
  return value.slice(0, 10)
}

function formatDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime()
  if (Number.isNaN(ms) || ms < 0) return "Not tracked"
  const hours = Math.floor(ms / 3_600_000)
  const minutes = Math.round((ms % 3_600_000) / 60_000)
  if (hours === 0) return `${minutes} min`
  return `${hours} h ${minutes} min`
}

function isFuelExpense(expense: ExpenseRow): boolean {
  return expense.type.toLowerCase().includes("fuel")
}

function SummaryTile({
  label,
  value,
  hint,
}: {
  label: string
  value: string
  hint?: string
}) {
  return (
    <div className="rounded-2xl border border-border bg-card px-4 py-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-xl font-semibold tabular-nums text-foreground">
        {value}
      </p>
      {hint ? (
        <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  )
}

export default function ReportsPage() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role ?? ""
  const allowed = !isLoaded || hasRouteAccess(role, "analytics")

  const [trips, setTrips] = useState<TripExtra[]>([])
  const [expenses, setExpenses] = useState<ExpenseRow[]>([])
  const [vehicles, setVehicles] = useState<VehicleRow[]>([])
  const [state, setState] = useState<"loading" | "ready" | "error">("loading")
  const [error, setError] = useState<string | null>(null)
  const [range, setRange] = useState<DateRange>({ from: null, to: null })
  const [page, setPage] = useState(1)

  const load = useCallback(() => {
    if (!isLoaded || !currentProfile) return
    if (!hasRouteAccess(currentProfile.role, "analytics")) {
      setState("ready")
      return
    }
    setState("loading")
    setError(null)
    Promise.all([
      apiRequest<unknown>("/trips/"),
      apiRequest<unknown>("/expenses"),
      apiRequest<unknown>("/fleet/vehicles"),
    ])
      .then(([tripData, expenseData, vehicleData]) => {
        setTrips(extractItems<TripExtra>(tripData))
        setExpenses(extractItems<ExpenseRow>(expenseData))
        setVehicles(extractItems<VehicleRow>(vehicleData))
        setState("ready")
      })
      .catch((err) => {
        setError(
          err instanceof ApiError ? err.detail : "Failed to load the daily report",
        )
        setState("error")
      })
  }, [isLoaded, currentProfile])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    setPage(1)
  }, [range])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: load,
    onReconnect: load,
  })

  const inRange = useCallback(
    (value: string | null) => {
      if (!value) return range.from == null && range.to == null
      const day = dayKey(value)
      if (range.from && day < range.from) return false
      if (range.to && day > range.to) return false
      return true
    },
    [range],
  )

  const rangedTrips = useMemo(
    () => trips.filter((trip) => inRange(tripStart(trip))),
    [trips, inRange],
  )

  const rangedExpenses = useMemo(
    () => expenses.filter((expense) => inRange(expense.date ?? null)),
    [expenses, inRange],
  )

  const fuelByTrip = useMemo(() => {
    const map = new Map<number, number>()
    for (const expense of rangedExpenses) {
      if (expense.trip_id == null || !isFuelExpense(expense)) continue
      if (expense.liters == null) continue
      map.set(expense.trip_id, (map.get(expense.trip_id) ?? 0) + expense.liters)
    }
    return map
  }, [rangedExpenses])

  const runnings = useMemo(
    () => rangedTrips.filter((trip) => trip.status === "Dispatched").length,
    [rangedTrips],
  )

  const totalKm = useMemo(
    () =>
      rangedTrips.reduce(
        (sum, trip) => sum + (tripDistance(trip) ?? 0),
        0,
      ),
    [rangedTrips],
  )

  const fuelLiters = useMemo(
    () =>
      rangedExpenses
        .filter(isFuelExpense)
        .reduce((sum, expense) => sum + (expense.liters ?? 0), 0),
    [rangedExpenses],
  )

  const mileage =
    fuelLiters > 0 ? Number((totalKm / fuelLiters).toFixed(2)) : null

  const dailyDistance = useMemo(() => {
    const map = new Map<string, number>()
    for (const trip of rangedTrips) {
      const start = tripStart(trip)
      if (!start) continue
      const key = dayKey(start)
      map.set(key, (map.get(key) ?? 0) + (tripDistance(trip) ?? 0))
    }
    return Array.from(map.entries())
      .sort(([a], [b]) => (a < b ? -1 : 1))
      .map(([day, distance]) => ({ day, distance: Number(distance.toFixed(1)) }))
  }, [rangedTrips])

  const dailyFuel = useMemo(() => {
    const map = new Map<string, number>()
    for (const expense of rangedExpenses) {
      if (!isFuelExpense(expense) || expense.liters == null || !expense.date)
        continue
      const key = dayKey(expense.date)
      map.set(key, (map.get(key) ?? 0) + expense.liters)
    }
    return Array.from(map.entries())
      .sort(([a], [b]) => (a < b ? -1 : 1))
      .map(([day, fuel]) => ({ day, fuel: Number(fuel.toFixed(1)) }))
  }, [rangedExpenses])

  const history = useMemo(
    () =>
      [...rangedTrips].sort((a, b) => {
        const aStart = tripStart(a) ?? ""
        const bStart = tripStart(b) ?? ""
        return bStart.localeCompare(aStart)
      }),
    [rangedTrips],
  )

  const totalPages = Math.max(1, Math.ceil(history.length / PAGE_SIZE))
  const safePage = Math.min(page, totalPages)
  const pageRows = history.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)

  if (!isLoaded || state === "loading") {
    return (
      <div className="space-y-6 p-6 lg:p-8" aria-busy="true">
        <LoadingBlock label="Loading daily trip report" />
      </div>
    )
  }

  if (!currentProfile) {
    return (
      <div className="p-6 lg:p-8">
        <ForbiddenBlock message="Sign in to view the daily trip report." />
      </div>
    )
  }

  if (!allowed) {
    return (
      <div className="p-6 lg:p-8">
        <ForbiddenBlock message="Your role does not have access to reports." />
      </div>
    )
  }

  if (state === "error") {
    return (
      <div className="p-6 lg:p-8">
        <ErrorBlock message={error ?? "Failed to load the daily report"} onRetry={load} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="flex items-center gap-3">
            <p className="text-sm text-muted-foreground">Daily trip report</p>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">Reports</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Derived from trips, expenses and fleet data. {vehicles.length}{" "}
            {vehicles.length === 1 ? "vehicle" : "vehicles"} on record.
          </p>
        </div>
        <DateRangePicker value={range} onChange={setRange} id="report-range" />
      </div>

      <CardShell
        as="section"
        aria-label="Overall summary"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <SectionHeading
          icon={Route}
          title="Overall summary"
          description="Trips in the selected range."
        />
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <SummaryTile label="Runnings" value={String(runnings)} hint="Dispatched trips" />
          <SummaryTile label="Distance" value={`${formatNumber(Math.round(totalKm))} km`} />
          <SummaryTile label="Alarms" value="Not tracked" hint="No alarms endpoint" />
          <SummaryTile label="Avg speed" value="Not tracked" hint="No telemetry endpoint" />
        </div>
      </CardShell>

      <CardShell
        as="section"
        aria-label="Fuel summary"
        tone="sage"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <SectionHeading
          icon={Fuel}
          title="Fuel summary"
          description="From logged fuel expenses in the selected range."
        />
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <SummaryTile label="Fuel used" value={`${formatNumber(Number(fuelLiters.toFixed(1)))} L`} />
          <SummaryTile
            label="Mileage"
            value={mileage != null ? `${mileage} km/L` : "Not tracked"}
            hint={mileage != null ? undefined : "Needs distance and fuel readings"}
          />
          <SummaryTile label="Filling alarms" value="Not tracked" hint="No alarms endpoint" />
          <SummaryTile label="Theft alarms" value="Not tracked" hint="No alarms endpoint" />
        </div>
        <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div>
            <h3 className="text-sm font-medium text-foreground">Distance per day</h3>
            {dailyDistance.length > 0 ? (
              <ComboChart
                className="mt-2 h-64"
                data={dailyDistance}
                index="day"
                barSeries={{ categories: [], showYAxis: false }}
                lineSeries={{ categories: ["distance"] }}
                showLegend={false}
                enableBiaxial={false}
              />
            ) : (
              <EmptyNote message="No dated trips in the selected range." />
            )}
          </div>
          <div>
            <h3 className="text-sm font-medium text-foreground">Fuel used per day</h3>
            {dailyFuel.length > 0 ? (
              <ComboChart
                className="mt-2 h-64"
                data={dailyFuel}
                index="day"
                barSeries={{ categories: [], showYAxis: false }}
                lineSeries={{ categories: ["fuel"] }}
                showLegend={false}
                enableBiaxial={false}
              />
            ) : (
              <EmptyNote message="No fuel expenses in the selected range." />
            )}
          </div>
        </div>
      </CardShell>

      <CardShell
        as="section"
        aria-label="Trip history"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <SectionHeading
          icon={CalendarDays}
          title="Trip history"
          description={`${history.length} ${history.length === 1 ? "trip" : "trips"} in the selected range.`}
        />
        {history.length === 0 ? (
          <EmptyNote message="No trips in the selected range. Adjust the dates to see history." />
        ) : (
          <>
            <div className="overflow-x-auto">
              <Table aria-label="Daily trip history">
                <TableHead>
                  <TableRow>
                    <TableHeaderCell scope="col">Vehicle</TableHeaderCell>
                    <TableHeaderCell scope="col">Start</TableHeaderCell>
                    <TableHeaderCell scope="col">End</TableHeaderCell>
                    <TableHeaderCell scope="col">From</TableHeaderCell>
                    <TableHeaderCell scope="col">To</TableHeaderCell>
                    <TableHeaderCell scope="col">Distance</TableHeaderCell>
                    <TableHeaderCell scope="col">Duration</TableHeaderCell>
                    <TableHeaderCell scope="col">Mileage</TableHeaderCell>
                    <TableHeaderCell scope="col">Fuel</TableHeaderCell>
                    <TableHeaderCell scope="col">Alarms</TableHeaderCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {pageRows.map((trip) => {
                    const start = tripStart(trip)
                    const end = tripEnd(trip)
                    const distance = tripDistance(trip)
                    const fuel =
                      (trip.id != null ? fuelByTrip.get(trip.id) : undefined) ??
                      trip.fuel_liters ??
                      null
                    const tripMileage =
                      distance != null && fuel != null && fuel > 0
                        ? Number((distance / fuel).toFixed(2))
                        : null
                    return (
                      <TableRow key={trip.id}>
                        <TableCell className="font-medium text-foreground">
                          {trip.vehicle_reg}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {start ? formatDateTime(start) : "Not tracked"}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {end ? formatDateTime(end) : "Not tracked"}
                        </TableCell>
                        <TableCell>{trip.source_name}</TableCell>
                        <TableCell>{trip.destination_name}</TableCell>
                        <TableCell className="tabular-nums">
                          {distance != null ? `${formatNumber(Math.round(distance))} km` : "Not tracked"}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {start && end ? formatDuration(start, end) : "Not tracked"}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {tripMileage != null ? `${tripMileage} km/L` : "Not tracked"}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {fuel != null ? `${formatNumber(Number(fuel.toFixed(1)))} L` : "Not tracked"}
                        </TableCell>
                        <TableCell>Not tracked</TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
            <Pagination page={safePage} pageSize={PAGE_SIZE} total={history.length} onPageChange={setPage} />
          </>
        )}
      </CardShell>

      <p className="flex items-center gap-2 text-xs text-muted-foreground">
        <IconBadge icon={CalendarDays} className="size-8" />
        No dedicated report endpoint yet. This page derives every figure from
        trips, expenses and fleet data.
      </p>
    </div>
  )
}
