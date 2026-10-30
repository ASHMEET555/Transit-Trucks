"use client"

import { useEffect, useState } from "react"
import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
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
import { formatNumber, type Trip } from "./types"

interface CompleteDialogProps {
  trip: Trip | null
  onClose: () => void
  onCompleted: () => void
}

export function CompleteDialog({ trip, onClose, onCompleted }: CompleteDialogProps) {
  const [finalOdometer, setFinalOdometer] = useState("")
  const [fuelLiters, setFuelLiters] = useState("")
  const [fuelCost, setFuelCost] = useState("")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (trip) {
      setFinalOdometer("")
      setFuelLiters("")
      setFuelCost("")
      setSaving(false)
      setError(null)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- reset keyed on trip identity, not object
  }, [trip?.id])

  async function confirmComplete() {
    if (!trip) return
    const final = Number(finalOdometer)
    const start = trip.start_odometer ?? null
    if (!finalOdometer.trim() || Number.isNaN(final)) {
      setError("Enter the final odometer reading from the vehicle.")
      return
    }
    if (start !== null && final <= start) {
      setError(
        `Final reading must be higher than the start reading (${formatNumber(start)}). Check the vehicle odometer and try again.`,
      )
      return
    }
    const litersGiven = fuelLiters.trim() !== ""
    const costGiven = fuelCost.trim() !== ""
    if (litersGiven !== costGiven) {
      setError("Fuel liters and fuel cost must be entered together, or left both empty.")
      return
    }
    const liters = litersGiven ? Number(fuelLiters) : undefined
    const cost = costGiven ? Number(fuelCost) : undefined
    if (liters !== undefined && (Number.isNaN(liters) || liters <= 0)) {
      setError("Fuel liters must be greater than zero.")
      return
    }
    if (cost !== undefined && (Number.isNaN(cost) || cost <= 0)) {
      setError("Fuel cost must be greater than zero.")
      return
    }
    setSaving(true)
    setError(null)
    try {
      const body: { final_odometer: number; fuel_liters?: number; fuel_cost?: number } = {
        final_odometer: final,
      }
      if (liters !== undefined && cost !== undefined) {
        body.fuel_liters = liters
        body.fuel_cost = cost
      }
      await apiRequest(`/trips/${trip.id}/complete`, {
        method: "POST",
        body,
        actionId: `complete-${trip.id}`,
      })
      onCompleted()
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        setError(
          `${err.code ? `[${err.code}] ` : ""}${err.detail} Correct the reading, then try again.`,
        )
      } else {
        setError("Completion failed. Check your connection, then try again.")
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Drawer open={trip !== null} onOpenChange={(open) => !open && onClose()}>
      <DrawerContent aria-label={trip ? `Complete trip ${trip.id}` : "Complete trip"}>
        <DrawerHeader>
          <DrawerTitle>
            {trip ? `Complete trip #${trip.id}` : "Complete trip"}
          </DrawerTitle>
          <DrawerDescription>
            {trip?.start_odometer !== null && trip?.start_odometer !== undefined
              ? `Start reading is ${formatNumber(trip.start_odometer)}. The final reading must be higher.`
              : "Enter the final odometer reading from the vehicle."}
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          <div className="space-y-4">
            <div>
              <label htmlFor="complete-odometer" className="mb-1.5 block text-sm font-medium text-foreground">
                Final odometer *
              </label>
              <Input
                id="complete-odometer"
                type="number"
                min="0"
                inputMode="decimal"
                placeholder="e.g. 128400"
                value={finalOdometer}
                onChange={(event) => setFinalOdometer(event.target.value)}
              />
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label htmlFor="complete-liters" className="mb-1.5 block text-sm font-medium text-foreground">
                  Fuel liters (optional)
                </label>
                <Input
                  id="complete-liters"
                  type="number"
                  min="0"
                  inputMode="decimal"
                  placeholder="e.g. 45.5"
                  value={fuelLiters}
                  onChange={(event) => setFuelLiters(event.target.value)}
                />
              </div>
              <div>
                <label htmlFor="complete-cost" className="mb-1.5 block text-sm font-medium text-foreground">
                  Fuel cost (optional)
                </label>
                <Input
                  id="complete-cost"
                  type="number"
                  min="0"
                  inputMode="decimal"
                  placeholder="e.g. 110.0"
                  value={fuelCost}
                  onChange={(event) => setFuelCost(event.target.value)}
                />
              </div>
            </div>
            <p className="text-xs text-muted-foreground">
              Fuel liters and fuel cost are saved together as a fuel expense,
              or leave both empty to skip.
            </p>
            {error && (
              <div
                role="alert"
                className="rounded-2xl border border-destructive/30 bg-destructive/12 p-3 text-sm text-foreground"
              >
                {error}
              </div>
            )}
          </div>
        </DrawerBody>
        <DrawerFooter>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Back
          </Button>
          <Button
            onClick={confirmComplete}
            isLoading={saving}
            loadingText="Completing"
            disabled={!trip}
          >
            Confirm completion
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}
