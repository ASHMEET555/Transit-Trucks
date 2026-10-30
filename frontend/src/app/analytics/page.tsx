"use client"

import { Activity, Download, Fuel, TrendingUp } from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useMemo, useState } from "react"
import { BarChart } from "@/components/BarChart"
import { Button } from "@/components/Button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { CardShell } from "@/components/fleet/card-shell"
import { ApiError, apiRequest } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { hasRouteAccess } from "@/lib/rbacConfig"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import {
  downloadClientCsv,
  downloadDashboardCsv,
} from "../dashboard/_components/export"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import {
  EmptyNote,
  ErrorBlock,
  ForbiddenBlock,
  IconBadge,
  LoadingBlock,
  SectionHeading,
} from "../dashboard/_components/states"
import { ChartCard } from "./_components/chart-card"
import { UtilizationBars } from "./_components/utilization-bars"

type KpiVehicle = {
  registration_number: string
  model: string
  type: string
  status: string
  roi: number | null
  fuel_efficiency: number | null
  operational_cost: number
}

type DashboardKPIs = {
  fleet_utilization: number
  vehicles: KpiVehicle[]
}

function TypeFilter({
  id,
  value,
  types,
  onChange,
}: {
  id: string
  value: string
  types: string[]
  onChange: (value: string) => void
}) {
  return (
    <div className="flex items-center gap-2">
      <label htmlFor={id} className="text-sm text-muted-foreground">
        Vehicle type
      </label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className="w-44">
          <SelectValue placeholder="All types" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All types</SelectItem>
          {types.map((type) => (
            <SelectItem key={type} value={type}>
              {type}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

export default function AnalyticsPage() {
  const { currentProfile, isLoaded } = useProfile()
  const allowed = !isLoaded || hasRouteAccess(currentProfile?.role ?? "", "analytics")

  const [kpis, setKpis] = useState<DashboardKPIs | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)
  const [loading, setLoading] = useState(true)

  const [roiType, setRoiType] = useState("all")
  const [fuelType, setFuelType] = useState("all")

  const [from, setFrom] = useState("")
  const [to, setTo] = useState("")
  const [vehicleReg, setVehicleReg] = useState("")
  const [exporting, setExporting] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)
  const [selectedVehicle, setSelectedVehicle] = useState<string | null>(null)

  const load = useCallback(() => {
    if (!isLoaded || !currentProfile) return
    if (!hasRouteAccess(currentProfile.role, "analytics")) {
      setForbidden(true)
      setLoading(false)
      return
    }
    setForbidden(false)
    setLoading(true)
    setError(null)
    apiRequest<DashboardKPIs>("/dashboard/kpis")
      .then(setKpis)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 403) {
          setForbidden(true)
        } else {
          setError(
            err instanceof ApiError ? err.detail : "Failed to load analytics",
          )
        }
      })
      .finally(() => setLoading(false))
  }, [isLoaded, currentProfile])

  useEffect(() => {
    load()
  }, [load])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: load,
    onReconnect: load,
  })

  const vehicleTypes = useMemo(
    () =>
      Array.from(
        new Set((kpis?.vehicles ?? []).map((vehicle) => vehicle.type)),
      ).sort(),
    [kpis],
  )

  const roiRows = useMemo(() => {
    if (!kpis) return []
    return kpis.vehicles
      .filter((vehicle) => roiType === "all" || vehicle.type === roiType)
      .filter((vehicle) => vehicle.roi != null)
      .sort((a, b) => (b.roi ?? 0) - (a.roi ?? 0))
      .slice(0, 35)
      .map((vehicle) => ({
        vehicle: vehicle.registration_number,
        ROI: Number((((vehicle.roi ?? 0) * 100) as number).toFixed(1)),
      }))
  }, [kpis, roiType])

  const fuelRows = useMemo(() => {
    if (!kpis) return []
    return kpis.vehicles
      .filter((vehicle) => fuelType === "all" || vehicle.type === fuelType)
      .filter(
        (vehicle) =>
          vehicle.fuel_efficiency != null && vehicle.fuel_efficiency > 0,
      )
      .sort((a, b) => (b.fuel_efficiency ?? 0) - (a.fuel_efficiency ?? 0))
      .slice(0, 35)
      .map((vehicle) => ({
        vehicle: vehicle.registration_number,
        efficiency: Number((vehicle.fuel_efficiency ?? 0).toFixed(2)),
      }))
  }, [kpis, fuelType])

  const exportServerCsv = useCallback(async () => {
    setExporting(true)
    setExportError(null)
    try {
      await downloadDashboardCsv({
        from: from || undefined,
        to: to || undefined,
        vehicle_reg: vehicleReg || undefined,
      })
    } catch (err) {
      setExportError(err instanceof Error ? err.message : "Export failed")
    } finally {
      setExporting(false)
    }
  }, [from, to, vehicleReg])

  const handleChartSelect = useCallback((value: unknown) => {
    if (value && typeof value === "object" && "vehicle" in value) {
      const reg = (value as { vehicle?: unknown }).vehicle
      setSelectedVehicle(typeof reg === "string" && reg ? reg : null)
    } else {
      setSelectedVehicle(null)
    }
  }, [])

  const vehicleRegs = useMemo(
    () =>
      Array.from(
        new Set((kpis?.vehicles ?? []).map((vehicle) => vehicle.registration_number)),
      ).sort(),
    [kpis],
  )

  const vehicleRows = useMemo(() => {
    const rows = (kpis?.vehicles ?? []).map((vehicle) => ({
      reg: vehicle.registration_number,
      type: vehicle.type,
      status: vehicle.status,
      roi: vehicle.roi != null ? Number(((vehicle.roi * 100) as number).toFixed(1)) : null,
      efficiency:
        vehicle.fuel_efficiency != null
          ? Number(vehicle.fuel_efficiency.toFixed(2))
          : null,
      cost: vehicle.operational_cost,
    }))
    const filtered =
      selectedVehicle != null
        ? rows.filter((row) => row.reg === selectedVehicle)
        : rows
    return filtered.sort((a, b) => a.reg.localeCompare(b.reg))
  }, [kpis, selectedVehicle])

  if (!isLoaded || loading) {
    return (
      <div className="space-y-6 p-6 lg:p-8" aria-busy="true">
        <LoadingBlock label="Loading analytics" />
      </div>
    )
  }

  if (!allowed || forbidden) {
    return (
      <div className="p-6 lg:p-8">
        <ForbiddenBlock message="Your role does not have access to analytics." />
      </div>
    )
  }

  if (error && !kpis) {
    return (
      <div className="p-6 lg:p-8">
        <ErrorBlock message={error} onRetry={load} />
      </div>
    )
  }

  if (!kpis) {
    return (
      <div className="p-6 lg:p-8">
        <EmptyNote message="No analytics data available." />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="flex items-center gap-3">
            <p className="text-sm text-muted-foreground">Trends and exports</p>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">Analytics</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Longer-term trends and data exports. Live figures stay on the
            dashboard.
          </p>
        </div>
      </div>

      <UtilizationBars vehicles={kpis.vehicles} />

      <CardShell
        as="section"
        aria-label="Utilization trend"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <SectionHeading
          icon={Activity}
          title="Utilization trend"
          description="Fleet utilization over time."
        />
        <EmptyNote message="Not tracked — no utilization history endpoint is available, only the current snapshot." />
      </CardShell>

      <ChartCard
        icon={TrendingUp}
        title="Vehicle return on investment"
        description="Return per vehicle, highest first. Select a bar to filter the vehicle table below."
        filters={
          <TypeFilter
            id="roi-type"
            value={roiType}
            types={vehicleTypes}
            onChange={setRoiType}
          />
        }
        onDownload={() =>
          downloadClientCsv(
            "vehicle-roi.csv",
            ["vehicle", "roi_percent"],
            roiRows.map((row) => [row.vehicle, row.ROI]),
          )
        }
      >
        {roiRows.length > 0 ? (
          <BarChart
            className="h-80"
            data={roiRows}
            index="vehicle"
            categories={["ROI"]}
            showLegend={false}
            valueFormatter={(value) => `${value}%`}
            yAxisWidth={72}
            onValueChange={handleChartSelect}
          />
        ) : (
          <EmptyNote message="No ROI data for the current filter." />
        )}
      </ChartCard>

      <ChartCard
        icon={Fuel}
        title="Fuel efficiency per vehicle"
        description="Kilometres per litre, highest first. Select a bar to filter the vehicle table below."
        filters={
          <TypeFilter
            id="fuel-type"
            value={fuelType}
            types={vehicleTypes}
            onChange={setFuelType}
          />
        }
        onDownload={() =>
          downloadClientCsv(
            "fuel-efficiency.csv",
            ["vehicle", "km_per_litre"],
            fuelRows.map((row) => [row.vehicle, row.efficiency]),
          )
        }
      >
        {fuelRows.length > 0 ? (
          <BarChart
            className="h-80"
            data={fuelRows}
            index="vehicle"
            categories={["efficiency"]}
            showLegend={false}
            valueFormatter={(value) => `${value} km/L`}
            yAxisWidth={72}
            onValueChange={handleChartSelect}
          />
        ) : (
          <EmptyNote message="No fuel efficiency data for the current filter." />
        )}
      </ChartCard>

      <CardShell
        as="section"
        aria-label="Vehicles"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <SectionHeading
          title="Vehicles"
          description={
            selectedVehicle
              ? `Filtered to ${selectedVehicle}. Select another bar or clear the filter.`
              : "All vehicles in the current dataset. Select a chart bar above to narrow this table."
          }
          action={
            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={selectedVehicle ?? "all"}
                onValueChange={(value) =>
                  setSelectedVehicle(value === "all" ? null : value)
                }
              >
                <SelectTrigger id="vehicle-table-filter" className="w-48" aria-label="Filter vehicle table">
                  <SelectValue placeholder="All vehicles" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All vehicles</SelectItem>
                  {vehicleRegs.map((reg) => (
                    <SelectItem key={reg} value={reg}>
                      {reg}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedVehicle && (
                <Button variant="secondary" onClick={() => setSelectedVehicle(null)}>
                  Clear filter
                </Button>
              )}
            </div>
          }
        />
        <p className="sr-only" role="status">
          {selectedVehicle
            ? `Vehicle table filtered to ${selectedVehicle}.`
            : "Vehicle table shows all vehicles."}
        </p>
        <div className="overflow-x-auto">
          <Table aria-label="Per-vehicle performance">
            <TableHead>
              <TableRow>
                <TableHeaderCell scope="col">Vehicle</TableHeaderCell>
                <TableHeaderCell scope="col">Type</TableHeaderCell>
                <TableHeaderCell scope="col">Status</TableHeaderCell>
                <TableHeaderCell scope="col">ROI</TableHeaderCell>
                <TableHeaderCell scope="col">Efficiency</TableHeaderCell>
                <TableHeaderCell scope="col">Operational cost</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {vehicleRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6}>
                    <EmptyNote message="No vehicles match the current filter." />
                  </TableCell>
                </TableRow>
              ) : (
                vehicleRows.map((row) => (
                  <TableRow
                    key={row.reg}
                    aria-selected={selectedVehicle === row.reg}
                    className={
                      selectedVehicle === row.reg
                        ? "bg-primary/8"
                        : undefined
                    }
                  >
                    <TableCell className="font-medium text-foreground">
                      {row.reg}
                    </TableCell>
                    <TableCell>{row.type}</TableCell>
                    <TableCell>{row.status}</TableCell>
                    <TableCell className="tabular-nums">
                      {row.roi != null ? `${row.roi}%` : "Not tracked"}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {row.efficiency != null ? `${row.efficiency} km/L` : "Not tracked"}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      ${row.cost.toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </CardShell>

      <CardShell
        as="section"
        aria-label="Exports"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <div className="flex items-center gap-3">
          <IconBadge icon={Download} />
          <div>
            <h2 className="text-[15px] font-medium tracking-tight">Exports</h2>
            <p className="mt-0.5 text-sm text-muted-foreground">
              Download the dashboard dataset as CSV, optionally narrowed by
              date or vehicle.
            </p>
          </div>
        </div>
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <div className="flex flex-col gap-1">
            <label htmlFor="export-from" className="text-sm text-muted-foreground">
              From
            </label>
            <input
              id="export-from"
              type="date"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
              className="h-9 rounded-md border border-border bg-card px-3 text-sm text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="export-to" className="text-sm text-muted-foreground">
              To
            </label>
            <input
              id="export-to"
              type="date"
              value={to}
              onChange={(event) => setTo(event.target.value)}
              className="h-9 rounded-md border border-border bg-card px-3 text-sm text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="export-vehicle" className="text-sm text-muted-foreground">
              Vehicle registration
            </label>
            <input
              id="export-vehicle"
              type="text"
              value={vehicleReg}
              onChange={(event) => setVehicleReg(event.target.value)}
              placeholder="Optional"
              className="h-9 rounded-md border border-border bg-card px-3 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            />
          </div>
          <Button
            variant="secondary"
            onClick={exportServerCsv}
            isLoading={exporting}
            loadingText="Exporting…"
          >
            Export dashboard CSV
          </Button>
          <Button variant="link" asChild>
            <Link href="/reports">Open daily trip report</Link>
          </Button>
        </div>
        {exportError && (
          <p role="alert" className="mt-3 text-sm text-destructive">
            {exportError}
          </p>
        )}
      </CardShell>
    </div>
  )
}
