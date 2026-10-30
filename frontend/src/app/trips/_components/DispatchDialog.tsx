"use client"

import { useEffect, useState } from "react"
import { RiErrorWarningLine } from "@remixicon/react"
import { Button } from "@/components/Button"
import { Skeleton } from "@/components/Skeleton"
import {
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/Drawer"
import { ApiError, apiRequest } from "@/lib/api"
import {
  formatNumber,
  ineligibilityLabel,
  type SuggestResponse,
  type Trip,
} from "./types"

interface DispatchDialogProps {
  trip: Trip | null
  onClose: () => void
  onDispatched: () => void
}

type SuggestState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "ready"; data: SuggestResponse }

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    return err.code ? `[${err.code}] ${err.detail}` : err.detail
  }
  return fallback
}

export function DispatchDialog({ trip, onClose, onDispatched }: DispatchDialogProps) {
  const [suggest, setSuggest] = useState<SuggestState>({ kind: "idle" })
  const [dispatching, setDispatching] = useState(false)
  const [dispatchError, setDispatchError] = useState<string | null>(null)

  useEffect(() => {
    if (!trip) return
    let cancelled = false
    setSuggest({ kind: "loading" })
    setDispatchError(null)
    setDispatching(false)
    apiRequest<SuggestResponse>("/trips/suggest", {
      method: "POST",
      body: {
        cargo_weight: trip.cargo_weight,
        source_name: trip.source_name,
        destination_name: trip.destination_name,
      },
    })
      .then((data) => {
        if (!cancelled) setSuggest({ kind: "ready", data })
      })
      .catch((err) => {
        if (!cancelled) setSuggest({ kind: "error", message: errorMessage(err, "Could not load suggestions.") })
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- suggest keyed on trip identity, not object
  }, [trip?.id])

  async function confirmDispatch() {
    if (!trip) return
    setDispatching(true)
    setDispatchError(null)
    try {
      await apiRequest(`/trips/${trip.id}/dispatch`, {
        method: "POST",
        actionId: `dispatch-${trip.id}`,
      })
      onDispatched()
      onClose()
    } catch (err) {
      setDispatchError(
        `${errorMessage(err, "Dispatch failed.")} Fix the assignment or availability issue, then try again.`,
      )
    } finally {
      setDispatching(false)
    }
  }

  const ranked =
    suggest.kind === "ready" ? (suggest.data.ranked_pairs ?? []).slice(0, 5) : []
  const ineligible = suggest.kind === "ready" ? (suggest.data.ineligible ?? []) : []
  const matchesTop =
    suggest.kind === "ready" &&
    suggest.data.suggested_vehicle_reg === trip?.vehicle_reg &&
    suggest.data.suggested_driver_license === trip?.driver_license

  return (
    <Drawer open={trip !== null} onOpenChange={(open) => !open && onClose()}>
      <DrawerContent aria-label={trip ? `Dispatch trip ${trip.id}` : "Dispatch trip"}>
        <DrawerHeader>
          <DrawerTitle>
            {trip ? `Dispatch trip #${trip.id}` : "Dispatch trip"}
          </DrawerTitle>
          <DrawerDescription>
            {trip
              ? `${trip.source_name} to ${trip.destination_name}, ${formatNumber(trip.cargo_weight)} kg. Review the suggested assignment before dispatching.`
              : "Review the suggested assignment before dispatching."}
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          {trip && suggest.kind === "loading" && (
            <div className="space-y-2" aria-label="Loading suggestions">
              <Skeleton className="h-16 w-full motion-reduce:animate-none" />
              <Skeleton className="h-10 w-full motion-reduce:animate-none" />
              <Skeleton className="h-10 w-full motion-reduce:animate-none" />
            </div>
          )}
          {suggest.kind === "error" && (
            <div
              role="alert"
              className="rounded-2xl border border-destructive/30 bg-destructive/12 p-4 text-sm text-foreground"
            >
              <p className="font-medium">Suggestions failed to load.</p>
              <p className="mt-1">
                {suggest.message} Check your connection, then try again. You
                can still dispatch with the current assignment.
              </p>
              <Button
                variant="secondary"
                onClick={() => {
                  if (!trip) return
                  setSuggest({ kind: "loading" })
                  apiRequest<SuggestResponse>("/trips/suggest", {
                    method: "POST",
                    body: {
                      cargo_weight: trip.cargo_weight,
                      source_name: trip.source_name,
                      destination_name: trip.destination_name,
                    },
                  })
                    .then((data) => setSuggest({ kind: "ready", data }))
                    .catch((err) =>
                      setSuggest({ kind: "error", message: errorMessage(err, "Could not load suggestions.") }),
                    )
                }}
                className="mt-3"
              >
                Retry suggestions
              </Button>
            </div>
          )}
          {suggest.kind === "ready" && trip && (
            <div className="space-y-4">
              <div className="rounded-2xl border border-border bg-muted p-3 text-sm">
                <p className="font-medium text-foreground">
                  Current assignment
                </p>
                <p className="mt-1 font-mono tabular-nums text-foreground">
                  {trip.vehicle_reg} · {trip.driver_license}
                </p>
                {matchesTop ? (
                  <p className="mt-1 text-muted-foreground">
                    This matches the top suggestion.
                  </p>
                ) : suggest.data.suggested_vehicle_reg ? (
                  <p className="mt-1 text-muted-foreground">
                    Top suggestion:{" "}
                    <span className="font-mono tabular-nums">
                      {suggest.data.suggested_vehicle_reg} ·{" "}
                      {suggest.data.suggested_driver_name} (
                      {suggest.data.suggested_driver_license})
                    </span>{" "}
                    · utilization{" "}
                    {(suggest.data.capacity_utilization * 100).toFixed(1)}%.
                  </p>
                ) : (
                  <p className="mt-1 text-muted-foreground">
                    No eligible vehicle and driver pair was found for this
                    cargo.
                  </p>
                )}
              </div>

              {ranked.length > 0 && (
                <section aria-label="Suggested assignments">
                  <h3 className="text-sm font-semibold text-foreground">
                    Suggested assignments
                  </h3>
                  <ul className="mt-2 space-y-2">
                    {ranked.map((pair) => (
                      <li
                        key={`${pair.vehicle_reg}-${pair.driver_license}`}
                        className="rounded-2xl border border-border bg-muted px-3 py-2 text-sm"
                      >
                        <span className="font-serif text-foreground">
                          {pair.vehicle_reg}
                        </span>{" "}
                        <span className="text-muted-foreground">
                          + {pair.driver_name} ({pair.driver_license})
                        </span>
                        <span className="mt-0.5 block text-xs tabular-nums text-muted-foreground">
                          Utilization {(pair.capacity_utilization * 100).toFixed(1)}% · score {pair.score.toFixed(2)}
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              )}

              {ineligible.length > 0 && (
                <section aria-label="Unavailable candidates">
                  <h3 className="text-sm font-semibold text-foreground">
                    Why others are unavailable ({ineligible.length})
                  </h3>
                  <ul className="mt-2 max-h-44 space-y-1.5 overflow-y-auto pr-1">
                    {ineligible.map((candidate) => (
                      <li
                        key={`${candidate.type}-${candidate.id}`}
                        className="flex items-start gap-2 rounded-2xl bg-muted px-3 py-1.5 text-sm"
                      >
                        <RiErrorWarningLine
                          className="mt-0.5 size-4 shrink-0 text-muted-foreground"
                          aria-hidden="true"
                        />
                        <span>
                          <span className="font-mono tabular-nums text-foreground">
                            {candidate.id}
                          </span>{" "}
                          <span className="text-muted-foreground">
                            — {ineligibilityLabel(candidate.reason)}
                          </span>
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              )}
            </div>
          )}
          {dispatchError && (
            <div
              role="alert"
              className="mt-4 rounded-2xl border border-destructive/30 bg-destructive/12 p-4 text-sm text-foreground"
            >
              {dispatchError}
            </div>
          )}
        </DrawerBody>
        <DrawerFooter>
          <Button variant="secondary" onClick={onClose} disabled={dispatching}>
            Keep as draft
          </Button>
          <Button
            onClick={confirmDispatch}
            isLoading={dispatching}
            loadingText="Dispatching"
            disabled={!trip}
          >
            Dispatch trip
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}
