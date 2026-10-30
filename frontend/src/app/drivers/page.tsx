"use client"

import { useEffect, useMemo, useState } from "react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
import { Pagination } from "@/components/Pagination"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { Skeleton } from "@/components/Skeleton"
import { ApiError, apiRequest } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite, hasRouteAccess } from "@/lib/rbacConfig"
import { CardHead } from "./_components/chrome"
import { ComplianceSection } from "./_components/complianceSection"
import { DriverFormDrawer, saveDriver, type DriverFormState } from "./_components/driverFormDrawer"
import { DriversTable } from "./_components/driversTable"
import { useDrivers } from "./_components/useDrivers"
import type { DriverRow } from "./_components/driverTypes"

export default function DriversPage() {
  const { currentProfile, isLoaded } = useProfile()
  const { drivers, loading, error, reload, realtimeStatus } = useDrivers()

  const PAGE_SIZE = 10
  const [page, setPage] = useState(1)

  const [statusFilter, setStatusFilter] = useState("All")
  const [sortSafetyScore, setSortSafetyScore] = useState<"asc" | "desc" | "none">("none")
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<DriverRow | null>(null)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [deletingLicense, setDeletingLicense] = useState<string | null>(null)

  const role = currentProfile?.role || ""
  const hasAccess = hasRouteAccess(role, "drivers")
  const canEdit = canWrite(role, "drivers")
  const canDelete = role === "Fleet Manager" || role === "Admin"

  const visibleDrivers = useMemo(() => {
    const filtered = drivers.filter((d) => statusFilter === "All" || d.status === statusFilter)
    if (sortSafetyScore === "asc") return [...filtered].sort((a, b) => a.safety_score - b.safety_score)
    if (sortSafetyScore === "desc") return [...filtered].sort((a, b) => b.safety_score - a.safety_score)
    return filtered
  }, [drivers, statusFilter, sortSafetyScore])

  useEffect(() => {
    setPage(1)
  }, [statusFilter, sortSafetyScore, drivers.length])

  const pagedDrivers = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return visibleDrivers.slice(start, start + PAGE_SIZE)
  }, [visibleDrivers, page])

  async function handleSave(form: DriverFormState, target: DriverRow | null) {
    setSaving(true)
    setFormError(null)
    try {
      await saveDriver(form, target)
      setDrawerOpen(false)
      setEditing(null)
      reload()
    } catch (err) {
      setFormError(err instanceof Error ? `${err.message} Check the details and try again.` : "Failed to save driver")
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(license: string) {
    if (!window.confirm(`Delete driver ${license}? This cannot be undone.`)) return
    setDeletingLicense(license)
    setFormError(null)
    try {
      await apiRequest(`/fleet/drivers/${encodeURIComponent(license)}`, { method: "DELETE" })
      reload()
    } catch (err) {
      setFormError(
        `${err instanceof ApiError ? err.detail : "Failed to delete driver"} Check the record and try again.`,
      )
    } finally {
      setDeletingLicense(null)
    }
  }

  if (isLoaded && !hasAccess) {
    return (
      <div className="mx-auto max-w-xl px-4 py-16 text-center">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Drivers</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Your role does not have access to driver records. Contact an administrator if you need access.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-4 p-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Drivers</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Driver records and licence compliance.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <RealtimeIndicator status={realtimeStatus} />
          {canEdit && (
            <Button
              onClick={() => {
                setEditing(null)
                setFormError(null)
                setDrawerOpen(true)
              }}
            >
              Add driver
            </Button>
          )}
        </div>
      </header>

      {formError && (
        <p role="alert" className="rounded-2xl border border-destructive/30 bg-destructive/12 px-4 py-3 text-sm text-foreground">
          {formError}
        </p>
      )}

      <div className="grid grid-cols-1 gap-3 rounded-2xl border border-border bg-muted p-3 sm:grid-cols-2 sm:p-4">
        <div>
          <label htmlFor="status-filter" className="mb-1.5 block text-sm font-medium">
            Status
          </label>
          <Select value={statusFilter} onValueChange={(v) => { setStatusFilter(v); setPage(1) }}>
            <SelectTrigger id="status-filter">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {["All", "Available", "On Trip", "Off Duty", "Suspended"].map((s) => (
                <SelectItem key={s} value={s}>
                  {s === "All" ? "All statuses" : s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label htmlFor="score-sort" className="mb-1.5 block text-sm font-medium">
            Sort by safety score
          </label>
          <Select value={sortSafetyScore} onValueChange={(v) => { setSortSafetyScore(v as "asc" | "desc" | "none"); setPage(1) }}>
            <SelectTrigger id="score-sort">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">No sorting</SelectItem>
              <SelectItem value="asc">Low to high</SelectItem>
              <SelectItem value="desc">High to low</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <CardShell as="section" aria-label="Driver registry">
        <CardHead
          title="Driver registry"
          meta={
            loading
              ? "Loading…"
              : `${visibleDrivers.length} ${visibleDrivers.length === 1 ? "driver" : "drivers"}`
          }
        />

        <div className="mt-4">
          {loading ? (
            <div role="status" aria-label="Loading drivers">
              <Skeleton className="mb-2 h-10 w-full motion-reduce:animate-none" />
              <Skeleton className="mb-2 h-10 w-full motion-reduce:animate-none" />
              <Skeleton className="h-10 w-full motion-reduce:animate-none" />
              <span className="sr-only">Loading drivers…</span>
            </div>
          ) : error && drivers.length === 0 ? (
            <div role="alert" className="rounded-2xl border border-destructive/30 bg-destructive/12 px-4 py-3 text-sm text-foreground">
              <p>{error}</p>
              <p className="mt-1 text-muted-foreground">Check your connection, then try again.</p>
              <Button variant="secondary" className="mt-3" onClick={reload}>
                Try again
              </Button>
            </div>
          ) : visibleDrivers.length === 0 && drivers.length === 0 ? (
            <div className="py-8 text-center">
              <p className="text-sm font-medium text-foreground">No drivers yet.</p>
              <p className="mt-1 text-sm text-muted-foreground">
                Add your first driver to start building the registry.
              </p>
              {canEdit && (
                <Button
                  className="mt-4"
                  onClick={() => {
                    setEditing(null)
                    setDrawerOpen(true)
                  }}
                >
                  Add driver
                </Button>
              )}
            </div>
          ) : visibleDrivers.length === 0 ? (
            <div className="py-8 text-center">
              <p className="text-sm font-medium text-foreground">No drivers match this filter.</p>
              <Button variant="secondary" className="mt-4" onClick={() => { setStatusFilter("All"); setPage(1) }}>
                Clear filter
              </Button>
            </div>
          ) : (
            <>
            <div className="overflow-x-auto">
              <DriversTable
                drivers={pagedDrivers}
                canEdit={canEdit}
                canDelete={canDelete}
                deletingLicense={deletingLicense}
                onEdit={(d) => {
                  setEditing(d)
                  setFormError(null)
                  setDrawerOpen(true)
                }}
                onDelete={handleDelete}
              />
            </div>
            <Pagination
              page={page}
              pageSize={PAGE_SIZE}
              total={visibleDrivers.length}
              onPageChange={setPage}
              id="drivers-registry-pagination"
            />
            </>
          )}
        </div>
      </CardShell>

      <ComplianceSection
        drivers={drivers}
        loading={loading}
        error={error}
        realtimeStatus={realtimeStatus}
        onReload={reload}
      />

      <DriverFormDrawer
        open={drawerOpen}
        driver={editing}
        saving={saving}
        onOpenChange={setDrawerOpen}
        onSubmit={handleSave}
      />
    </div>
  )
}
