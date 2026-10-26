"use client"

import Link from "next/link"
import { useEffect, useState } from "react"
import { BarChart } from "@/components/BarChart"
import { LiveFleetMap } from "@/components/map/LiveFleetMap"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import type { Destination, Trip } from "@/lib/tripTypes"

type DashboardKPIs = {
  active_vehicles: number
  available_vehicles: number
  in_shop_vehicles: number
  active_trips: number
  drivers_on_duty: number
  fleet_utilization: number
  overall_fuel_efficiency: number | null
  total_operational_cost: number
  total_revenue: number
  vehicles: Array<{
    registration_number: string
    model: string
    type: string
    status: string
    roi: number | null
    fuel_efficiency: number | null
    operational_cost: number
  }>
}

function Kpi({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="kpi-card">
      <p>{label}</p>
      <p>{value}</p>
    </div>
  )
}

export default function DashboardPage() {
  const [data, setData] = useState<DashboardKPIs | null>(null)
  const [trips, setTrips] = useState<Trip[]>([])
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [error, setError] = useState<string | null>(null)
  const [typeFilter, setTypeFilter] = useState("all")
  const [statusFilter, setStatusFilter] = useState("all")
  const [lastRealtimeEvent, setLastRealtimeEvent] = useState<string | null>(null)

  const loadKpis = () =>
    apiRequest<DashboardKPIs>("/dashboard/kpis")
      .then(setData)
      .catch((err) =>
        setError(err instanceof ApiError ? err.detail : "Failed to load KPIs"),
      )

  const loadMapData = () =>
    Promise.all([
      apiRequest<any>("/trips/"),
      apiRequest<any>("/destinations/"),
    ])
      .then(([t, d]) => {
        setTrips(extractItems<Trip>(t))
        setDestinations(extractItems<Destination>(d))
      })
      .catch(() => {
        // Map is optional on dashboard
      })

  useEffect(() => {
    loadKpis()
    loadMapData()
  }, [])

  const { status } = useFleetEvents({
    onEvent: (event) => {
      setLastRealtimeEvent(event.type)
      loadKpis()
      loadMapData()
    },
    onReconnect: () => {
      loadKpis()
      loadMapData()
    },
  })

  if (error) return <div className="p-6 text-red-500">{error}</div>
  if (!data) return <div className="p-6 text-gray-500">Loading dashboard...</div>

  const filtered = data.vehicles.filter((v) => {
    if (typeFilter !== "all" && v.type !== typeFilter) return false
    if (statusFilter !== "all" && v.status !== statusFilter) return false
    return true
  })

  const chartData = filtered.map((v) => ({
    name: v.registration_number,
    ROI: Number(((v.roi ?? 0) * 100).toFixed(1)),
  }))

  const types = Array.from(new Set(data.vehicles.map((v) => v.type)))

  return (
    <div className="page-shell space-y-6">
      <div className="page-header">
        <div>
          <div className="flex items-center gap-3">
            <p className="page-eyebrow">Command center</p>
            <RealtimeIndicator status={status} />
          </div>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-subtitle">
            Fleet overview with live map, KPIs, and vehicle performance.
          </p>
          {lastRealtimeEvent && (
            <p className="mt-2 text-xs text-cyan-400">
              Live event: {lastRealtimeEvent}
            </p>
          )}
        </div>
        <Link href="/live-map" className="landing-cta-sm">
          Full screen map
        </Link>
      </div>

      <div className="dashboard-map-embed">
        <LiveFleetMap
          trips={trips}
          destinations={destinations}
          height="360px"
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Kpi label="Active Vehicles" value={data.active_vehicles} />
        <Kpi label="Available Vehicles" value={data.available_vehicles} />
        <Kpi label="In Maintenance" value={data.in_shop_vehicles} />
        <Kpi label="Active Trips" value={data.active_trips} />
        <Kpi label="Drivers On Duty" value={data.drivers_on_duty} />
        <Kpi
          label="Fleet Utilization"
          value={`${data.fleet_utilization.toFixed(1)}%`}
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <Kpi
          label="Fuel Efficiency"
          value={
            data.overall_fuel_efficiency != null
              ? `${data.overall_fuel_efficiency.toFixed(2)} km/L`
              : "N/A"
          }
        />
        <Kpi
          label="Total Revenue"
          value={`$${data.total_revenue.toLocaleString()}`}
        />
        <Kpi
          label="Operational Cost"
          value={`$${data.total_operational_cost.toLocaleString()}`}
        />
      </div>

      <div className="rounded-lg border p-4 dark:border-gray-800">
        <div className="mb-4 flex flex-wrap gap-2">
          <Select value={typeFilter} onValueChange={setTypeFilter}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="Type" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All types</SelectItem>
              {types.map((t) => (
                <SelectItem key={t} value={t}>
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="Available">Available</SelectItem>
              <SelectItem value="On Trip">On Trip</SelectItem>
              <SelectItem value="In Shop">In Shop</SelectItem>
              <SelectItem value="Retired">Retired</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <h2 className="mb-3 font-semibold">Vehicle ROI (%)</h2>
        {chartData.length > 0 ? (
          <BarChart
            className="h-72"
            data={chartData}
            index="name"
            categories={["ROI"]}
            showLegend={false}
          />
        ) : (
          <p className="py-8 text-center text-sm text-gray-500">No data</p>
        )}
      </div>

      <div className="overflow-x-auto rounded-lg border dark:border-gray-800">
        <Table>
          <TableHead>
            <TableRow>
              <TableHeaderCell>Vehicle</TableHeaderCell>
              <TableHeaderCell>Status</TableHeaderCell>
              <TableHeaderCell>ROI</TableHeaderCell>
              <TableHeaderCell>Fuel Eff.</TableHeaderCell>
              <TableHeaderCell>Op. Cost</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filtered.map((v) => (
              <TableRow key={v.registration_number}>
                <TableCell>
                  {v.registration_number} · {v.model}
                </TableCell>
                <TableCell>{v.status}</TableCell>
                <TableCell>
                  {v.roi != null ? `${(v.roi * 100).toFixed(1)}%` : "N/A"}
                </TableCell>
                <TableCell>
                  {v.fuel_efficiency != null
                    ? `${v.fuel_efficiency.toFixed(2)} km/L`
                    : "N/A"}
                </TableCell>
                <TableCell>${v.operational_cost.toLocaleString()}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
