"use client"

import { useEffect, useMemo, useState } from "react"
import { BarChart } from "@/components/BarChart"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { ApiError, apiRequest, extractItems } from "@/lib/api"

type VehicleAnalytics = {
  registration_number: string
  model: string
  type: string
  status: string
  fuel_efficiency: number | null
  roi: number | null
}

type DashboardKPIs = {
  vehicles: VehicleAnalytics[]
}

type FleetVehicle = {
  registration_number: string
  model: string
  type: string
  odometer: number
  status: string
}

type Driver = {
  license_number: string
  name: string
  safety_score: number
  status: string
}

type Trip = {
  id: number
  source_name: string
  destination_name: string
  cargo_weight: number
  status: string
}

type RouteEdge = {
  source: string
  target: string
  maxCargo: number
  tripCount: number
}

function ChartCard({
  title,
  subtitle,
  filters,
  children,
}: {
  title: string
  subtitle: string
  filters?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="w-full rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
      <div className="mb-5 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-50">
            {title}
          </h2>
          <p className="mt-1.5 text-base text-gray-500">{subtitle}</p>
        </div>
        {filters ? <div className="flex flex-wrap gap-2">{filters}</div> : null}
      </div>
      {children}
    </section>
  )
}

function TypeStatusFilters({
  types,
  typeValue,
  statusValue,
  onType,
  onStatus,
}: {
  types: string[]
  typeValue: string
  statusValue: string
  onType: (v: string) => void
  onStatus: (v: string) => void
}) {
  return (
    <>
      <Select value={typeValue} onValueChange={onType}>
        <SelectTrigger className="w-44">
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
      <Select value={statusValue} onValueChange={onStatus}>
        <SelectTrigger className="w-44">
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
    </>
  )
}

/** Directed route graph: large circle, readable labels, small arrow tips */
function RouteFlowMap({ edges }: { edges: RouteEdge[] }) {
  const width = 1400
  const height = 900
  const cx = width / 2
  const cy = height / 2
  const radius = Math.min(width, height) * 0.4

  const nodes = useMemo(() => {
    const names = Array.from(
      new Set(edges.flatMap((e) => [e.source, e.target])),
    ).sort((a, b) => a.localeCompare(b))
    const n = Math.max(names.length, 1)
    return names.map((name, i) => {
      const angle = (2 * Math.PI * i) / n - Math.PI / 2
      return {
        name,
        short: name.length > 26 ? `${name.slice(0, 24)}…` : name,
        x: cx + radius * Math.cos(angle),
        y: cy + radius * Math.sin(angle),
        lx: cx + (radius + 70) * Math.cos(angle),
        ly: cy + (radius + 70) * Math.sin(angle),
      }
    })
  }, [edges, cx, cy, radius])

  const nodeMap = useMemo(
    () => new Map(nodes.map((n) => [n.name, n])),
    [nodes],
  )

  const maxCargo = Math.max(...edges.map((e) => e.maxCargo), 1)

  if (edges.length === 0) {
    return (
      <p className="py-16 text-center text-base text-gray-500">
        No route data available
      </p>
    )
  }

  return (
    <div className="w-full overflow-x-auto">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="mx-auto h-[720px] w-full min-w-[960px]"
        role="img"
        aria-label="Route capacity map"
      >
        <defs>
          <marker
            id="route-arrow-sm"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="5"
            markerHeight="5"
            markerUnits="userSpaceOnUse"
            orient="auto-start-reverse"
          >
            <path d="M 0 1.5 L 8 5 L 0 8.5 Z" fill="#2563eb" />
          </marker>
        </defs>

        {edges.map((edge) => {
          const from = nodeMap.get(edge.source)
          const to = nodeMap.get(edge.target)
          if (!from || !to) return null

          const dx = to.x - from.x
          const dy = to.y - from.y
          const len = Math.hypot(dx, dy) || 1
          const pad = 18
          const x1 = from.x + (dx / len) * pad
          const y1 = from.y + (dy / len) * pad
          const x2 = to.x - (dx / len) * (pad + 6)
          const y2 = to.y - (dy / len) * (pad + 6)
          const mx = (x1 + x2) / 2 - dy * 0.1
          const my = (y1 + y2) / 2 + dx * 0.1
          const strokeW = 2 + (edge.maxCargo / maxCargo) * 10

          return (
            <g key={`${edge.source}->${edge.target}`}>
              <title>
                {`${edge.source} → ${edge.target}\nMax cargo: ${edge.maxCargo.toLocaleString()} kg\nTrips: ${edge.tripCount}`}
              </title>
              <path
                d={`M ${x1} ${y1} Q ${mx} ${my} ${x2} ${y2}`}
                fill="none"
                stroke="#2563eb"
                strokeOpacity={0.7}
                strokeWidth={strokeW}
                strokeLinecap="round"
                markerEnd="url(#route-arrow-sm)"
              />
            </g>
          )
        })}

        {nodes.map((node) => (
          <g key={node.name}>
            <circle
              cx={node.x}
              cy={node.y}
              r={7}
              fill="#0f172a"
              stroke="#cbd5e1"
              strokeWidth={2}
            />
            <text
              x={node.lx}
              y={node.ly}
              textAnchor="middle"
              dominantBaseline="middle"
              fill="#334155"
              fontSize={13}
              fontWeight={500}
            >
              {node.short}
            </text>
          </g>
        ))}
      </svg>
      <p className="mt-3 text-center text-sm text-gray-500">
        Line thickness = max cargo · small arrows show direction (source →
        destination)
      </p>
    </div>
  )
}

export default function AnalyticsPage() {
  const [kpis, setKpis] = useState<DashboardKPIs | null>(null)
  const [vehicles, setVehicles] = useState<FleetVehicle[]>([])
  const [drivers, setDrivers] = useState<Driver[]>([])
  const [trips, setTrips] = useState<Trip[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Independent filters per chart
  const [roiType, setRoiType] = useState("all")
  const [roiStatus, setRoiStatus] = useState("all")
  const [fuelType, setFuelType] = useState("all")
  const [fuelStatus, setFuelStatus] = useState("all")
  const [driverStatus, setDriverStatus] = useState("all")
  const [odoType, setOdoType] = useState("all")
  const [odoStatus, setOdoStatus] = useState("all")
  const [tripStatusFilter, setTripStatusFilter] = useState("all")

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        setError(null)
        const [kpiData, fleet, driverList, tripList] = await Promise.all([
          apiRequest<DashboardKPIs>("/dashboard/kpis"),
          apiRequest<any>("/fleet/vehicles"),
          apiRequest<any>("/fleet/drivers"),
          apiRequest<any>("/trips/"),
        ])
        if (cancelled) return
        setKpis(kpiData)
        setVehicles(extractItems<FleetVehicle>(fleet))
        setDrivers(extractItems<Driver>(driverList))
        setTrips(extractItems<Trip>(tripList))
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof ApiError ? err.detail : "Failed to load analytics",
          )
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [])

  const vehicleTypes = useMemo(
    () =>
      Array.from(
        new Set(
          (kpis?.vehicles ?? [])
            .map((v) => v.type)
            .concat(vehicles.map((v) => v.type)),
        ),
      ).sort(),
    [kpis, vehicles],
  )

  const roiChart = useMemo(() => {
    if (!kpis) return []
    return kpis.vehicles
      .filter((v) => {
        if (roiType !== "all" && v.type !== roiType) return false
        if (roiStatus !== "all" && v.status !== roiStatus) return false
        return v.roi != null
      })
      .sort((a, b) => (b.roi ?? 0) - (a.roi ?? 0))
      .slice(0, 35)
      .map((v) => ({
        vehicle: v.registration_number,
        ROI: Number(((v.roi ?? 0) * 100).toFixed(1)),
      }))
  }, [kpis, roiType, roiStatus])

  const fuelChart = useMemo(() => {
    if (!kpis) return []
    return kpis.vehicles
      .filter((v) => {
        if (fuelType !== "all" && v.type !== fuelType) return false
        if (fuelStatus !== "all" && v.status !== fuelStatus) return false
        return v.fuel_efficiency != null && v.fuel_efficiency > 0
      })
      .sort((a, b) => (b.fuel_efficiency ?? 0) - (a.fuel_efficiency ?? 0))
      .slice(0, 35)
      .map((v) => ({
        vehicle: v.registration_number,
        "Fuel Efficiency": Number((v.fuel_efficiency ?? 0).toFixed(2)),
      }))
  }, [kpis, fuelType, fuelStatus])

  const safetyChart = useMemo(() => {
    return [...drivers]
      .filter((d) => driverStatus === "all" || d.status === driverStatus)
      .sort((a, b) => b.safety_score - a.safety_score)
      .slice(0, 35)
      .map((d) => ({
        driver: `${d.name} (${d.license_number.slice(-4)})`,
        "Safety Rating": Number(d.safety_score.toFixed(1)),
      }))
  }, [drivers, driverStatus])

  const odometerChart = useMemo(() => {
    return [...vehicles]
      .filter((v) => {
        if (odoType !== "all" && v.type !== odoType) return false
        if (odoStatus !== "all" && v.status !== odoStatus) return false
        return true
      })
      .sort((a, b) => b.odometer - a.odometer)
      .slice(0, 35)
      .map((v) => ({
        vehicle: v.registration_number,
        Odometer: Number(v.odometer.toFixed(0)),
      }))
  }, [vehicles, odoType, odoStatus])

  const routeEdges = useMemo(() => {
    const map = new Map<string, RouteEdge>()
    for (const t of trips) {
      if (tripStatusFilter !== "all" && t.status !== tripStatusFilter) continue
      if (t.source_name === t.destination_name) continue
      const key = `${t.source_name}||${t.destination_name}`
      const existing = map.get(key)
      if (!existing) {
        map.set(key, {
          source: t.source_name,
          target: t.destination_name,
          maxCargo: t.cargo_weight,
          tripCount: 1,
        })
      } else {
        existing.maxCargo = Math.max(existing.maxCargo, t.cargo_weight)
        existing.tripCount += 1
      }
    }
    return Array.from(map.values())
      .sort((a, b) => b.maxCargo - a.maxCargo)
      .slice(0, 16)
  }, [trips, tripStatusFilter])

  if (loading) {
    return (
      <div className="p-8 text-base text-gray-500">Loading analytics...</div>
    )
  }

  if (error) {
    return <div className="p-8 text-base text-red-600">{error}</div>
  }

  return (
    <div className="flex w-full flex-col gap-10 p-6 lg:p-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Analytics</h1>
        <p className="mt-2 text-base text-gray-600 dark:text-gray-400">
          Full-width charts stacked top to bottom. Each chart has its own
          filters.
        </p>
      </div>

      <ChartCard
        title="1. Vehicle ROI (%)"
        subtitle="X: vehicle · Y: ROI — filters apply only to this chart"
        filters={
          <TypeStatusFilters
            types={vehicleTypes}
            typeValue={roiType}
            statusValue={roiStatus}
            onType={setRoiType}
            onStatus={setRoiStatus}
          />
        }
      >
        {roiChart.length > 0 ? (
          <BarChart
            className="h-[520px]"
            data={roiChart}
            index="vehicle"
            categories={["ROI"]}
            colors={["violet"]}
            showLegend={false}
            layout="vertical"
            valueFormatter={(v) => `${v}%`}
            yAxisWidth={88}
          />
        ) : (
          <p className="py-12 text-center text-base text-gray-500">
            No ROI data for current filters
          </p>
        )}
      </ChartCard>

      <ChartCard
        title="2. Fuel Efficiency per Truck"
        subtitle="X: vehicle · Y: km/L — independent filters"
        filters={
          <TypeStatusFilters
            types={vehicleTypes}
            typeValue={fuelType}
            statusValue={fuelStatus}
            onType={setFuelType}
            onStatus={setFuelStatus}
          />
        }
      >
        {fuelChart.length > 0 ? (
          <BarChart
            className="h-[520px]"
            data={fuelChart}
            index="vehicle"
            categories={["Fuel Efficiency"]}
            colors={["blue"]}
            showLegend={false}
            layout="vertical"
            valueFormatter={(v) => `${v} km/L`}
            yAxisWidth={88}
          />
        ) : (
          <p className="py-12 text-center text-base text-gray-500">
            No fuel efficiency data for current filters
          </p>
        )}
      </ChartCard>

      <ChartCard
        title="3. Driver vs Safety Rating"
        subtitle="X: driver · Y: safety score — filter by driver status only"
        filters={
          <Select value={driverStatus} onValueChange={setDriverStatus}>
            <SelectTrigger className="w-44">
              <SelectValue placeholder="Driver status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="Available">Available</SelectItem>
              <SelectItem value="On Trip">On Trip</SelectItem>
              <SelectItem value="Off Duty">Off Duty</SelectItem>
              <SelectItem value="Suspended">Suspended</SelectItem>
            </SelectContent>
          </Select>
        }
      >
        {safetyChart.length > 0 ? (
          <BarChart
            className="h-[520px]"
            data={safetyChart}
            index="driver"
            categories={["Safety Rating"]}
            colors={["emerald"]}
            showLegend={false}
            layout="vertical"
            valueFormatter={(v) => `${v}`}
            yAxisWidth={140}
            maxValue={100}
          />
        ) : (
          <p className="py-12 text-center text-base text-gray-500">
            No drivers for current filters
          </p>
        )}
      </ChartCard>

      <ChartCard
        title="4. Vehicle vs Odometer"
        subtitle="X: vehicle · Y: odometer km — independent filters"
        filters={
          <TypeStatusFilters
            types={vehicleTypes}
            typeValue={odoType}
            statusValue={odoStatus}
            onType={setOdoType}
            onStatus={setOdoStatus}
          />
        }
      >
        {odometerChart.length > 0 ? (
          <BarChart
            className="h-[520px]"
            data={odometerChart}
            index="vehicle"
            categories={["Odometer"]}
            colors={["amber"]}
            showLegend={false}
            layout="vertical"
            valueFormatter={(v) => `${v.toLocaleString()} km`}
            yAxisWidth={88}
          />
        ) : (
          <p className="py-12 text-center text-base text-gray-500">
            No vehicles for current filters
          </p>
        )}
      </ChartCard>

      <ChartCard
        title="5. Route Capacity Map"
        subtitle="Larger circle · destinations as vertices · small direction arrows · thicker edges = more cargo"
        filters={
          <Select value={tripStatusFilter} onValueChange={setTripStatusFilter}>
            <SelectTrigger className="w-48">
              <SelectValue placeholder="Trip status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All trips</SelectItem>
              <SelectItem value="Dispatched">Dispatched</SelectItem>
              <SelectItem value="Completed">Completed</SelectItem>
              <SelectItem value="Cancelled">Cancelled</SelectItem>
              <SelectItem value="Draft">Draft</SelectItem>
            </SelectContent>
          </Select>
        }
      >
        <div className="mb-3 flex flex-wrap gap-6 text-sm text-gray-500">
          <span>
            Top corridors: <strong className="text-gray-800">{routeEdges.length}</strong>
          </span>
          <span>
            Max cargo:{" "}
            <strong className="text-gray-800">
              {routeEdges[0]
                ? `${routeEdges[0].maxCargo.toLocaleString()} kg`
                : "—"}
            </strong>
          </span>
        </div>
        <RouteFlowMap edges={routeEdges} />
      </ChartCard>
    </div>
  )
}
