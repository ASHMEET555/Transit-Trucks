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
import { ApiError, apiRequest } from "@/lib/api"
import type { DriverRow } from "./driverTypes"

const emptyForm = {
  license_number: "",
  name: "",
  license_category: "Light Transport",
  license_expiry_date: "",
  contact_number: "",
  safety_score: 100,
  status: "Available",
}

type FormState = typeof emptyForm

interface DriverFormDrawerProps {
  open: boolean
  driver: DriverRow | null
  saving: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (form: FormState, editing: DriverRow | null) => void
}

const categories = ["Light Transport", "Heavy Transport", "Commercial", "Hazardous Materials"]
const statuses = ["Available", "On Trip", "Off Duty", "Suspended"]

export function DriverFormDrawer({ open, driver, saving, onOpenChange, onSubmit }: DriverFormDrawerProps) {
  const [form, setForm] = useState<FormState>(emptyForm)

  useEffect(() => {
    if (open) {
      setForm(
        driver
          ? {
              license_number: driver.license_number,
              name: driver.name,
              license_category: driver.license_category,
              license_expiry_date: driver.license_expiry_date,
              contact_number: driver.contact_number,
              safety_score: Number(driver.safety_score) || 0,
              status: driver.status,
            }
          : emptyForm,
      )
    }
  }, [open, driver])

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent aria-label={driver ? "Edit driver" : "Add driver"}>
        <DrawerHeader>
          <DrawerTitle>{driver ? "Edit driver" : "Add driver"}</DrawerTitle>
          <DrawerDescription>
            {driver
              ? `Update the record for licence ${driver.license_number}.`
              : "Register a new driver with licence and contact details."}
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          <form
            id="driver-form"
            className="grid grid-cols-1 gap-4"
            onSubmit={(e) => {
              e.preventDefault()
              onSubmit(form, driver)
            }}
          >
            <div>
              <label htmlFor="driver-license" className="mb-1 block text-sm font-medium">
                Licence number
              </label>
              <Input
                id="driver-license"
                required
                disabled={driver !== null || saving}
                value={form.license_number}
                onChange={(e) => set("license_number", e.target.value)}
                placeholder="e.g. DL123456789"
              />
            </div>
            <div>
              <label htmlFor="driver-name" className="mb-1 block text-sm font-medium">
                Name
              </label>
              <Input
                id="driver-name"
                required
                value={form.name}
                onChange={(e) => set("name", e.target.value)}
                placeholder="e.g. Jane Doe"
              />
            </div>
            <div>
              <label htmlFor="driver-category" className="mb-1 block text-sm font-medium">
                Licence category
              </label>
              <Select value={form.license_category} onValueChange={(v) => set("license_category", v)}>
                <SelectTrigger id="driver-category">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {categories.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label htmlFor="driver-expiry" className="mb-1 block text-sm font-medium">
                Licence expiry date
              </label>
              <Input
                id="driver-expiry"
                required
                type="date"
                value={form.license_expiry_date}
                onChange={(e) => set("license_expiry_date", e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="driver-contact" className="mb-1 block text-sm font-medium">
                Contact number
              </label>
              <Input
                id="driver-contact"
                required
                value={form.contact_number}
                onChange={(e) => set("contact_number", e.target.value)}
                placeholder="e.g. +1234567890"
              />
            </div>
            <div>
              <label htmlFor="driver-score" className="mb-1 block text-sm font-medium">
                Safety score
              </label>
              <Input
                id="driver-score"
                required
                type="number"
                min="0"
                max="100"
                step="0.1"
                value={String(form.safety_score)}
                onChange={(e) => set("safety_score", parseFloat(e.target.value) || 0)}
              />
            </div>
            <div>
              <label htmlFor="driver-status" className="mb-1 block text-sm font-medium">
                Status
              </label>
              <Select value={form.status} onValueChange={(v) => set("status", v)}>
                <SelectTrigger id="driver-status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {statuses.map((s) => (
                    <SelectItem key={s} value={s}>
                      {s}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </form>
        </DrawerBody>
        <DrawerFooter>
          <Button variant="secondary" type="button" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" form="driver-form" isLoading={saving} loadingText="Saving">
            {driver ? "Save changes" : "Add driver"}
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}

export async function saveDriver(form: FormState, editing: DriverRow | null): Promise<void> {
  try {
    if (editing) {
      await apiRequest(`/fleet/drivers/${encodeURIComponent(editing.license_number)}`, {
        method: "PUT",
        body: form,
      })
    } else {
      await apiRequest("/fleet/drivers", { method: "POST", body: form })
    }
  } catch (err) {
    throw new Error(err instanceof ApiError ? err.detail : "Failed to save driver")
  }
}

export type { FormState as DriverFormState }
