"use client"

import { useCallback, useEffect, useState } from "react"
import {
  RiFileListLine,
  RiLockLine,
  RiMoneyDollarCircleLine,
  RiRoadMapLine,
  RiTimeLine,
  RiUserLine,
} from "@remixicon/react"
import { Button } from "@/components/Button"
import { Skeleton } from "@/components/Skeleton"
import {
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "@/components/Drawer"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { SectionIcon } from "./chrome"
import { StatusBadge } from "./StatusBadge"
import {
  formatDateTime,
  formatNumber,
  type AuditEntry,
  type ExpenseRow,
  type Trip,
} from "./types"

interface TripDetailDrawerProps {
  tripId: number | null
  role: string
  onClose: () => void
}

type SectionState<T> =
  | { kind: "idle" | "loading" }
  | { kind: "error"; message: string; forbidden: boolean }
  | { kind: "ready"; data: T }

const EXPENSE_ROLES = ["Admin", "Fleet Manager", "Financial Analyst"]

function errorState(err: unknown, fallback: string): Extract<SectionState<never>, { kind: "error" }> {
  if (err instanceof ApiError && err.status === 403) {
    return {
      kind: "error",
      forbidden: true,
      message: "Your role cannot view this section. Ask an administrator for access if you need it.",
    }
  }
  const message =
    err instanceof ApiError
      ? `${err.code ? `[${err.code}] ` : ""}${err.detail}`
      : fallback
  return { kind: "error", forbidden: false, message }
}

function SectionSkeleton() {
  return (
    <div className="space-y-2" aria-label="Loading section">
      <Skeleton className="h-4 w-2/3 motion-reduce:animate-none" />
      <Skeleton className="h-4 w-1/2 motion-reduce:animate-none" />
    </div>
  )
}

function SectionError({
  message,
  forbidden,
  onRetry,
}: {
  message: string
  forbidden: boolean
  onRetry: () => void
}) {
  return (
    <div
      role="alert"
      className="rounded-2xl border border-border bg-muted p-3 text-sm"
    >
      <p className="flex items-start gap-2 text-foreground">
        {forbidden && (
          <RiLockLine className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
        )}
        <span>{message}</span>
      </p>
      {!forbidden && (
        <Button variant="secondary" onClick={onRetry} className="mt-2">
          Retry
        </Button>
      )}
    </div>
  )
}

export function TripDetailDrawer({ tripId, role, onClose }: TripDetailDrawerProps) {
  const [detail, setDetail] = useState<SectionState<Trip>>({ kind: "idle" })
  const [expenses, setExpenses] = useState<SectionState<ExpenseRow[]>>({ kind: "idle" })
  const [audit, setAudit] = useState<SectionState<AuditEntry[]>>({ kind: "idle" })

  const loadDetail = useCallback(() => {
    if (tripId === null) return
    setDetail({ kind: "loading" })
    apiRequest<Trip>(`/trips/${tripId}`)
      .then((data) => setDetail({ kind: "ready", data }))
      .catch((err) =>
        setDetail(errorState(err, "Trip details failed to load. Check your connection, then retry.")),
      )
  }, [tripId])

  const loadExpenses = useCallback(() => {
    if (tripId === null) return
    if (!EXPENSE_ROLES.includes(role)) {
      setExpenses({
        kind: "error",
        forbidden: true,
        message: "Your role cannot view expenses. Ask an administrator for access if you need it.",
      })
      return
    }
    setExpenses({ kind: "loading" })
    apiRequest<unknown>(`/expenses?trip_id=${tripId}`)
      .then((data) => setExpenses({ kind: "ready", data: extractItems<ExpenseRow>(data) }))
      .catch((err) =>
        setExpenses(errorState(err, "Expenses failed to load. Check your connection, then retry.")),
      )
  }, [tripId, role])

  const loadAudit = useCallback(() => {
    if (tripId === null) return
    if (role !== "Admin") {
      setAudit({
        kind: "error",
        forbidden: true,
        message: "The audit log is limited to administrators. The trip lifecycle below covers the key events.",
      })
      return
    }
    setAudit({ kind: "loading" })
    apiRequest<unknown>(`/audit?entity=trips&entity_id=${tripId}&limit=50`)
      .then((data) => setAudit({ kind: "ready", data: extractItems<AuditEntry>(data) }))
      .catch((err) =>
        setAudit(errorState(err, "Audit entries failed to load. Check your connection, then retry.")),
      )
  }, [tripId, role])

  useEffect(() => {
    if (tripId === null) {
      setDetail({ kind: "idle" })
      setExpenses({ kind: "idle" })
      setAudit({ kind: "idle" })
      return
    }
    loadDetail()
    loadExpenses()
    loadAudit()
  }, [tripId, loadDetail, loadExpenses, loadAudit])

  const trip = detail.kind === "ready" ? detail.data : null
  const lifecycle: { label: string; at: string | null | undefined }[] = trip
    ? [
        { label: "Trip created", at: trip.created_at },
        { label: "Last update", at: trip.updated_at },
        ...(trip.completed_at ? [{ label: "Trip completed", at: trip.completed_at }] : []),
      ]
    : []

  return (
    <Drawer open={tripId !== null} onOpenChange={(open) => !open && onClose()}>
      <DrawerContent aria-label={tripId !== null ? `Details for trip ${tripId}` : "Trip details"}>
        <DrawerHeader>
          <DrawerTitle>
            {tripId !== null ? `Trip #${tripId}` : "Trip details"}
          </DrawerTitle>
          <DrawerDescription>
            {trip
              ? `${trip.source_name} to ${trip.destination_name}`
              : "Loading trip details."}
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          {detail.kind === "loading" && <SectionSkeleton />}
          {detail.kind === "error" && (
            <SectionError
              message={detail.message}
              forbidden={detail.forbidden}
              onRetry={loadDetail}
            />
          )}
          {trip && (
            <div className="space-y-6">
              <div className="flex items-center gap-2">
                <StatusBadge status={trip.status} />
                <span className="text-sm tabular-nums text-muted-foreground">
                  ${formatNumber(trip.revenue)} · {formatNumber(trip.planned_distance)} km planned
                </span>
              </div>

              <section aria-label="Assignment">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
                  <SectionIcon icon={RiUserLine} />
                  Assignment
                </h3>
                <dl className="mt-2 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
                  <div className="rounded-2xl border border-border bg-muted px-3 py-2">
                    <dt className="text-xs text-muted-foreground">Vehicle</dt>
                    <dd className="font-serif text-foreground">
                      {trip.vehicle_reg}
                    </dd>
                  </div>
                  <div className="rounded-2xl border border-border bg-muted px-3 py-2">
                    <dt className="text-xs text-muted-foreground">Driver</dt>
                    <dd className="font-mono tabular-nums text-foreground">
                      {trip.driver_license}
                    </dd>
                  </div>
                  <div className="rounded-2xl border border-border bg-muted px-3 py-2">
                    <dt className="text-xs text-muted-foreground">Cargo</dt>
                    <dd className="tabular-nums text-foreground">
                      {formatNumber(trip.cargo_weight)} kg
                    </dd>
                  </div>
                  <div className="rounded-2xl border border-border bg-muted px-3 py-2">
                    <dt className="text-xs text-muted-foreground">Revenue</dt>
                    <dd className="tabular-nums text-foreground">
                      ${formatNumber(trip.revenue)}
                    </dd>
                  </div>
                </dl>
              </section>

              <section aria-label="Odometer chain">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
                  <SectionIcon icon={RiRoadMapLine} />
                  Odometer chain
                </h3>
                {trip.start_odometer === null || trip.start_odometer === undefined ? (
                  <p className="mt-2 text-sm text-muted-foreground">
                    {trip.status === "Draft"
                      ? "The start reading is recorded when the trip is dispatched."
                      : "No start reading recorded yet."}
                  </p>
                ) : (
                  <ol className="mt-2 space-y-2 text-sm">
                    <li className="flex items-center justify-between gap-2 rounded-2xl border border-border bg-muted px-3 py-2">
                      <span className="text-muted-foreground">Start</span>
                      <span className="font-mono tabular-nums text-foreground">
                        {formatNumber(trip.start_odometer)}
                      </span>
                    </li>
                    <li className="flex items-center justify-between gap-2 rounded-2xl border border-border bg-muted px-3 py-2">
                      <span className="text-muted-foreground">Final</span>
                      <span className="font-mono tabular-nums text-foreground">
                        {trip.final_odometer ?? "Pending"}
                      </span>
                    </li>
                    {trip.final_odometer !== null && trip.final_odometer !== undefined && (
                      <li className="flex items-center justify-between gap-2 rounded-2xl border border-border bg-card px-3 py-2">
                        <span className="text-muted-foreground">Distance travelled</span>
                        <span className="font-mono tabular-nums text-foreground">
                          {formatNumber(trip.final_odometer - trip.start_odometer)} km
                        </span>
                      </li>
                    )}
                  </ol>
                )}
              </section>

              <section aria-label="Expenses">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
                  <SectionIcon icon={RiMoneyDollarCircleLine} />
                  Expenses
                </h3>
                <div className="mt-2">
                  {expenses.kind === "loading" && <SectionSkeleton />}
                  {expenses.kind === "error" && (
                    <SectionError
                      message={expenses.message}
                      forbidden={expenses.forbidden}
                      onRetry={loadExpenses}
                    />
                  )}
                  {expenses.kind === "ready" &&
                    (expenses.data.length === 0 ? (
                      <p className="text-sm text-muted-foreground">
                        No expenses logged for this trip yet. Fuel logged at
                        completion appears here.
                      </p>
                    ) : (
                      <ul className="space-y-2">
                        {expenses.data.map((expense) => (
                          <li
                            key={expense.id}
                            className="flex items-center justify-between gap-2 rounded-2xl border border-border bg-muted px-3 py-2 text-sm"
                          >
                            <span>
                              <span className="font-medium text-foreground">
                                {expense.type}
                              </span>{" "}
                              <span className="text-muted-foreground">
                                · {expense.date}
                                {expense.liters !== null && expense.liters !== undefined &&
                                  ` · ${formatNumber(expense.liters)} L`}
                              </span>
                            </span>
                            <span className="font-mono tabular-nums text-foreground">
                              ${(expense.amount ?? expense.cost).toLocaleString()}
                            </span>
                          </li>
                        ))}
                      </ul>
                    ))}
                </div>
              </section>

              <section aria-label="Timeline">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
                  <SectionIcon icon={audit.kind === "ready" ? RiFileListLine : RiTimeLine} />
                  Timeline
                </h3>
                <div className="mt-2">
                  {audit.kind === "loading" && <SectionSkeleton />}
                  {audit.kind === "error" && !audit.forbidden && (
                    <SectionError
                      message={audit.message}
                      forbidden={false}
                      onRetry={loadAudit}
                    />
                  )}
                  {(audit.kind === "ready" || (audit.kind === "error" && audit.forbidden)) && (
                    <div className="space-y-3">
                      {audit.kind === "error" && (
                        <p className="flex items-start gap-2 rounded-2xl bg-muted p-3 text-sm text-muted-foreground">
                          <RiLockLine className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                          <span>{audit.message}</span>
                        </p>
                      )}
                      {audit.kind === "ready" && audit.data.length > 0 && (
                        <ol className="space-y-2">
                          {audit.data.map((entry) => (
                            <li
                              key={entry.id}
                              className="rounded-2xl border border-border bg-muted px-3 py-2 text-sm"
                            >
                              <p className="font-medium text-foreground">
                                {entry.action}
                              </p>
                              <p className="text-xs text-muted-foreground">
                                {entry.actor_email || entry.actor || "System"} ·{" "}
                                <time dateTime={entry.at}>{formatDateTime(entry.at)}</time>
                              </p>
                            </li>
                          ))}
                        </ol>
                      )}
                      <ol className="space-y-2" aria-label="Trip lifecycle">
                        {lifecycle.map((item) => (
                          <li
                            key={item.label}
                            className="flex items-center justify-between gap-2 rounded-2xl bg-muted px-3 py-2 text-sm"
                          >
                            <span className="text-foreground">{item.label}</span>
                            <span className="text-xs tabular-nums text-muted-foreground">
                              {item.at ? (
                                <time dateTime={item.at}>{formatDateTime(item.at)}</time>
                              ) : (
                                "—"
                              )}
                            </span>
                          </li>
                        ))}
                      </ol>
                    </div>
                  )}
                </div>
              </section>
            </div>
          )}
        </DrawerBody>
      </DrawerContent>
    </Drawer>
  )
}
