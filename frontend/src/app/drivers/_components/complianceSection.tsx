"use client"

import { useMemo, useState } from "react"
import { Clock, ShieldAlert, ShieldCheck } from "lucide-react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
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
import { ApiError, apiRequest } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import type { RealtimeStatus } from "@/lib/realtime/useFleetEvents"
import { CardHead, DotPill, SectionIcon } from "./chrome"
import {
  canModerateDriver,
  conflictCopy,
  expiryStateOf,
  plusDaysKey,
  safetyBand,
  todayKey,
  type DriverRow,
} from "./driverTypes"

type ExpiryFilter = "attention" | "all" | "expired" | "expiring"

interface ComplianceSectionProps {
  drivers: DriverRow[]
  loading: boolean
  error: string | null
  realtimeStatus?: RealtimeStatus
  onReload: () => void
}

function expiryPill(expiry: "expired" | "expiring" | "valid"): { dot: string; pill: string; label: string } {
  if (expiry === "expired") {
    return { dot: "bg-destructive", pill: "bg-destructive/12 text-destructive", label: "Expired" }
  }
  if (expiry === "expiring") {
    return { dot: "bg-amber-600", pill: "bg-sand text-foreground", label: "Expiring soon" }
  }
  return { dot: "bg-chart-5", pill: "bg-sage text-foreground", label: "Valid" }
}

export function ComplianceSection({ drivers, loading, error, realtimeStatus, onReload }: ComplianceSectionProps) {
  const { currentProfile } = useProfile()
  const [filter, setFilter] = useState<ExpiryFilter>("attention")
  const [actioning, setActioning] = useState<string | null>(null)
  const [notice, setNotice] = useState<{ kind: "success" | "error"; text: string } | null>(null)

  const canModerate = canModerateDriver(currentProfile?.role)
  const today = todayKey()
  const horizon = plusDaysKey(30)

  const withExpiry = useMemo(
    () =>
      drivers.map((d) => ({
        driver: d,
        expiry: expiryStateOf(d.license_expiry_date, today, horizon),
      })),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [drivers],
  )

  const expiredCount = withExpiry.filter((r) => r.expiry === "expired").length
  const expiringCount = withExpiry.filter((r) => r.expiry === "expiring").length
  const validCount = withExpiry.filter((r) => r.expiry === "valid").length

  const visible = withExpiry.filter((r) => {
    if (filter === "all") return true
    if (filter === "expired") return r.expiry === "expired"
    if (filter === "expiring") return r.expiry === "expiring"
    return r.expiry !== "valid"
  })

  async function setStatus(license: string, action: "suspend" | "reinstate") {
    setActioning(license)
    setNotice(null)
    try {
      await apiRequest(`/fleet/drivers/${encodeURIComponent(license)}/${action}`, {
        method: "POST",
      })
      setNotice({
        kind: "success",
        text: action === "suspend" ? `Driver ${license} suspended.` : `Driver ${license} reinstated as available.`,
      })
      onReload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setNotice({ kind: "error", text: conflictCopy(action, err.detail) })
      } else {
        const detail = err instanceof ApiError ? err.detail : `Failed to ${action} driver`
        setNotice({ kind: "error", text: `${detail} Check the record and try again.` })
      }
    } finally {
      setActioning(null)
    }
  }

  return (
    <CardShell as="section" aria-labelledby="compliance-heading">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <CardHead
          title="Licence compliance"
          meta={`${expiredCount} expired · ${expiringCount} expiring within 30 days`}
        />
        {realtimeStatus && <RealtimeIndicator status={realtimeStatus} />}
      </div>

      <div className="mt-4">
        <StatPills
          items={[
            { label: "Expired", value: expiredCount, icon: ShieldAlert, tone: "signal" },
            { label: "Expiring soon", value: expiringCount, icon: Clock, tone: "primary" },
            { label: "Valid", value: validCount, icon: ShieldCheck, tone: "sage" },
          ]}
        />
      </div>

      <div className="mt-4 rounded-2xl border border-border bg-muted p-3 sm:p-4">
        <label
          htmlFor="compliance-expiry-filter"
          className="mb-1.5 block text-sm font-medium"
        >
          Licence expiry filter
        </label>
        <Select value={filter} onValueChange={(v) => setFilter(v as ExpiryFilter)}>
          <SelectTrigger id="compliance-expiry-filter" className="w-full sm:max-w-64">
            <SelectValue placeholder="Expiry filter" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="attention">Needs attention</SelectItem>
            <SelectItem value="all">All licences</SelectItem>
            <SelectItem value="expired">Expired only</SelectItem>
            <SelectItem value="expiring">Expiring within 30 days</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="mt-4">
        {notice && (
          <p
            role={notice.kind === "error" ? "alert" : "status"}
            className={
              notice.kind === "error"
                ? "mb-4 rounded-2xl border border-destructive/30 bg-destructive/12 px-4 py-3 text-sm text-foreground"
                : "mb-4 rounded-2xl border border-border bg-muted px-4 py-3 text-sm text-foreground"
            }
          >
            {notice.text}
          </p>
        )}

        {loading ? (
          <div role="status" aria-label="Loading compliance data">
            <Skeleton className="mb-2 h-10 w-full motion-reduce:animate-none" />
            <Skeleton className="mb-2 h-10 w-full motion-reduce:animate-none" />
            <Skeleton className="h-10 w-full motion-reduce:animate-none" />
            <span className="sr-only">Loading compliance data…</span>
          </div>
        ) : error && drivers.length === 0 ? (
          <div role="alert" className="rounded-2xl border border-destructive/30 bg-destructive/12 px-4 py-3 text-sm text-foreground">
            <p>{error}</p>
            <p className="mt-1 text-muted-foreground">Check your connection, then try again.</p>
            <Button variant="secondary" className="mt-3" onClick={onReload}>
              Try again
            </Button>
          </div>
        ) : (
          <>
            {error && (
              <p role="alert" className="mb-4 text-sm text-muted-foreground">
                Refresh failed: {error}{" "}
                <Button variant="ghost" className="px-1 py-0 text-sm underline" onClick={onReload}>
                  Try again
                </Button>
              </p>
            )}
            {visible.length === 0 ? (
              <div className="py-8 text-center">
                <p className="text-sm font-medium text-foreground">
                  {filter === "all"
                    ? "No drivers recorded yet."
                    : "No licences need attention."}
                </p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {filter === "all"
                    ? "Add a driver from the registry above to start tracking compliance."
                    : "All recorded licences are valid beyond the next 30 days."}
                </p>
                {filter !== "all" && (
                  <Button variant="secondary" className="mt-4" onClick={() => setFilter("all")}>
                    Show all licences
                  </Button>
                )}
              </div>
            ) : (
              <>
                <div className="overflow-x-auto">
                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableHeaderCell>Driver</TableHeaderCell>
                        <TableHeaderCell>Licence</TableHeaderCell>
                        <TableHeaderCell>Expiry</TableHeaderCell>
                        <TableHeaderCell>Safety score</TableHeaderCell>
                        <TableHeaderCell>Status</TableHeaderCell>
                        {canModerate && <TableHeaderCell>Action</TableHeaderCell>}
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {visible.map(({ driver: d, expiry }) => {
                        const pill = expiryPill(expiry)
                        return (
                          <TableRow key={d.license_number}>
                            <TableCell className="font-serif font-medium text-foreground">
                              {d.name}
                            </TableCell>
                            <TableCell className="font-mono tabular-nums">{d.license_number}</TableCell>
                            <TableCell className={expiry === "expired" ? "font-semibold text-destructive" : undefined}>
                              <span className="flex flex-wrap items-center gap-2">
                                <span>{d.license_expiry_date || "Not recorded"}</span>
                                <DotPill dot={pill.dot} pill={pill.pill}>
                                  {pill.label}
                                </DotPill>
                              </span>
                            </TableCell>
                            <TableCell className="tabular-nums">
                              {Number(d.safety_score).toFixed(1)} · {safetyBand(Number(d.safety_score))}
                            </TableCell>
                            <TableCell>{d.status}</TableCell>
                            {canModerate && (
                              <TableCell>
                                {d.status === "Suspended" ? (
                                  <Button
                                    variant="secondary"
                                    isLoading={actioning === d.license_number}
                                    loadingText="Reinstating"
                                    onClick={() => setStatus(d.license_number, "reinstate")}
                                  >
                                    Reinstate
                                  </Button>
                                ) : (
                                  <Button
                                    variant="secondary"
                                    isLoading={actioning === d.license_number}
                                    loadingText="Suspending"
                                    disabled={d.status === "On Trip"}
                                    title={
                                      d.status === "On Trip"
                                        ? "Drivers on a trip cannot be suspended until the trip ends"
                                        : undefined
                                    }
                                    onClick={() => setStatus(d.license_number, "suspend")}
                                  >
                                    Suspend
                                  </Button>
                                )}
                              </TableCell>
                            )}
                          </TableRow>
                    )
                    })}
                  </TableBody>
                </Table>
              </div>
              <p className="mt-3 flex items-start gap-2 text-xs text-muted-foreground">
                <SectionIcon icon={ShieldCheck} className="size-7 [&_svg]:size-3.5" />
                <span>
                  Safety bands are short labels for the recorded score, which remains the source value.
                  {!canModerate &&
                    " Suspend and reinstate actions are limited to Safety Officer and Admin roles."}
                </span>
              </p>
            </>
          )}
        </>
      )}
      </div>
    </CardShell>
  )
}
