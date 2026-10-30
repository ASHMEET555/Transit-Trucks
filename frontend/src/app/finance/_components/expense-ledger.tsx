"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { Fuel, Gauge, Receipt, Wrench } from "lucide-react"
import { RiMoneyDollarCircleLine, RiReceiptLine, RiTruckLine } from "@remixicon/react"
import { Badge } from "@/components/Badge"
import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
import { CardShell } from "@/components/fleet/card-shell"
import { DateRangePicker } from "@/components/DateRangePicker"
import { Pagination } from "@/components/Pagination"
import { StatPills } from "@/components/live-map/stat-pills"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { Skeleton } from "@/components/Skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { BACKEND_URL } from "@/data/data"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { canWrite, hasRouteAccess } from "@/lib/rbacConfig"
import { CardHead, SectionIcon } from "./chrome"

type Trip = {
  id: number
  source_name: string
  destination_name: string
  status: string
}

type Expense = {
  id: number
  trip_id: number
  type: string
  liters?: number | null
  cost: number
  date: string
}

type VehicleFinanceSummary = {
  vehicle_reg: string
  fuel_expenses: number
  maintenance_expenses: number
  other_expenses: number
  closed_maintenance_cost: number
  total_distance: number
  cost_per_km: number | null
}

type FinanceSummary = {
  from: string | null
  to: string | null
  vehicles: VehicleFinanceSummary[]
  totals: VehicleFinanceSummary | null
}

const EXPENSE_TYPES = ["Fuel", "Toll", "Maintenance", "Other"]

function formatMoney(value: number): string {
  return `$${Number(value).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function toQuery(params: Record<string, string>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value.trim() !== "") search.set(key, value.trim())
  }
  const query = search.toString()
  return query ? `?${query}` : ""
}

export function ExpenseLedger() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role ?? ""
  const hasAccess = !isLoaded || hasRouteAccess(role, "fuelAndExpenses")
  const canManage = canWrite(role, "fuelAndExpenses")

  const [trips, setTrips] = useState<Trip[]>([])
  const [tripsError, setTripsError] = useState<string | null>(null)

  const [tripFilter, setTripFilter] = useState("all")
  const [typeFilter, setTypeFilter] = useState("all")
  const [vehicleFilter, setVehicleFilter] = useState("")
  const [range, setRange] = useState<{ from: string | null; to: string | null }>({
    from: null,
    to: null,
  })
  const fromFilter = range.from ?? ""
  const toFilter = range.to ?? ""

  const LEDGER_PAGE_SIZE = 10
  const SUMMARY_PAGE_SIZE = 10
  const [ledgerPage, setLedgerPage] = useState(1)
  const [summaryPage, setSummaryPage] = useState(1)

  useEffect(() => {
    setLedgerPage(1)
  }, [tripFilter, typeFilter, vehicleFilter, range])

  useEffect(() => {
    setSummaryPage(1)
  }, [vehicleFilter, range])

  const [expenses, setExpenses] = useState<Expense[]>([])
  const [ledgerLoading, setLedgerLoading] = useState(true)
  const [ledgerError, setLedgerError] = useState<string | null>(null)

  const [summary, setSummary] = useState<FinanceSummary | null>(null)
  const [summaryLoading, setSummaryLoading] = useState(true)
  const [summaryBlocked, setSummaryBlocked] = useState(false)

  const [exporting, setExporting] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)

  const [formTripId, setFormTripId] = useState("")
  const [formType, setFormType] = useState("Fuel")
  const [formLiters, setFormLiters] = useState("")
  const [formCost, setFormCost] = useState("")
  const [formDate, setFormDate] = useState(() => new Date().toISOString().slice(0, 10))
  const [formError, setFormError] = useState<string | null>(null)
  const [formSaving, setFormSaving] = useState(false)

  useEffect(() => {
    if (!isLoaded || !hasRouteAccess(role, "fuelAndExpenses")) return
    apiRequest<unknown>("/trips/")
      .then((res) => {
        const eligible = extractItems<Trip>(res).filter(
          (trip) => trip.status === "Dispatched" || trip.status === "Completed",
        )
        setTrips(eligible)
        setTripsError(null)
      })
      .catch((err) =>
        setTripsError(err instanceof ApiError ? err.detail : "Could not load trips"),
      )
  }, [isLoaded, role])

  const loadLedger = useCallback(async () => {
    setLedgerLoading(true)
    setLedgerError(null)
    try {
      const res = await apiRequest<unknown>(
        `/expenses${toQuery({
          trip_id: tripFilter === "all" ? "" : tripFilter,
          type: typeFilter === "all" ? "" : typeFilter,
          vehicle_reg: vehicleFilter,
          from: fromFilter,
          to: toFilter,
        })}`,
      )
      setExpenses(extractItems<Expense>(res))
    } catch (err) {
      setLedgerError(err instanceof ApiError ? err.detail : "Could not load expenses")
    } finally {
      setLedgerLoading(false)
    }
  }, [tripFilter, typeFilter, vehicleFilter, fromFilter, toFilter])

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true)
    setSummaryBlocked(false)
    try {
      const res = await apiRequest<FinanceSummary>(
        `/finance/summary${toQuery({
          vehicle_reg: vehicleFilter,
          from: fromFilter,
          to: toFilter,
        })}`,
      )
      setSummary(res)
    } catch (err) {
      if (err instanceof ApiError && (err.status === 403 || err.status === 401)) {
        setSummary(null)
        setSummaryBlocked(true)
      } else {
        setSummary(null)
        setSummaryBlocked(false)
      }
    } finally {
      setSummaryLoading(false)
    }
  }, [vehicleFilter, fromFilter, toFilter])

  const reload = useCallback(() => {
    void loadLedger()
    void loadSummary()
  }, [loadLedger, loadSummary])

  useEffect(() => {
    if (!isLoaded || !hasRouteAccess(role, "fuelAndExpenses")) {
      setLedgerLoading(false)
      setSummaryLoading(false)
      return
    }
    reload()
  }, [isLoaded, role, reload])

  const { status: realtimeStatus } = useFleetEvents({
    enabled: isLoaded && hasRouteAccess(role, "fuelAndExpenses"),
    onEvent: (event) => {
      if (event.type.startsWith("expense")) reload()
    },
    onReconnect: () => reload(),
  })

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!canManage || !formTripId) return
    setFormSaving(true)
    setFormError(null)
    try {
      await apiRequest("/expenses", {
        method: "POST",
        body: {
          trip_id: Number(formTripId),
          type: formType,
          liters: formType === "Fuel" ? Number(formLiters) : null,
          cost: Number(formCost),
          date: formDate,
        },
      })
      setFormType("Fuel")
      setFormLiters("")
      setFormCost("")
      setFormDate(new Date().toISOString().slice(0, 10))
      reload()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.detail : "Could not log expense")
    } finally {
      setFormSaving(false)
    }
  }

  async function handleExport() {
    setExporting(true)
    setExportError(null)
    try {
      const res = await fetch(
        `${BACKEND_URL}/dashboard/export.csv${toQuery({
          vehicle_reg: vehicleFilter,
          from: fromFilter,
          to: toFilter,
        })}`,
        { credentials: "include" },
      )
      if (!res.ok) throw new Error(`Export failed (${res.status})`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = "expenses.csv"
      link.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setExportError(err instanceof Error ? err.message : "Export failed")
    } finally {
      setExporting(false)
    }
  }

  const totals = summary?.totals ?? null

  const pagedExpenses = useMemo(() => {
    const start = (ledgerPage - 1) * LEDGER_PAGE_SIZE
    return expenses.slice(start, start + LEDGER_PAGE_SIZE)
  }, [expenses, ledgerPage])

  const summaryVehicles = useMemo(() => summary?.vehicles ?? [], [summary])
  const pagedSummaryVehicles = useMemo(() => {
    const start = (summaryPage - 1) * SUMMARY_PAGE_SIZE
    return summaryVehicles.slice(start, start + SUMMARY_PAGE_SIZE)
  }, [summaryVehicles, summaryPage])

  if (isLoaded && !hasAccess) {
    return (
      <div className="space-y-4 p-6">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Fuel and expenses</h1>
        <CardShell as="section" className="items-center p-8 text-center">
          <p className="font-medium text-foreground">Access restricted</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Your role does not have access to the expense ledger.
          </p>
        </CardShell>
      </div>
    )
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Fuel and expenses</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Trip-linked fuel, tolls, and other costs
          </p>
        </div>
        <div className="flex items-center gap-2">
          <RealtimeIndicator
            status={realtimeStatus}
            className="[&_.animate-ping]:motion-reduce:animate-none [&_.animate-pulse]:motion-reduce:animate-none"
          />
          <Button
            variant="secondary"
            onClick={handleExport}
            disabled={exporting || ledgerLoading}
          >
            {exporting ? "Exporting…" : "Export csv"}
          </Button>
        </div>
      </div>

      {exportError && (
        <div role="alert" className="rounded-2xl border border-destructive/30 bg-destructive/12 p-3 text-sm text-foreground">
          {exportError}
        </div>
      )}

      <section
        aria-label="Filters"
        className="grid grid-cols-1 gap-4 rounded-2xl border border-border bg-muted p-3 sm:p-4 lg:grid-cols-5"
      >
        <div>
          <label
            htmlFor="expense-trip-filter"
            className="mb-1 block text-sm font-medium text-foreground"
          >
            Trip
          </label>
          <Select value={tripFilter} onValueChange={setTripFilter}>
            <SelectTrigger id="expense-trip-filter" className="w-full">
              <SelectValue placeholder="All trips" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All trips</SelectItem>
              {trips.map((trip) => (
                <SelectItem key={trip.id} value={String(trip.id)}>
                  #{trip.id} · {trip.source_name} → {trip.destination_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label
            htmlFor="expense-type-filter"
            className="mb-1 block text-sm font-medium text-foreground"
          >
            Type
          </label>
          <Select value={typeFilter} onValueChange={setTypeFilter}>
            <SelectTrigger id="expense-type-filter" className="w-full">
              <SelectValue placeholder="All types" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All types</SelectItem>
              {EXPENSE_TYPES.map((type) => (
                <SelectItem key={type} value={type}>
                  {type}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label
            htmlFor="expense-vehicle-filter"
            className="mb-1 block text-sm font-medium text-foreground"
          >
            Vehicle
          </label>
          <Input
            id="expense-vehicle-filter"
            type="search"
            placeholder="Registration number"
            value={vehicleFilter}
            onChange={(event) => setVehicleFilter(event.target.value)}
          />
        </div>
        <div className="lg:col-span-2">
          <span className="mb-1 block text-sm font-medium text-foreground">
            Date range
          </span>
          <DateRangePicker
            value={range}
            onChange={setRange}
            id="expense-date-range"
          />
        </div>
      </section>
      {tripsError && (
        <p role="alert" className="text-sm text-muted-foreground">
          Trip list unavailable: {tripsError}
        </p>
      )}

      <section aria-label="Totals">
        {summaryLoading ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" aria-label="Loading totals">
            <Skeleton className="h-24 rounded-2xl motion-reduce:animate-none" />
            <Skeleton className="h-24 rounded-2xl motion-reduce:animate-none" />
            <Skeleton className="h-24 rounded-2xl motion-reduce:animate-none" />
            <Skeleton className="h-24 rounded-2xl motion-reduce:animate-none" />
          </div>
        ) : (
          <StatPills
            items={[
              { label: "Fuel", value: totals?.fuel_expenses != null ? formatMoney(totals.fuel_expenses) : "Not tracked", icon: Fuel, tone: "sage" },
              { label: "Maintenance", value: totals?.maintenance_expenses != null ? formatMoney(totals.maintenance_expenses) : "Not tracked", icon: Wrench, tone: "primary" },
              { label: "Other", value: totals?.other_expenses != null ? formatMoney(totals.other_expenses) : "Not tracked", icon: Receipt, tone: "mist" },
              { label: "Cost per km", value: totals?.cost_per_km != null ? formatMoney(totals.cost_per_km) : "Not tracked", icon: Gauge, tone: "signal" },
            ]}
          />
        )}
      </section>
      {summaryBlocked && (
        <p className="text-sm text-muted-foreground">
          Summary totals are not tracked for your role.
        </p>
      )}

      <CardShell as="section" aria-label="Per-vehicle cost summary">
        <CardHead
          title="Per-vehicle cost summary"
          action={<SectionIcon icon={RiTruckLine} />}
        />
        <div className="mt-4">
          {summaryLoading ? (
            <div className="space-y-2" aria-label="Loading summary">
              <Skeleton className="h-10 rounded-2xl motion-reduce:animate-none" />
              <Skeleton className="h-10 rounded-2xl motion-reduce:animate-none" />
              <Skeleton className="h-10 rounded-2xl motion-reduce:animate-none" />
            </div>
          ) : summaryBlocked ? (
            <p className="text-sm text-muted-foreground">Not tracked</p>
          ) : !summary || summaryVehicles.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No summary data for the selected filters.
            </p>
          ) : (
            <>
            <div className="overflow-x-auto">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableHeaderCell scope="col">Vehicle</TableHeaderCell>
                    <TableHeaderCell scope="col">Fuel</TableHeaderCell>
                    <TableHeaderCell scope="col">Maintenance</TableHeaderCell>
                    <TableHeaderCell scope="col">Other</TableHeaderCell>
                    <TableHeaderCell scope="col">Distance (km)</TableHeaderCell>
                    <TableHeaderCell scope="col">Cost per km</TableHeaderCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {pagedSummaryVehicles.map((vehicle) => (
                    <TableRow key={vehicle.vehicle_reg}>
                      <TableCell className="font-serif text-foreground">
                        {vehicle.vehicle_reg}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {formatMoney(vehicle.fuel_expenses)}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {formatMoney(vehicle.maintenance_expenses)}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {formatMoney(vehicle.other_expenses)}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {vehicle.total_distance.toLocaleString()}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {vehicle.cost_per_km === null || vehicle.cost_per_km === undefined ? (
                          <span className="font-sans text-sm">Not tracked</span>
                        ) : (
                          formatMoney(vehicle.cost_per_km)
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pagination
              page={summaryPage}
              pageSize={SUMMARY_PAGE_SIZE}
              total={summaryVehicles.length}
              onPageChange={setSummaryPage}
              id="finance-summary-pagination"
            />
            </>
          )}
        </div>
      </CardShell>

      {canManage && (
        <CardShell as="section" aria-label="Log expense">
          <div className="flex items-center gap-3">
            <SectionIcon icon={RiMoneyDollarCircleLine} />
            <CardHead title="Log expense" />
          </div>
          <div className="mt-4">
            {formError && (
              <div role="alert" className="mb-4 rounded-2xl border border-destructive/30 bg-destructive/12 p-3 text-sm text-foreground">
                {formError}
              </div>
            )}
            <form
              onSubmit={handleSubmit}
              className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-5"
            >
              <div>
                <label
                  htmlFor="expense-form-trip"
                  className="mb-1 block text-sm font-medium text-foreground"
                >
                  Trip
                </label>
                <Select value={formTripId} onValueChange={setFormTripId} required>
                  <SelectTrigger id="expense-form-trip" className="w-full">
                    <SelectValue placeholder="Choose a trip" />
                  </SelectTrigger>
                  <SelectContent>
                    {trips.map((trip) => (
                      <SelectItem key={trip.id} value={String(trip.id)}>
                        #{trip.id} · {trip.source_name} → {trip.destination_name} ({trip.status})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="mt-1 text-xs text-muted-foreground">
                  Only dispatched or completed trips accept expenses.
                </p>
              </div>
              <div>
                <label
                  htmlFor="expense-form-type"
                  className="mb-1 block text-sm font-medium text-foreground"
                >
                  Type
                </label>
                <Select value={formType} onValueChange={setFormType}>
                  <SelectTrigger id="expense-form-type" className="w-full">
                    <SelectValue placeholder="Select type" />
                  </SelectTrigger>
                  <SelectContent>
                    {EXPENSE_TYPES.map((type) => (
                      <SelectItem key={type} value={type}>
                        {type}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {formType === "Fuel" && (
                <div>
                  <label
                    htmlFor="expense-form-liters"
                    className="mb-1 block text-sm font-medium text-foreground"
                  >
                    Liters
                  </label>
                  <Input
                    id="expense-form-liters"
                    type="number"
                    min="0"
                    step="0.01"
                    placeholder="e.g. 40"
                    value={formLiters}
                    onChange={(event) => setFormLiters(event.target.value)}
                    required
                  />
                </div>
              )}
              <div>
                <label
                  htmlFor="expense-form-cost"
                  className="mb-1 block text-sm font-medium text-foreground"
                >
                  Cost
                </label>
                <Input
                  id="expense-form-cost"
                  type="number"
                  min="0"
                  step="0.01"
                  placeholder="e.g. 80.00"
                  value={formCost}
                  onChange={(event) => setFormCost(event.target.value)}
                  required
                />
              </div>
              <div>
                <label
                  htmlFor="expense-form-date"
                  className="mb-1 block text-sm font-medium text-foreground"
                >
                  Date
                </label>
                <Input
                  id="expense-form-date"
                  type="date"
                  value={formDate}
                  onChange={(event) => setFormDate(event.target.value)}
                  required
                />
              </div>
              <div className="flex items-end md:col-span-2 lg:col-span-5">
                <Button type="submit" isLoading={formSaving} loadingText="Logging expense">
                  Log expense
                </Button>
              </div>
            </form>
          </div>
        </CardShell>
      )}

      <CardShell as="section" aria-label="Expense ledger">
        <CardHead
          title="Expense ledger"
          meta={ledgerLoading ? "Loading…" : `${expenses.length} ${expenses.length === 1 ? "entry" : "entries"}`}
          action={
            ledgerLoading ? (
              <Badge variant="neutral" aria-label="Loading expenses">
                Loading…
              </Badge>
            ) : (
              <SectionIcon icon={RiReceiptLine} />
            )
          }
        />
        <div className="mt-4">
          {ledgerError ? (
            <div role="alert" className="rounded-2xl border border-destructive/30 bg-destructive/12 p-4 text-center">
              <p className="font-medium text-foreground">Could not load expenses</p>
              <p className="mt-1 text-sm text-muted-foreground">{ledgerError}</p>
              <Button variant="secondary" className="mt-3" onClick={reload}>
                Try again
              </Button>
            </div>
          ) : ledgerLoading ? (
            <div className="space-y-2" aria-label="Loading expenses">
              <Skeleton className="h-10 rounded-2xl motion-reduce:animate-none" />
              <Skeleton className="h-10 rounded-2xl motion-reduce:animate-none" />
              <Skeleton className="h-10 rounded-2xl motion-reduce:animate-none" />
            </div>
          ) : expenses.length === 0 ? (
            <div className="pb-2 text-center">
              <p className="font-medium text-foreground">No expenses yet</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {canManage
                  ? "Log the first expense for a dispatched or completed trip."
                  : "No expenses match the selected filters."}
              </p>
            </div>
          ) : (
            <>
            <div className="overflow-x-auto">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableHeaderCell scope="col">Id</TableHeaderCell>
                    <TableHeaderCell scope="col">Trip</TableHeaderCell>
                    <TableHeaderCell scope="col">Type</TableHeaderCell>
                    <TableHeaderCell scope="col">Liters</TableHeaderCell>
                    <TableHeaderCell scope="col">Cost</TableHeaderCell>
                    <TableHeaderCell scope="col">Date</TableHeaderCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {pagedExpenses.map((expense) => (
                    <TableRow key={expense.id}>
                      <TableCell className="font-mono text-xs">#{expense.id}</TableCell>
                      <TableCell className="font-mono text-xs">
                        Trip #{expense.trip_id}
                      </TableCell>
                      <TableCell>{expense.type}</TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {expense.liters === null || expense.liters === undefined ? (
                          <span className="font-sans text-sm">Not tracked</span>
                        ) : (
                          Number(expense.liters).toLocaleString()
                        )}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">
                        {formatMoney(expense.cost)}
                      </TableCell>
                      <TableCell className="font-mono tabular-nums">{expense.date}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pagination
              page={ledgerPage}
              pageSize={LEDGER_PAGE_SIZE}
              total={expenses.length}
              onPageChange={setLedgerPage}
              id="finance-ledger-pagination"
            />
            </>
          )}
        </div>
      </CardShell>
    </div>
  )
}
