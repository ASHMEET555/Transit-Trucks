"use client"

import { useState } from "react"
import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
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
  type DestinationOption,
  type DriverOption,
  type SuggestResponse,
  type VehicleOption,
} from "./types"

interface CreateTripDialogProps {
  open: boolean
  onClose: () => void
  onCreated: () => void
  destinations: DestinationOption[]
  vehicles: VehicleOption[]
  drivers: DriverOption[]
  refsLoading: boolean
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    return `${err.code ? `[${err.code}] ` : ""}${err.detail}`
  }
  return fallback
}

export function CreateTripDialog({
  open,
  onClose,
  onCreated,
  destinations,
  vehicles,
  drivers,
  refsLoading,
}: CreateTripDialogProps) {
  const [form, setForm] = useState({
    source_name: "",
    destination_name: "",
    vehicle_reg: "",
    driver_license: "",
    cargo_weight: "",
    planned_distance: "",
    revenue: "0",
  })
  const [dispatchNow, setDispatchNow] = useState(true)
  const [suggest, setSuggest] = useState<SuggestResponse | null>(null)
  const [suggesting, setSuggesting] = useState(false)
  const [suggestError, setSuggestError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const availableVehicles = vehicles.filter((v) => v.status === "Available")
  const today = new Date().toISOString().slice(0, 10)
  const availableDrivers = drivers.filter(
    (d) => d.status === "Available" && d.license_expiry_date > today,
  )
  const selectedVehicle = vehicles.find((v) => v.registration_number === form.vehicle_reg)
  const overloaded =
    selectedVehicle && form.cargo_weight
      ? Number(form.cargo_weight) > selectedVehicle.max_load_capacity
      : false

  function reset() {
    setForm({
      source_name: "",
      destination_name: "",
      vehicle_reg: "",
      driver_license: "",
      cargo_weight: "",
      planned_distance: "",
      revenue: "0",
    })
    setDispatchNow(true)
    setSuggest(null)
    setSuggestError(null)
    setSaving(false)
    setError(null)
  }

  async function suggestAssignment() {
    if (!form.cargo_weight || Number(form.cargo_weight) <= 0) {
      setSuggestError("Enter a cargo weight above zero first.")
      return
    }
    if (form.source_name && form.destination_name && form.source_name === form.destination_name) {
      setSuggestError("Source and destination must be different.")
      return
    }
    setSuggesting(true)
    setSuggestError(null)
    try {
      const data = await apiRequest<SuggestResponse>("/trips/suggest", {
        method: "POST",
        body: {
          cargo_weight: Number(form.cargo_weight),
          source_name: form.source_name || undefined,
          destination_name: form.destination_name || undefined,
        },
      })
      setSuggest(data)
      if (data.suggested_vehicle_reg) {
        setForm((prev) => ({
          ...prev,
          vehicle_reg: data.suggested_vehicle_reg,
          driver_license: data.suggested_driver_license,
        }))
      }
    } catch (err) {
      setSuggestError(`${errorMessage(err, "Suggestion failed.")} You can still pick a vehicle and driver manually.`)
    } finally {
      setSuggesting(false)
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    setError(null)
    try {
      await apiRequest("/trips/", {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
        body: {
          source_name: form.source_name,
          destination_name: form.destination_name,
          vehicle_reg: form.vehicle_reg,
          driver_license: form.driver_license,
          cargo_weight: Number(form.cargo_weight),
          planned_distance: Number(form.planned_distance),
          revenue: Number(form.revenue || "0"),
          dispatch: dispatchNow,
        },
      })
      reset()
      onCreated()
      onClose()
    } catch (err) {
      setError(
        `${errorMessage(err, "Creation failed.")} Check the highlighted requirements, then try again.`,
      )
    } finally {
      setSaving(false)
    }
  }

  const ineligible = (suggest?.ineligible ?? []).slice(0, 8)
  const ineligibleTotal = suggest?.ineligible?.length ?? 0

  return (
    <Drawer
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          reset()
          onClose()
        }
      }}
    >
      <DrawerContent aria-label="Create trip">
        <DrawerHeader>
          <DrawerTitle>Create trip</DrawerTitle>
          <DrawerDescription>
            Plan a draft or dispatch it right away. Suggestions rank the best
            available vehicle and driver for the cargo.
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          <form id="create-trip-form" onSubmit={submit} className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label htmlFor="create-source" className="mb-1.5 block text-sm font-medium text-foreground">
                  Source *
                </label>
                <Select
                  value={form.source_name}
                  onValueChange={(value) => setForm({ ...form, source_name: value })}
                  disabled={refsLoading}
                >
                  <SelectTrigger id="create-source">
                    <SelectValue placeholder="Select source" />
                  </SelectTrigger>
                  <SelectContent>
                    {destinations.map((destination) => (
                      <SelectItem key={destination.name} value={destination.name}>
                        {destination.name} ({destination.region})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label htmlFor="create-destination" className="mb-1.5 block text-sm font-medium text-foreground">
                  Destination *
                </label>
                <Select
                  value={form.destination_name}
                  onValueChange={(value) => setForm({ ...form, destination_name: value })}
                  disabled={refsLoading}
                >
                  <SelectTrigger id="create-destination">
                    <SelectValue placeholder="Select destination" />
                  </SelectTrigger>
                  <SelectContent>
                    {destinations.map((destination) => (
                      <SelectItem key={destination.name} value={destination.name}>
                        {destination.name} ({destination.region})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label htmlFor="create-cargo" className="mb-1.5 block text-sm font-medium text-foreground">
                  Cargo weight (kg) *
                </label>
                <Input
                  id="create-cargo"
                  required
                  type="number"
                  min="0"
                  inputMode="decimal"
                  placeholder="e.g. 450"
                  value={form.cargo_weight}
                  onChange={(event) => setForm({ ...form, cargo_weight: event.target.value })}
                />
              </div>
              <div>
                <label htmlFor="create-distance" className="mb-1.5 block text-sm font-medium text-foreground">
                  Planned distance (km) *
                </label>
                <Input
                  id="create-distance"
                  required
                  type="number"
                  min="0"
                  inputMode="decimal"
                  placeholder="e.g. 120"
                  value={form.planned_distance}
                  onChange={(event) => setForm({ ...form, planned_distance: event.target.value })}
                />
              </div>
            </div>

            <div>
              <label htmlFor="create-vehicle" className="mb-1.5 block text-sm font-medium text-foreground">
                Vehicle *
              </label>
              <Select
                value={form.vehicle_reg}
                onValueChange={(value) => setForm({ ...form, vehicle_reg: value })}
                disabled={refsLoading}
              >
                <SelectTrigger id="create-vehicle">
                  <SelectValue placeholder={refsLoading ? "Loading vehicles…" : "Select vehicle"} />
                </SelectTrigger>
                <SelectContent>
                  {availableVehicles.map((vehicle) => (
                    <SelectItem key={vehicle.registration_number} value={vehicle.registration_number}>
                      {vehicle.registration_number} · {vehicle.model} (
                      {formatNumber(vehicle.max_load_capacity)} kg)
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedVehicle && (
                <p className="mt-1 text-xs text-muted-foreground">
                  Max capacity: {formatNumber(selectedVehicle.max_load_capacity)} kg
                </p>
              )}
              {overloaded && (
                <p className="mt-1 text-xs font-medium text-foreground">
                  Cargo exceeds this vehicle capacity. Pick a larger vehicle or reduce the cargo.
                </p>
              )}
            </div>

            <div>
              <label htmlFor="create-driver" className="mb-1.5 block text-sm font-medium text-foreground">
                Driver *
              </label>
              <Select
                value={form.driver_license}
                onValueChange={(value) => setForm({ ...form, driver_license: value })}
                disabled={refsLoading}
              >
                <SelectTrigger id="create-driver">
                  <SelectValue placeholder={refsLoading ? "Loading drivers…" : "Select driver"} />
                </SelectTrigger>
                <SelectContent>
                  {availableDrivers.map((driver) => (
                    <SelectItem key={driver.license_number} value={driver.license_number}>
                      {driver.name} ({driver.license_number})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div>
              <label htmlFor="create-revenue" className="mb-1.5 block text-sm font-medium text-foreground">
                Revenue
              </label>
              <Input
                id="create-revenue"
                type="number"
                min="0"
                step="0.01"
                inputMode="decimal"
                value={form.revenue}
                onChange={(event) => setForm({ ...form, revenue: event.target.value })}
              />
            </div>

            <div className="flex items-center gap-2">
              <input
                id="create-dispatch-toggle"
                type="checkbox"
                checked={dispatchNow}
                onChange={(event) => setDispatchNow(event.target.checked)}
                className="h-4 w-4 rounded border-border accent-primary"
              />
              <label htmlFor="create-dispatch-toggle" className="text-sm font-medium text-foreground">
                Dispatch immediately (leave off to save as draft)
              </label>
            </div>

            <div className="rounded-2xl border border-border bg-muted p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm font-medium text-foreground">
                  Need a recommendation?
                </p>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={suggestAssignment}
                  isLoading={suggesting}
                  loadingText="Suggesting"
                >
                  Suggest assignment
                </Button>
              </div>
              {suggestError && (
                <p role="alert" className="mt-2 text-sm text-destructive">
                  {suggestError}
                </p>
              )}
              {suggest && !suggestError && (
                <div className="mt-2 text-sm text-foreground">
                  {suggest.suggested_vehicle_reg ? (
                    <p>
                      Suggested:{" "}
                      <span className="font-mono tabular-nums">
                        {suggest.suggested_vehicle_reg} · {suggest.suggested_driver_name} (
                        {suggest.suggested_driver_license})
                      </span>{" "}
                      · utilization {(suggest.capacity_utilization * 100).toFixed(1)}%.
                    </p>
                  ) : (
                    <p>No eligible vehicle and driver pair found for this cargo.</p>
                  )}
                  {ineligibleTotal > 0 && (
                    <ul className="mt-2 space-y-1">
                      {ineligible.map((candidate) => (
                        <li key={`${candidate.type}-${candidate.id}`} className="text-xs text-muted-foreground">
                          <span className="font-mono tabular-nums">{candidate.id}</span>
                          {" — "}
                          {ineligibilityLabel(candidate.reason)}
                        </li>
                      ))}
                      {ineligibleTotal > ineligible.length && (
                        <li className="text-xs text-muted-foreground">
                          And {ineligibleTotal - ineligible.length} more unavailable candidates.
                        </li>
                      )}
                    </ul>
                  )}
                </div>
              )}
            </div>

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
          <Button
            variant="secondary"
            onClick={() => {
              reset()
              onClose()
            }}
            disabled={saving}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            form="create-trip-form"
            isLoading={saving}
            loadingText="Saving"
            disabled={overloaded || refsLoading}
          >
            {dispatchNow ? "Dispatch trip" : "Save draft"}
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}
