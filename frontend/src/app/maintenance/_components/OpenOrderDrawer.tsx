"use client"

import { useEffect, useState } from "react"
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
import type { ShopVehicle } from "./types"

export type OpenOrderInput = {
  vehicle_reg: string
  description: string
  cost: number
  start_date: string
}

interface OpenOrderDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  vehicles: ShopVehicle[]
  saving: boolean
  onSubmit: (input: OpenOrderInput) => void
}

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

export function OpenOrderDrawer({
  open,
  onOpenChange,
  vehicles,
  saving,
  onSubmit,
}: OpenOrderDrawerProps) {
  const [vehicleReg, setVehicleReg] = useState("")
  const [description, setDescription] = useState("")
  const [cost, setCost] = useState("0.00")
  const [startDate, setStartDate] = useState(today())

  useEffect(() => {
    if (open) {
      setVehicleReg("")
      setDescription("")
      setCost("0.00")
      setStartDate(today())
    }
  }, [open ])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!vehicleReg) return
    onSubmit({
      vehicle_reg: vehicleReg,
      description: description.trim(),
      cost: Number(cost),
      start_date: startDate || today(),
    })
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent aria-label="Open work order">
        <DrawerHeader>
          <DrawerTitle>Open work order</DrawerTitle>
          <DrawerDescription>
            Pull a vehicle into the shop. It returns to service when the order
            closes.
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          <form id="open-order-form" onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label
                htmlFor="open-order-vehicle"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                Vehicle
              </label>
              <Select value={vehicleReg} onValueChange={setVehicleReg} required>
                <SelectTrigger id="open-order-vehicle" className="w-full">
                  <SelectValue placeholder="Select a vehicle" />
                </SelectTrigger>
                <SelectContent>
                  {vehicles.length === 0 ? (
                    <SelectItem value="none" disabled>
                      No eligible vehicles
                    </SelectItem>
                  ) : (
                    vehicles.map((v) => (
                      <SelectItem key={v.registration_number} value={v.registration_number}>
                        {v.registration_number} · {v.model} · {v.status}
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label
                htmlFor="open-order-description"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                Description
              </label>
              <Input
                id="open-order-description"
                required
                minLength={5}
                placeholder="e.g. Brake replacement, oil service"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <label
                  htmlFor="open-order-cost"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  Estimated cost
                </label>
                <Input
                  id="open-order-cost"
                  required
                  type="number"
                  min="0"
                  step="0.01"
                  value={cost}
                  onChange={(e) => setCost(e.target.value)}
                />
              </div>
              <div>
                <label
                  htmlFor="open-order-start"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  Start date
                </label>
                <Input
                  id="open-order-start"
                  required
                  type="date"
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                />
              </div>
            </div>
          </form>
        </DrawerBody>
        <DrawerFooter>
          <Button
            variant="secondary"
            disabled={saving}
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            form="open-order-form"
            isLoading={saving}
            loadingText="Opening"
          >
            Open work order
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}
