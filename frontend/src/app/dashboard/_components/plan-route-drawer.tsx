"use client"

import { useState } from "react"
import { Button } from "@/components/Button"
import {
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/Drawer"
import { Input } from "@/components/Input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { ApiError, apiRequest } from "@/lib/api"
import type { Destination, Trip } from "@/lib/tripTypes"

type PlanRouteDrawerProps = {
  open: boolean
  destinations: Destination[]
  onClose: () => void
  onCreated: (trip: Trip) => void
}

export function PlanRouteDrawer({
  open,
  destinations,
  onClose,
  onCreated,
}: PlanRouteDrawerProps) {
  const [source, setSource] = useState("")
  const [destination, setDestination] = useState("")
  const [boarding, setBoarding] = useState("")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function reset() {
    setSource("")
    setDestination("")
    setBoarding("")
    setSaving(false)
    setError(null)
  }

  function close() {
    reset()
    onClose()
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (destinations.length === 0) {
      setError("No destinations available yet. The route was not planned.")
      return
    }
    if (!source) {
      setError("Select a source to start the route.")
      return
    }
    if (!destination) {
      setError("Select a destination to end the route.")
      return
    }
    if (source === destination) {
      setError("Source and destination must be different.")
      return
    }
    setSaving(true)
    setError(null)
    const boarding_points = boarding
      .split(/[\n,]+/)
      .map((stop) => stop.trim())
      .filter(Boolean)
    try {
      const trip = await apiRequest<Trip>("/trips/", {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
        body: {
          source_name: source,
          destination_name: destination,
          dispatch: false,
          ...(boarding_points.length > 0 ? { boarding_points } : {}),
        },
      })
      reset()
      onCreated(trip)
    } catch (err) {
      setError(
        `${err instanceof ApiError ? err.detail : "Plan route failed."} The draft was not created.`,
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(next) => {
        if (!next) close()
      }}
    >
      <DrawerContent aria-label="Plan route">
        <DrawerHeader>
          <DrawerTitle>Plan route</DrawerTitle>
          <DrawerDescription>
            Pick two stops to draft a trip. The new route shows on the map
            right away.
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          <form id="plan-route-form" onSubmit={submit} className="space-y-4">
            <div>
              <label
                htmlFor="plan-source"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                Source
              </label>
              <Select
                value={source}
                onValueChange={setSource}
                disabled={saving || destinations.length === 0}
              >
                <SelectTrigger id="plan-source">
                  <SelectValue placeholder="Select source" />
                </SelectTrigger>
                <SelectContent>
                  {destinations.map((stop) => (
                    <SelectItem key={stop.name} value={stop.name}>
                      {stop.name} ({stop.region})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label
                htmlFor="plan-destination"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                Destination
              </label>
              <Select
                value={destination}
                onValueChange={setDestination}
                disabled={saving || destinations.length === 0}
              >
                <SelectTrigger id="plan-destination">
                  <SelectValue placeholder="Select destination" />
                </SelectTrigger>
                <SelectContent>
                  {destinations.map((stop) => (
                    <SelectItem key={stop.name} value={stop.name}>
                      {stop.name} ({stop.region})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label
                htmlFor="plan-boarding"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                Boarding points{" "}
                <span className="font-normal text-muted-foreground">
                  (optional)
                </span>
              </label>
              <Input
                id="plan-boarding"
                placeholder="e.g. Gate 2, Weighbridge, Depot yard"
                value={boarding}
                onChange={(event) => setBoarding(event.target.value)}
                disabled={saving}
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Separate stops with commas. Stored with the draft trip.
              </p>
            </div>
            {destinations.length === 0 && (
              <p role="note" className="text-sm text-muted-foreground">
                No destinations available yet. Add one before planning a
                route.
              </p>
            )}
            {error && (
              <div
                role="alert"
                className="rounded-2xl border border-destructive/30 bg-destructive/12 p-3 text-sm text-foreground"
              >
                {error}
              </div>
            )}
          </form>
        </DrawerBody>
        <DrawerFooter>
          <Button variant="secondary" onClick={close} disabled={saving}>
            Cancel
          </Button>
          <Button
            type="submit"
            form="plan-route-form"
            isLoading={saving}
            loadingText="Planning"
            disabled={destinations.length === 0}
          >
            Save draft route
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}
