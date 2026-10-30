"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { CheckCircle2, FileText, Truck, XCircle } from "lucide-react"
import { RiAddLine, RiErrorWarningLine, RiLockLine } from "@remixicon/react"
import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
import { StatPills } from "@/components/live-map/stat-pills"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite, hasRouteAccess } from "@/lib/rbacConfig"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { CompleteDialog } from "./_components/CompleteDialog"
import { CreateTripDialog } from "./_components/CreateTripDialog"
import { DispatchDialog } from "./_components/DispatchDialog"
import { TripBoard } from "./_components/TripBoard"
import { TripDetailDrawer } from "./_components/TripDetailDrawer"
import { TripTable } from "./_components/TripTable"
import {
  TRIP_STATUSES,
  type DestinationOption,
  type DriverOption,
  type Trip,
  type VehicleOption,
} from "./_components/types"

function listErrorMessage(err: unknown): string {
  const detail =
    err instanceof ApiError
      ? `${err.code ? `[${err.code}] ` : ""}${err.detail}`
      : "Trips failed to load."
  return `${detail} Check your connection, then try again.`
}

export default function TripsPage() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role ?? ""
  const canManage = canWrite(role, "trips")
  const canAccess = hasRouteAccess(role, "trips")

  const [trips, setTrips] = useState<Trip[]>([])
  const [vehicles, setVehicles] = useState<VehicleOption[]>([])
  const [drivers, setDrivers] = useState<DriverOption[]>([])
  const [destinations, setDestinations] = useState<DestinationOption[]>([])
  const [listState, setListState] = useState<"loading" | "ready" | "error">("loading")
  const [listError, setListError] = useState<string | null>(null)

  const [view, setView] = useState<"board" | "table">("table")
  const [search, setSearch] = useState("")
  const [statusFilter, setStatusFilter] = useState<string>("All")

  const [createOpen, setCreateOpen] = useState(false)
  const [dispatchTrip, setDispatchTrip] = useState<Trip | null>(null)
  const [completeTrip, setCompleteTrip] = useState<Trip | null>(null)
  const [detailId, setDetailId] = useState<number | null>(null)
  const [confirmCancelId, setConfirmCancelId] = useState<number | null>(null)
  const [cancellingId, setCancellingId] = useState<number | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [lastTripEvent, setLastTripEvent] = useState<string | null>(null)

  const reloadTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const load = useCallback(async (silent = false) => {
    if (!silent) {
      setListState("loading")
    }
    setListError(null)
    try {
      const [tripData, vehicleData, driverData, destinationData] = await Promise.all([
        apiRequest<unknown>("/trips/"),
        apiRequest<unknown>("/fleet/vehicles"),
        apiRequest<unknown>("/fleet/drivers"),
        apiRequest<unknown>("/destinations/"),
      ])
      setTrips(extractItems<Trip>(tripData))
      setVehicles(extractItems<VehicleOption>(vehicleData))
      setDrivers(extractItems<DriverOption>(driverData))
      setDestinations(extractItems<DestinationOption>(destinationData))
      setListState("ready")
    } catch (err) {
      if (!silent) {
        setListError(listErrorMessage(err))
        setListState("error")
      }
    }
  }, [])

  useEffect(() => {
    load(false)
  }, [load])

  useEffect(() => {
    return () => {
      if (reloadTimer.current) clearTimeout(reloadTimer.current)
    }
  }, [])

  const scheduleReload = useCallback(() => {
    if (reloadTimer.current) clearTimeout(reloadTimer.current)
    reloadTimer.current = setTimeout(() => load(true), 500)
  }, [load])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: (event) => {
      if (event.type.startsWith("trip.")) {
        setLastTripEvent(event.type)
        scheduleReload()
      }
    },
    onReconnect: () => scheduleReload(),
  })

  const canActOnTrip = useCallback(
    (trip: Trip) =>
      canManage ||
      (role === "Driver" &&
        Boolean(currentProfile?.driver_license) &&
        currentProfile?.driver_license === trip.driver_license),
    [canManage, role, currentProfile?.driver_license],
  )

  const scopedTrips = useMemo(() => {
    if (role === "Driver" && currentProfile?.driver_license) {
      return trips.filter((trip) => trip.driver_license === currentProfile.driver_license)
    }
    return trips
  }, [trips, role, currentProfile?.driver_license])

  const filteredTrips = useMemo(() => {
    const query = search.trim().toLowerCase()
    return scopedTrips.filter((trip) => {
      if (statusFilter !== "All" && trip.status !== statusFilter) return false
      if (!query) return true
      return (
        String(trip.id).includes(query) ||
        trip.source_name.toLowerCase().includes(query) ||
        trip.destination_name.toLowerCase().includes(query) ||
        trip.vehicle_reg.toLowerCase().includes(query) ||
        trip.driver_license.toLowerCase().includes(query)
      )
    })
  }, [scopedTrips, search, statusFilter])

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = { Draft: 0, Dispatched: 0, Completed: 0, Cancelled: 0 }
    for (const trip of scopedTrips) {
      if (trip.status in counts) counts[trip.status] += 1
    }
    return counts
  }, [scopedTrips])

  async function cancelTrip(id: number) {
    setCancellingId(id)
    setActionError(null)
    try {
      await apiRequest(`/trips/${id}/cancel`, {
        method: "POST",
        actionId: `cancel-${id}`,
      })
      setConfirmCancelId(null)
      await load(true)
    } catch (err) {
      setActionError(
        `${listErrorMessage(err)} The trip was not cancelled.`,
      )
    } finally {
      setCancellingId(null)
    }
  }

  function renderActions(trip: Trip) {
    if (!canActOnTrip(trip)) return null
    if (trip.status === "Draft") {
      return (
        <>
          <Button variant="secondary" onClick={() => setDispatchTrip(trip)}>
            Dispatch
          </Button>
          {confirmCancelId === trip.id ? (
            <>
              <Button
                variant="destructive"
                onClick={() => cancelTrip(trip.id)}
                disabled={cancellingId === trip.id}
              >
                {cancellingId === trip.id ? "Cancelling…" : "Confirm cancel"}
              </Button>
              <Button variant="ghost" onClick={() => setConfirmCancelId(null)}>
                Keep
              </Button>
            </>
          ) : (
            <Button variant="ghost" onClick={() => setConfirmCancelId(trip.id)}>
              Cancel
            </Button>
          )}
        </>
      )
    }
    if (trip.status === "Dispatched") {
      return (
        <>
          <Button variant="secondary" onClick={() => setCompleteTrip(trip)}>
            Complete
          </Button>
          {confirmCancelId === trip.id ? (
            <>
              <Button
                variant="destructive"
                onClick={() => cancelTrip(trip.id)}
                disabled={cancellingId === trip.id}
              >
                {cancellingId === trip.id ? "Cancelling…" : "Confirm cancel"}
              </Button>
              <Button variant="ghost" onClick={() => setConfirmCancelId(null)}>
                Keep
              </Button>
            </>
          ) : (
            <Button variant="ghost" onClick={() => setConfirmCancelId(trip.id)}>
              Cancel
            </Button>
          )}
        </>
      )
    }
    return null
  }

  if (!isLoaded) {
    return (
      <div className="p-6" aria-label="Loading trips page">
        <TripTable
          trips={[]}
          loading
          onOpen={() => {}}
          renderActions={() => null}
          emptyAction={null}
        />
      </div>
    )
  }

  if (!currentProfile) {
    return (
      <div className="flex min-h-96 flex-col items-center justify-center gap-3 p-6 text-center">
        <RiLockLine className="size-8 text-muted-foreground" aria-hidden="true" />
        <h1 className="text-xl font-semibold text-foreground">
          Sign in required
        </h1>
        <p className="max-w-sm text-sm text-muted-foreground">
          Your session expired. Sign in again to view trips.
        </p>
        <Button asChild>
          <a href="/login">Go to sign in</a>
        </Button>
      </div>
    )
  }

  if (!canAccess) {
    return (
      <div className="flex min-h-96 flex-col items-center justify-center gap-3 p-6 text-center">
        <RiLockLine className="size-8 text-muted-foreground" aria-hidden="true" />
        <h1 className="text-xl font-semibold text-foreground">
          Trips are unavailable for your role
        </h1>
        <p className="max-w-sm text-sm text-muted-foreground">
          Your role ({role || "unknown"}) has no access to trips. Ask an
          administrator to update your permissions.
        </p>
      </div>
    )
  }

  const newTripAction = canManage ? (
    <Button onClick={() => setCreateOpen(true)}>
      <RiAddLine className="size-4" aria-hidden="true" />
      New trip
    </Button>
  ) : null

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">Trips</h1>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Dispatch and trip lifecycle
            {role === "Driver" && " · Showing only trips assigned to your license"}
          </p>
          <span className="sr-only" role="status">
            {lastTripEvent ? `Updated after ${lastTripEvent}.` : ""}
          </span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div role="group" aria-label="Change layout" className="inline-flex rounded-full border border-border bg-muted p-1">
            <button
              type="button"
              aria-pressed={view === "board"}
              onClick={() => setView("board")}
              className={`rounded-full px-3 py-1.5 text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
                view === "board"
                  ? "bg-card text-foreground shadow-sm ring-1 ring-foreground/5"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              Board
            </button>
            <button
              type="button"
              aria-pressed={view === "table"}
              onClick={() => setView("table")}
              className={`rounded-full px-3 py-1.5 text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
                view === "table"
                  ? "bg-card text-foreground shadow-sm ring-1 ring-foreground/5"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              Table
            </button>
          </div>
          {newTripAction}
        </div>
      </div>

      <StatPills
        items={[
          { label: "Draft", value: statusCounts.Draft, icon: FileText, tone: "mist" },
          { label: "Dispatched", value: statusCounts.Dispatched, icon: Truck, tone: "sage" },
          { label: "Completed", value: statusCounts.Completed, icon: CheckCircle2, tone: "primary" },
          { label: "Cancelled", value: statusCounts.Cancelled, icon: XCircle, tone: "signal" },
        ]}
      />

      {(listError || actionError) && (
        <div
          role="alert"
          className="rounded-2xl border border-destructive/30 bg-destructive/12 p-4 text-sm text-foreground"
        >
          <p className="flex items-start gap-2 font-medium">
            <RiErrorWarningLine className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <span>{listError || actionError}</span>
          </p>
          <Button variant="secondary" onClick={() => load(false)} className="mt-2">
            Retry
          </Button>
        </div>
      )}

      <div className="grid grid-cols-1 gap-3 rounded-2xl border border-border bg-muted p-3 sm:grid-cols-[1fr_220px] sm:p-4">
        <Input
          type="search"
          aria-label="Search trips"
          placeholder="Search by id, route, plate, or license"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger aria-label="Filter by status">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="All">All statuses</SelectItem>
            {TRIP_STATUSES.map((status) => (
              <SelectItem key={status} value={status}>
                {status}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <p className="text-sm tabular-nums text-muted-foreground" role="status">
        {listState === "loading"
          ? "Loading trips…"
          : `${filteredTrips.length} ${filteredTrips.length === 1 ? "trip" : "trips"}`}
      </p>

      {listState === "error" && !listError ? (
        <div className="rounded-2xl border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">
            Trips failed to load. Check your connection, then retry.
          </p>
          <Button variant="secondary" onClick={() => load(false)} className="mt-3">
            Retry
          </Button>
        </div>
      ) : view === "board" ? (
        <TripBoard
          trips={filteredTrips}
          loading={listState === "loading"}
          visibleStatuses={statusFilter === "All" ? [...TRIP_STATUSES] : [statusFilter]}
          onOpen={(trip) => setDetailId(trip.id)}
          renderActions={renderActions}
          draftEmptyAction={newTripAction}
        />
      ) : (
        <TripTable
          trips={filteredTrips}
          loading={listState === "loading"}
          onOpen={(trip) => setDetailId(trip.id)}
          renderActions={renderActions}
          emptyAction={newTripAction}
        />
      )}

      <CreateTripDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => load(true)}
        destinations={destinations}
        vehicles={vehicles}
        drivers={drivers}
        refsLoading={listState === "loading"}
      />
      <DispatchDialog
        trip={dispatchTrip}
        onClose={() => setDispatchTrip(null)}
        onDispatched={() => load(true)}
      />
      <CompleteDialog
        trip={completeTrip}
        onClose={() => setCompleteTrip(null)}
        onCompleted={() => load(true)}
      />
      <TripDetailDrawer tripId={detailId} role={role} onClose={() => setDetailId(null)} />
    </div>
  )
}
