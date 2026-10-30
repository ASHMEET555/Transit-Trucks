"use client"

import { TriangleAlert } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/Button"
import { LiveFleetMap } from "@/components/map/LiveFleetMap"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite, hasRouteAccess } from "@/lib/rbacConfig"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import type { Destination, Trip } from "@/lib/tripTypes"
import { DriverHome, type DriverDashboard } from "./_components/DriverHome"
import { downloadDashboardCsv } from "./_components/export"
import {
  EmptyNote,
  ErrorBlock,
  ForbiddenBlock,
  LoadingBlock,
  SectionHeading,
} from "./_components/states"
import { KpiCompact } from "./_components/kpi-compact"
import { PlanRouteDrawer } from "./_components/plan-route-drawer"
import type { DashboardKPIs } from "./_components/kpi-strip"
import {
  ExceptionQueue,
  EXPIRY_WINDOW_DAYS,
  type FleetDriver,
  type MaintenanceLog,
} from "./_components/exception-queue"
import { MapPanel } from "./_components/map-panel"

export default function DashboardPage() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role ?? ""
  const isDriver = role === "Driver"
  const allowed = !isLoaded || hasRouteAccess(role, "dashboard")
  const canPlan = Boolean(
    currentProfile && canWrite(currentProfile.role, "trips"),
  )

  const [kpis, setKpis] = useState<DashboardKPIs | null>(null)
  const [driverData, setDriverData] = useState<DriverDashboard | null>(null)
  const [trips, setTrips] = useState<Trip[]>([])
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [expiringDrivers, setExpiringDrivers] = useState<FleetDriver[] | null>(
    null,
  )
  const [openMaintenance, setOpenMaintenance] = useState<
    MaintenanceLog[] | null
  >(null)
  const [error, setError] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)
  const [loading, setLoading] = useState(true)
  const [exporting, setExporting] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)
  const [selectedTripId, setSelectedTripId] = useState<number | null>(null)
  const [planOpen, setPlanOpen] = useState(false)
  const [lastRealtimeEvent, setLastRealtimeEvent] = useState<string | null>(
    null,
  )

  const load = useCallback(() => {
    if (!isLoaded || !currentProfile) return
    if (!hasRouteAccess(currentProfile.role, "dashboard")) {
      setForbidden(true)
      setLoading(false)
      return
    }
    setForbidden(false)
    setLoading(true)
    setError(null)

    if (currentProfile.role === "Driver") {
      apiRequest<DriverDashboard>("/dashboard/me")
        .then(setDriverData)
        .catch((err) => {
          if (err instanceof ApiError && err.status === 403) {
            setForbidden(true)
          } else {
            setError(
              err instanceof ApiError
                ? err.detail
                : "Failed to load driver dashboard",
            )
          }
        })
        .finally(() => setLoading(false))
      return
    }

    Promise.allSettled([
      apiRequest<DashboardKPIs>("/dashboard/kpis"),
      apiRequest<unknown>("/trips/"),
      apiRequest<unknown>("/destinations/"),
      apiRequest<unknown>(
        `/fleet/drivers?license_expiring_within_days=${EXPIRY_WINDOW_DAYS}`,
      ),
      apiRequest<unknown>("/maintenance/"),
    ]).then((results) => {
      const [kpisRes, tripsRes, destRes, driversRes, maintRes] = results

      if (kpisRes.status === "rejected") {
        const err = kpisRes.reason
        if (err instanceof ApiError && err.status === 403) {
          setForbidden(true)
        } else {
          setError(
            err instanceof ApiError ? err.detail : "Failed to load KPIs",
          )
        }
        setLoading(false)
        return
      }
      setKpis(kpisRes.value)

      if (tripsRes.status === "fulfilled") {
        setTrips(extractItems<Trip>(tripsRes.value))
      }
      if (destRes.status === "fulfilled") {
        setDestinations(extractItems<Destination>(destRes.value))
      }
      setExpiringDrivers(
        driversRes.status === "fulfilled"
          ? extractItems<FleetDriver>(driversRes.value)
          : null,
      )
      setOpenMaintenance(
        maintRes.status === "fulfilled"
          ? extractItems<MaintenanceLog>(maintRes.value).filter(
              (log) => log.end_date == null,
            )
          : null,
      )
      setLoading(false)
    })
  }, [isLoaded, currentProfile])

  useEffect(() => {
    load()
  }, [load])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: (event) => {
      setLastRealtimeEvent(event.type)
      load()
    },
    onReconnect: load,
  })

  const exportCsv = useCallback(async () => {
    setExporting(true)
    setExportError(null)
    try {
      await downloadDashboardCsv()
    } catch (err) {
      setExportError(err instanceof Error ? err.message : "Export failed")
    } finally {
      setExporting(false)
    }
  }, [])

  const draftTrips = useMemo(
    () => trips.filter((trip) => trip.status === "Draft").slice(0, 5),
    [trips],
  )
  const draftCount = useMemo(
    () => trips.filter((trip) => trip.status === "Draft").length,
    [trips],
  )

  if (!isLoaded || loading) {
    return (
      <div className="space-y-6 p-6 lg:p-8" aria-busy="true">
        <LoadingBlock label="Loading dashboard" />
      </div>
    )
  }

  if (!allowed || forbidden) {
    return (
      <div className="p-6 lg:p-8">
        <ForbiddenBlock message="Your role does not have access to the dashboard." />
      </div>
    )
  }

  if (error && !kpis && !driverData) {
    return (
      <div className="p-6 lg:p-8">
        <ErrorBlock message={error} onRetry={load} />
      </div>
    )
  }

  if (isDriver && driverData) {
    return (
      <div className="p-6 lg:p-8">
        <DriverHome data={driverData} realtimeStatus={realtimeStatus} />
      </div>
    )
  }

  if (!kpis) {
    return (
      <div className="p-6 lg:p-8">
        <EmptyNote message="No dashboard data available." />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="flex items-center gap-3">
            <p className="text-sm text-muted-foreground">Operations today</p>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">Dashboard</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            What needs attention right now across the fleet.
          </p>
          {lastRealtimeEvent && (
            <p className="mt-2 font-mono text-xs text-muted-foreground">
              Live event: {lastRealtimeEvent}
            </p>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {canPlan && (
            <Button onClick={() => setPlanOpen(true)}>Plan route</Button>
          )}
          <Button
            variant="secondary"
            onClick={exportCsv}
            isLoading={exporting}
            loadingText="Exporting…"
          >
            Export CSV
          </Button>
        </div>
      </div>
      {exportError && (
        <p role="alert" className="text-sm text-destructive">
          {exportError}
        </p>
      )}

      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <KpiCompact kpis={kpis} />
        <MapPanel>
          <LiveFleetMap
            trips={trips}
            destinations={destinations}
            selectedTripId={selectedTripId}
            showRoutes
            focusKey="dashboard"
            onSelectTrip={setSelectedTripId}
            onReady={() => {}}
          />
        </MapPanel>
      </div>

      <section aria-label="Needs attention">
        <SectionHeading
          icon={TriangleAlert}
          title="Needs attention"
          description="Exceptions that need someone to act, with links to fix them."
        />
        <ExceptionQueue
          expiringDrivers={expiringDrivers}
          openMaintenance={openMaintenance}
          draftTrips={draftTrips}
          draftCount={draftCount}
        />
      </section>

      {canPlan && (
        <PlanRouteDrawer
          open={planOpen}
          destinations={destinations}
          onClose={() => setPlanOpen(false)}
          onCreated={(trip) => {
            setTrips((previous) => [trip, ...previous])
            setSelectedTripId(trip.id)
            setPlanOpen(false)
          }}
        />
      )}
    </div>
  )
}
