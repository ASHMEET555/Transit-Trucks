"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { CheckCircle2, Wallet, Wrench } from "lucide-react"
import { RiAddLine, RiAlertLine, RiDownloadLine, RiRefreshLine } from "@remixicon/react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
import { StatPills } from "@/components/live-map/stat-pills"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import { Skeleton } from "@/components/Skeleton"
import { BACKEND_URL } from "@/data/data"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite, hasRouteAccess } from "@/lib/rbacConfig"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { CardHead } from "./_components/chrome"
import { HistoryTable } from "./_components/HistoryTable"
import { OpenOrderDrawer, type OpenOrderInput } from "./_components/OpenOrderDrawer"
import { OpenQueue } from "./_components/OpenQueue"
import { isOpen, type MaintenanceLog, type ShopVehicle } from "./_components/types"

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

function LoadingState() {
  return (
    <div className="space-y-8" aria-label="Loading maintenance records">
      <div>
        <Skeleton className="h-5 w-40 motion-reduce:animate-none" />
        <div className="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-44 rounded-[28px] motion-reduce:animate-none" />
          ))}
        </div>
      </div>
      <div>
        <Skeleton className="h-5 w-32 motion-reduce:animate-none" />
        <Skeleton className="mt-3 h-64 rounded-[28px] motion-reduce:animate-none" />
      </div>
    </div>
  )
}

export default function MaintenancePage() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role || ""
  const canManage = canWrite(role, "maintenance")
  const forbidden = isLoaded && !hasRouteAccess(role, "maintenance")

  const [logs, setLogs] = useState<MaintenanceLog[]>([])
  const [vehicles, setVehicles] = useState<ShopVehicle[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [confirmId, setConfirmId] = useState<number | null>(null)
  const [closingId, setClosingId] = useState<number | null>(null)
  const [exporting, setExporting] = useState(false)
  const [liveEvent, setLiveEvent] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setError(null)
      const [m, v] = await Promise.all([
        apiRequest<any>("/maintenance/"),
        apiRequest<any>("/fleet/vehicles"),
      ])
      setLogs(extractItems<MaintenanceLog>(m))
      setVehicles(extractItems<ShopVehicle>(v))
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load maintenance records")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (isLoaded && !forbidden) {
      load()
    } else if (isLoaded && forbidden) {
      setLoading(false)
    }
  }, [isLoaded, forbidden, load])

  const { status } = useFleetEvents({
    onEvent: (event) => {
      if (event.type.startsWith("maintenance.")) {
        setLiveEvent(event.type)
        load()
      }
    },
    onReconnect: () => {
      load()
    },
  })

  const openLogs = useMemo(() => logs.filter(isOpen), [logs])
  const closedLogs = useMemo(() => logs.filter((log) => !isOpen(log)), [logs])
  const openCost = useMemo(
    () => openLogs.reduce((sum, log) => sum + Number(log.cost || 0), 0),
    [openLogs],
  )

  const eligibleVehicles = useMemo(
    () =>
      vehicles.filter((v) => v.status !== "On Trip" && v.status !== "Retired"),
    [vehicles],
  )

  async function handleOpenOrder(input: OpenOrderInput) {
    setSaving(true)
    try {
      setError(null)
      await apiRequest("/maintenance/", { method: "POST", body: input })
      setDrawerOpen(false)
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to open work order")
    } finally {
      setSaving(false)
    }
  }

  async function handleConfirmClose(id: number) {
    setClosingId(id)
    try {
      setError(null)
      await apiRequest(`/maintenance/${id}/close`, {
        method: "POST",
        body: { end_date: today() },
      })
      setConfirmId(null)
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to close work order")
    } finally {
      setClosingId(null)
    }
  }

  async function handleExportCsv() {
    setExporting(true)
    try {
      setError(null)
      const res = await fetch(`${BACKEND_URL}/dashboard/export.csv`, {
        credentials: "include",
      })
      if (!res.ok) throw new Error(`Export failed (${res.status})`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = "maintenance.csv"
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Export failed")
    } finally {
      setExporting(false)
    }
  }

  if (isLoaded && forbidden) {
    return (
      <div className="p-6">
        <CardShell as="section" className="mx-auto max-w-lg items-center p-8 text-center">
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Maintenance is not available for this role
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Your current role does not have access to maintenance records.
            Contact your fleet manager if you need access.
          </p>
        </CardShell>
      </div>
    )
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="flex items-center gap-3">
            <p className="text-sm text-muted-foreground">Workshop operations</p>
            <RealtimeIndicator status={status} />
          </div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-foreground">Maintenance</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Open work orders in the shop, and the history of completed work.
          </p>
          {liveEvent && (
            <p className="mt-2 font-mono text-xs tabular-nums text-muted-foreground">
              Live event: {liveEvent}
            </p>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            onClick={handleExportCsv}
            isLoading={exporting}
            loadingText="Exporting"
            disabled={loading || logs.length === 0}
          >
            <RiDownloadLine className="mr-1.5 size-4" aria-hidden="true" />
            Export CSV
          </Button>
          {canManage && (
            <Button onClick={() => setDrawerOpen(true)}>
              <RiAddLine className="mr-1.5 size-4" aria-hidden="true" />
              Open work order
            </Button>
          )}
        </div>
      </div>

      <StatPills
        items={[
          { label: "Open orders", value: openLogs.length, icon: Wrench, tone: "sage" },
          { label: "Closed orders", value: closedLogs.length, icon: CheckCircle2, tone: "primary" },
          {
            label: "Open estimates",
            value: `$${openCost.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
            icon: Wallet,
            tone: "mist",
          },
        ]}
      />

      {error && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-destructive/30 bg-destructive/12 p-4"
        >
          <p className="flex items-center gap-2 text-sm text-foreground">
            <RiAlertLine className="size-4 shrink-0" aria-hidden="true" />
            {error}
          </p>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={() => setError(null)}>
              Dismiss
            </Button>
            <Button
              variant="secondary"
              onClick={load}
              disabled={loading}
            >
              <RiRefreshLine className="mr-1.5 size-4" aria-hidden="true" />
              Retry
            </Button>
          </div>
        </div>
      )}

      {loading ? (
        <LoadingState />
      ) : (
        <>
          <section aria-labelledby="open-queue-heading">
            <div id="open-queue-heading">
              <CardHead
                title={`Open queue (${openLogs.length})`}
              />
            </div>
            <div className="mt-3">
            <OpenQueue
              logs={openLogs}
              canManage={canManage}
              confirmId={confirmId}
              closingId={closingId}
              onAskClose={setConfirmId}
              onCancelClose={() => setConfirmId(null)}
              onConfirmClose={handleConfirmClose}
              onOpenOrder={() => setDrawerOpen(true)}
            />
            </div>
          </section>

          <section aria-labelledby="history-heading">
            <h2 id="history-heading" className="sr-only">
              History ({closedLogs.length})
            </h2>
            <HistoryTable logs={closedLogs} />
          </section>
        </>
      )}

      {canManage && (
        <OpenOrderDrawer
          open={drawerOpen}
          onOpenChange={setDrawerOpen}
          vehicles={eligibleVehicles}
          saving={saving}
          onSubmit={handleOpenOrder}
        />
      )}
    </div>
  )
}
