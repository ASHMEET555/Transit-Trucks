"use client"

import Link from "next/link"
import { useEffect, useState, useCallback } from "react"
import {
  Truck,
  CheckCircle2,
  Wrench,
  TrendingUp,
  DollarSign,
  AlertTriangle,
  Download,
  Calendar,
  Layers,
  ArrowRight,
  Clock,
  Sparkles,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import { useProfile } from "@/lib/ProfileContext"
import { RBAC_CONFIG } from "@/lib/rbacConfig"
import {
  getDashboardKPIs,
  getDriverDashboard,
  getDashboardUtilization,
  listDrivers,
  listMaintenanceLogs,
  listTrips,
  type DashboardKPIs,
  type DriverDashboard,
  type UtilizationResponse,
  type Driver,
  type MaintenanceLog,
  type Trip,
} from "@/lib/api/client"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { TableSkeleton, ErrorState } from "@/components/ui/data-states"

export default function DashboardPage() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role || "Fleet Manager"
  const isDriver = role === "Driver"
  const tripsPerm = RBAC_CONFIG[role]?.permissions?.trips
  const canWriteTrips = tripsPerm === "read_write" || tripsPerm === "admin"

  const [kpis, setKpis] = useState<DashboardKPIs | null>(null)
  const [driverData, setDriverData] = useState<DriverDashboard | null>(null)
  const [utilization, setUtilization] = useState<UtilizationResponse | null>(null)
  const [expiringDrivers, setExpiringDrivers] = useState<Driver[]>([])
  const [openRepairs, setOpenRepairs] = useState<MaintenanceLog[]>([])
  const [draftTrips, setDraftTrips] = useState<Trip[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    try {
      setError(null)
      if (isDriver) {
        const [meData, utilData] = await Promise.all([
          getDriverDashboard(),
          getDashboardUtilization(7).catch(() => null),
        ])
        setDriverData(meData)
        setUtilization(utilData)
      } else {
        const [kpiRes, utilRes, driversRes, maintRes, tripsRes] = await Promise.all([
          getDashboardKPIs(),
          getDashboardUtilization(7).catch(() => null),
          listDrivers({ license_expiring_within_days: 30, limit: 5 }).catch(() => ({ items: [], next_cursor: null })),
          listMaintenanceLogs({ status: "Open", limit: 5 }).catch(() => ({ items: [], next_cursor: null })),
          listTrips({ status: "Draft", limit: 5 }).catch(() => ({ items: [], next_cursor: null })),
        ])
        setKpis(kpiRes)
        setUtilization(utilRes)
        setExpiringDrivers(driversRes.items || [])
        setOpenRepairs(maintRes.items || [])
        setDraftTrips(tripsRes.items || [])
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load dashboard data.")
    } finally {
      setLoading(false)
    }
  }, [isDriver])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: (event) => {
      if (
        event.type.startsWith("trip.") ||
        event.type.startsWith("vehicle.") ||
        event.type.startsWith("maintenance.") ||
        event.type.startsWith("driver.")
      ) {
        loadData()
      }
    },
  })

  useEffect(() => {
    if (isLoaded) {
      loadData()
    }
  }, [isLoaded, loadData])

  if (loading) {
    return (
      <div className="p-6 md:p-8 space-y-6">
        <div className="h-8 w-48 bg-neutral-200/70 rounded animate-pulse" />
        <TableSkeleton rows={4} cols={4} />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-6 md:p-8">
        <ErrorState message={error} onRetry={loadData} />
      </div>
    )
  }

  return (
    <div className="p-6 md:p-8 space-y-8 max-w-7xl mx-auto">
      {/* Header bar */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border/60 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">
              {isDriver ? "Driver Workstation" : "Operations Control"}
            </h1>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <p className="text-xs text-muted-foreground mt-1">
            {isDriver
              ? `License: ${driverData?.driver_license || "—"} • Real-time assigned freight status`
              : "Live depot throughput, vehicle availability, and fleet KPI telemetry."}
          </p>
        </div>

        <div className="flex items-center gap-2">
          {!isDriver && (
            <a href="/api/v1/dashboard/export.csv" download="transitops-operations.csv">
              <Button variant="outline" size="sm" className="gap-2 text-xs">
                <Download className="size-3.5" /> Export Operations CSV
              </Button>
            </a>
          )}
          {canWriteTrips && (
            <Link href="/trips">
              <Button size="sm" className="gap-2 text-xs bg-primary text-primary-foreground hover:bg-primary/90">
                <Sparkles className="size-3.5" /> Dispatch Trip
              </Button>
            </Link>
          )}
        </div>
      </div>

      {/* KPI Row (Operations-Now Centerpiece) */}
      {isDriver ? (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-2">
              <span className="text-xs font-medium">Completed Trips</span>
              <CheckCircle2 className="size-4 text-primary" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              {driverData?.total_completed_trips ?? 0}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">Verified deliveries</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-2">
              <span className="text-xs font-medium">Total Distance</span>
              <Truck className="size-4 text-primary" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              {(driverData?.total_distance ?? 0).toLocaleString()} <span className="text-sm font-normal">km</span>
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">Logged freight odometer</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-2">
              <span className="text-xs font-medium">Active / Dispatched</span>
              <Clock className="size-4 text-signal" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              {driverData?.trips_by_status?.Dispatched ?? 0}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">In transit right now</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-2">
              <span className="text-xs font-medium">Assigned Drafts</span>
              <Layers className="size-4 text-muted-foreground" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              {driverData?.trips_by_status?.Draft ?? 0}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">Awaiting dispatch</div>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-6 gap-3">
          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-1.5">
              <span className="text-xs font-medium">Active Trips</span>
              <Truck className="size-3.5 text-signal" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">{kpis?.active_trips ?? 0}</div>
            <div className="text-[11px] text-muted-foreground mt-1">En route currently</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-1.5">
              <span className="text-xs font-medium">Available</span>
              <CheckCircle2 className="size-3.5 text-primary" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">{kpis?.available_vehicles ?? 0}</div>
            <div className="text-[11px] text-muted-foreground mt-1">Ready for dispatch</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-1.5">
              <span className="text-xs font-medium">In Shop</span>
              <Wrench className="size-3.5 text-destructive" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">{kpis?.in_shop_vehicles ?? 0}</div>
            <div className="text-[11px] text-muted-foreground mt-1">Active repair orders</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-1.5">
              <span className="text-xs font-medium">Utilization</span>
              <TrendingUp className="size-3.5 text-primary" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              {((kpis?.fleet_utilization ?? 0) * 100).toFixed(1)}%
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">Fleet active ratio</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-1.5">
              <span className="text-xs font-medium">Total Revenue</span>
              <DollarSign className="size-3.5 text-primary" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              ${(kpis?.total_revenue ?? 0).toLocaleString()}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">Billed freight</div>
          </div>

          <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
            <div className="flex items-center justify-between text-muted-foreground mb-1.5">
              <span className="text-xs font-medium">Operating Cost</span>
              <DollarSign className="size-3.5 text-muted-foreground" />
            </div>
            <div className="text-2xl font-bold text-foreground tabular-nums">
              ${(kpis?.total_operational_cost ?? 0).toLocaleString()}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1">Fuel + maintenance</div>
          </div>
        </div>
      )}

      {/* Middle Grid: 7-Day Utilization Chart + Exceptions Queue */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* 7-Day Utilization Chart */}
        <div className="lg:col-span-2 rounded-xl border border-border/80 bg-card p-5 shadow-xs">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
                <Calendar className="size-4 text-primary" /> 7-Day Dispatch & Completion Trend
              </h2>
              <p className="text-xs text-muted-foreground mt-0.5">
                Daily freight dispatch volume vs completed shipments (UTC)
              </p>
            </div>
            <div className="flex items-center gap-4 text-xs font-medium">
              <span className="flex items-center gap-1.5">
                <span className="size-2.5 rounded-full bg-signal" /> Dispatched
              </span>
              <span className="flex items-center gap-1.5">
                <span className="size-2.5 rounded-full bg-primary" /> Completed
              </span>
            </div>
          </div>

          {utilization?.points && utilization.points.length > 0 ? (
            <div className="h-48 w-full flex items-end gap-2 pt-6 pb-2 border-b border-border/50">
              {utilization.points.map((pt) => {
                const maxVal = Math.max(
                  ...((utilization.points || []).map((p) => Math.max(p.dispatched ?? 0, p.completed ?? 0))),
                  5
                )
                const disp = pt.dispatched ?? 0
                const comp = pt.completed ?? 0
                const dispH = Math.max(Math.round((disp / maxVal) * 140), 4)
                const compH = Math.max(Math.round((comp / maxVal) * 140), 4)
                const dateStr = pt.date ?? ""

                return (
                  <div key={dateStr} className="flex-1 flex flex-col items-center gap-1 group">
                    <div className="w-full flex items-end justify-center gap-1 h-36">
                      <div
                        style={{ height: `${dispH}px` }}
                        className="w-1/2 max-w-[18px] bg-signal rounded-t transition-all group-hover:opacity-85"
                        title={`${dateStr}: ${disp} dispatched`}
                      />
                      <div
                        style={{ height: `${compH}px` }}
                        className="w-1/2 max-w-[18px] bg-primary rounded-t transition-all group-hover:opacity-85"
                        title={`${dateStr}: ${comp} completed`}
                      />
                    </div>
                    <span className="text-[10px] text-muted-foreground truncate w-full text-center">
                      {dateStr.slice(5)}
                    </span>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="h-48 flex items-center justify-center text-xs text-muted-foreground">
              No trip volume recorded over the past 7 days.
            </div>
          )}
        </div>

        {/* Exceptions Queue */}
        <div className="rounded-xl border border-border/80 bg-card p-5 shadow-xs flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
                <AlertTriangle className="size-4 text-signal" /> Operational Exceptions
              </h2>
              <span className="text-[11px] font-mono px-2 py-0.5 rounded-full bg-neutral-100 text-neutral-700">
                {expiringDrivers.length + openRepairs.length + draftTrips.length} pending
              </span>
            </div>
            <p className="text-xs text-muted-foreground mb-4">
              Critical items requiring manager action or dispatch review.
            </p>

            <div className="space-y-2.5">
              {/* Expiring Drivers */}
              {expiringDrivers.length > 0 && (
                <Link
                  href="/drivers"
                  className="flex items-center justify-between p-2.5 rounded-lg border border-signal/20 bg-signal/5 hover:bg-signal/10 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <span className="size-2 rounded-full bg-signal" />
                    <div>
                      <div className="text-xs font-semibold text-foreground">
                        {expiringDrivers.length} Driver License{expiringDrivers.length > 1 ? "s" : ""} Expiring
                      </div>
                      <div className="text-[11px] text-muted-foreground">Expires within 30 days</div>
                    </div>
                  </div>
                  <ArrowRight className="size-3.5 text-muted-foreground" />
                </Link>
              )}

              {/* Open Repairs */}
              {openRepairs.length > 0 && (
                <Link
                  href="/maintenance"
                  className="flex items-center justify-between p-2.5 rounded-lg border border-destructive/20 bg-destructive/5 hover:bg-destructive/10 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <span className="size-2 rounded-full bg-destructive" />
                    <div>
                      <div className="text-xs font-semibold text-foreground">
                        {openRepairs.length} Open Vehicle Repair{openRepairs.length > 1 ? "s" : ""}
                      </div>
                      <div className="text-[11px] text-muted-foreground">Maintenance in progress</div>
                    </div>
                  </div>
                  <ArrowRight className="size-3.5 text-muted-foreground" />
                </Link>
              )}

              {/* Draft Trips */}
              {draftTrips.length > 0 && (
                <Link
                  href="/trips"
                  className="flex items-center justify-between p-2.5 rounded-lg border border-border bg-neutral-50 hover:bg-neutral-100 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <span className="size-2 rounded-full bg-neutral-400" />
                    <div>
                      <div className="text-xs font-semibold text-foreground">
                        {draftTrips.length} Unassigned Draft Trip{draftTrips.length > 1 ? "s" : ""}
                      </div>
                      <div className="text-[11px] text-muted-foreground">Pending vehicle assignment</div>
                    </div>
                  </div>
                  <ArrowRight className="size-3.5 text-muted-foreground" />
                </Link>
              )}

              {expiringDrivers.length === 0 && openRepairs.length === 0 && draftTrips.length === 0 && (
                <div className="text-center py-8 text-xs text-muted-foreground">
                  <CheckCircle2 className="size-5 text-primary mx-auto mb-2" />
                  All compliance and maintenance queues are clear.
                </div>
              )}
            </div>
          </div>

          <div className="pt-4 border-t border-border/60 mt-4">
            <Link
              href="/trips"
              className="text-xs font-medium text-primary hover:underline flex items-center justify-between"
            >
              <span>Go to full dispatch board</span>
              <ArrowRight className="size-3.5" />
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}
