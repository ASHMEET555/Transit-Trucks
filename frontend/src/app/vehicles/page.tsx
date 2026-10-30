"use client"

import React, { useState, useEffect } from "react"
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite } from "@/lib/rbacConfig"

/** UI form shape (teammate labels) — mapped to backend on save/load */
type FleetForm = {
  registration_number: string
  name: string // maps to backend `model`
  type: string
  capacity: number // maps to backend `max_load_capacity`
  odometer: number
  acquisition_cost: number
  status: string
}

type VehicleRow = {
  registration_number: string
  name: string
  type: string
  capacity: number
  odometer: number
  acquisition_cost: number
  status: string
}

type ApiVehicle = {
  registration_number: string
  model: string
  type: string
  max_load_capacity: number
  odometer: number
  acquisition_cost: number
  status: string
}

function fromApi(v: ApiVehicle): VehicleRow {
  return {
    registration_number: v.registration_number,
    name: v.model,
    type: v.type,
    capacity: v.max_load_capacity,
    odometer: v.odometer,
    acquisition_cost: v.acquisition_cost,
    status: v.status,
  }
}

function toApi(form: FleetForm) {
  return {
    registration_number: form.registration_number,
    model: form.name,
    type: form.type,
    max_load_capacity: form.capacity,
    odometer: form.odometer,
    acquisition_cost: form.acquisition_cost,
  }
}

const emptyForm: FleetForm = {
  registration_number: "",
  name: "",
  type: "Van",
  capacity: 1000,
  odometer: 0,
  acquisition_cost: 0,
  status: "Available",
}

export default function FleetPage() {
  const { currentProfile } = useProfile()
  const [vehicles, setVehicles] = useState<VehicleRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)

  const [statusFilter, setStatusFilter] = useState<string>("All")
  const [typeFilter, setTypeFilter] = useState<string>("All")
  const [capacityMin, setCapacityMin] = useState<number>(0)
  const [capacityMax, setCapacityMax] = useState<number>(20000)
  const [capacityCeiling, setCapacityCeiling] = useState<number>(20000)

  const [formData, setFormData] = useState<FleetForm>(emptyForm)

  const canAddVehicle = canWrite(currentProfile?.role || "", "fleet")

  useEffect(() => {
    fetchVehicles()
  }, [])

  const fetchVehicles = async () => {
    try {
      setError(null)
      const res = await apiRequest<any>("/fleet/vehicles")
      const data = extractItems<ApiVehicle>(res)
      const rows = data.map(fromApi)
      setVehicles(rows)
      const highest = Math.max(20000, ...rows.map((v) => v.capacity), 0)
      setCapacityCeiling(highest)
      setCapacityMin(0)
      setCapacityMax(highest)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to fetch vehicles")
    } finally {
      setLoading(false)
    }
  }

  const handleAddVehicle = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!canAddVehicle) return

    try {
      setError(null)
      await apiRequest("/fleet/vehicles", {
        method: "POST",
        body: toApi(formData),
      })
      await fetchVehicles()
      setShowAddForm(false)
      setFormData(emptyForm)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to add vehicle")
    }
  }

  const handleDeleteVehicle = async (regNum: string) => {
    if (!canAddVehicle) return
    if (!confirm("Are you sure you want to delete this vehicle?")) return

    try {
      setError(null)
      await apiRequest(`/fleet/vehicles/${encodeURIComponent(regNum)}`, {
        method: "DELETE",
      })
      await fetchVehicles()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to delete vehicle")
    }
  }

  const filteredVehicles = vehicles.filter((vehicle) => {
    if (statusFilter !== "All" && vehicle.status !== statusFilter) return false
    if (typeFilter !== "All" && vehicle.type !== typeFilter) return false
    if (vehicle.capacity < capacityMin || vehicle.capacity > capacityMax) return false
    return true
  })

  const getStatusColor = (status: string) => {
    switch (status) {
      case "Available":
        return "bg-primary/10 text-primary border border-primary/20"
      case "On Trip":
        return "bg-signal/10 text-signal border border-signal/20"
      case "In Shop":
        return "bg-destructive/10 text-destructive border border-destructive/20"
      case "Retired":
        return "bg-neutral-100 text-neutral-600 border border-neutral-200"
      default:
        return "bg-neutral-100 text-neutral-800"
    }
  }

  const handleRetireVehicle = async (regNum: string) => {
    if (!canAddVehicle) return
    if (!confirm(`Retire vehicle ${regNum}? Retired vehicles cannot be dispatched.`)) return

    try {
      setError(null)
      await apiRequest(`/fleet/vehicles/${encodeURIComponent(regNum)}/retire`, {
        method: "POST",
      })
      await fetchVehicles()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to retire vehicle")
    }
  }

  const typeOptions = Array.from(
    new Set(["Truck", "Van", "Pickup", ...vehicles.map((v) => v.type)]),
  )

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border/60 pb-5">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">Vehicles Registry</h1>
          <p className="text-xs text-muted-foreground mt-1">
            Commercial vehicle asset records, cargo limits, odometer tracking, and service status.
          </p>
        </div>
        {canAddVehicle && (
          <Button
            onClick={() => setShowAddForm(!showAddForm)}
            className="bg-primary text-primary-foreground hover:bg-primary/90"
          >
            {showAddForm ? "Close Form" : "Add New Vehicle"}
          </Button>
        )}
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-950">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              <span className="mr-2 text-red-600 dark:text-red-400">⚠</span>
              <p className="text-sm font-medium text-red-900 dark:text-red-100">
                {error}
              </p>
            </div>
            <button
              onClick={() => setError(null)}
              className="text-sm font-medium text-red-700 hover:text-red-900 dark:text-red-300 dark:hover:text-red-100"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {/* Filters — original layout */}
      <div className="mb-6 flex flex-wrap gap-4 rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-950">
        <div className="min-w-[200px] flex-1">
          <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
            Status
          </label>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All Statuses</SelectItem>
              <SelectItem value="Available">Available</SelectItem>
              <SelectItem value="On Trip">On Trip</SelectItem>
              <SelectItem value="In Shop">In Shop</SelectItem>
              <SelectItem value="Retired">Retired</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="min-w-[200px] flex-1">
          <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
            Vehicle Type
          </label>
          <Select value={typeFilter} onValueChange={setTypeFilter}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All Types</SelectItem>
              {typeOptions.map((t) => (
                <SelectItem key={t} value={t}>
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="min-w-[320px] flex-[1.5]">
          <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
            Capacity (kg): {capacityMin.toLocaleString()} –{" "}
            {capacityMax.toLocaleString()}
          </label>
          <div className="mb-3 flex gap-3">
            <div className="flex-1">
              <span className="mb-1 block text-xs text-gray-500">Min</span>
              <Input
                type="number"
                min={0}
                max={capacityCeiling}
                value={capacityMin}
                onChange={(e) => {
                  const v = Math.max(0, Number(e.target.value) || 0)
                  setCapacityMin(Math.min(v, capacityMax))
                }}
              />
            </div>
            <div className="flex-1">
              <span className="mb-1 block text-xs text-gray-500">Max</span>
              <Input
                type="number"
                min={0}
                max={capacityCeiling}
                value={capacityMax}
                onChange={(e) => {
                  const v = Math.min(
                    capacityCeiling,
                    Math.max(0, Number(e.target.value) || 0),
                  )
                  setCapacityMax(Math.max(v, capacityMin))
                }}
              />
            </div>
          </div>
          <div className="relative h-8">
            <div className="absolute top-1/2 h-2 w-full -translate-y-1/2 rounded-lg bg-gray-200 dark:bg-gray-700" />
            <div
              className="absolute top-1/2 h-2 -translate-y-1/2 rounded-lg bg-blue-500"
              style={{
                left: `${(capacityMin / capacityCeiling) * 100}%`,
                width: `${((capacityMax - capacityMin) / capacityCeiling) * 100}%`,
              }}
            />
            <input
              type="range"
              min={0}
              max={capacityCeiling}
              step={100}
              value={capacityMin}
              onChange={(e) => {
                const newVal = Number(e.target.value)
                setCapacityMin(Math.min(newVal, capacityMax))
              }}
              className="pointer-events-none absolute inset-0 z-[1] h-8 w-full appearance-none bg-transparent [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:relative [&::-webkit-slider-thumb]:z-[2] [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-blue-500 [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:cursor-pointer [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-blue-500"
            />
            <input
              type="range"
              min={0}
              max={capacityCeiling}
              step={100}
              value={capacityMax}
              onChange={(e) => {
                const newVal = Number(e.target.value)
                setCapacityMax(Math.max(newVal, capacityMin))
              }}
              className="pointer-events-none absolute inset-0 z-[2] h-8 w-full appearance-none bg-transparent [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:relative [&::-webkit-slider-thumb]:z-[3] [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-blue-600 [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:cursor-pointer [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-blue-600"
            />
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Range ceiling: {capacityCeiling.toLocaleString()} kg (auto from fleet)
          </p>
        </div>
      </div>

      {/* Add New Vehicle — original form layout */}
      {showAddForm && canAddVehicle && (
        <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
          <h2 className="mb-4 text-lg font-semibold">Add New Vehicle</h2>
          <form
            onSubmit={handleAddVehicle}
            className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
          >
            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Registration Number *
              </label>
              <Input
                required
                value={formData.registration_number}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    registration_number: e.target.value,
                  })
                }
                placeholder="e.g., ABC-1234"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Vehicle Name *
              </label>
              <Input
                required
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="e.g., City Bus 01"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Vehicle Type *
              </label>
              <Select
                value={formData.type}
                onValueChange={(value) =>
                  setFormData({ ...formData, type: value })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Van">Van</SelectItem>
                  <SelectItem value="Box Truck">Box Truck</SelectItem>
                  <SelectItem value="Flatbed">Flatbed</SelectItem>
                  <SelectItem value="Semi">Semi</SelectItem>
                  <SelectItem value="Light Truck">Light Truck</SelectItem>
                  <SelectItem value="Medium Truck">Medium Truck</SelectItem>
                  <SelectItem value="Heavy Truck">Heavy Truck</SelectItem>
                  <SelectItem value="Refrigerated">Refrigerated</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Capacity (KG) *
              </label>
              <Input
                required
                type="number"
                min="0"
                value={formData.capacity || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    capacity: parseInt(e.target.value) || 0,
                  })
                }
                placeholder="Capacity in kg"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Odometer (km) *
              </label>
              <Input
                required
                type="number"
                min="0"
                value={formData.odometer || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    odometer: parseInt(e.target.value) || 0,
                  })
                }
                placeholder="Current odometer reading"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Acquisition Cost *
              </label>
              <Input
                required
                type="number"
                min="0"
                step="0.01"
                value={formData.acquisition_cost || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    acquisition_cost: parseFloat(e.target.value) || 0,
                  })
                }
                placeholder="Purchase cost"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Status *
              </label>
              <Select
                value={formData.status}
                onValueChange={(value) =>
                  setFormData({ ...formData, status: value })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Available">Available</SelectItem>
                  <SelectItem value="On Trip">On Trip</SelectItem>
                  <SelectItem value="In Shop">In Shop</SelectItem>
                  <SelectItem value="Retired">Retired</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-end md:col-span-2 lg:col-span-3">
              <div className="flex gap-2">
                <Button type="submit">Add Vehicle</Button>
                <Button
                  variant="secondary"
                  type="button"
                  onClick={() => setShowAddForm(false)}
                >
                  Cancel
                </Button>
              </div>
            </div>
          </form>
        </div>
      )}

      {/* Vehicles Table — original columns */}
      <div className="rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
        <Table>
          <TableHead>
            <TableRow>
              <TableHeaderCell>Registration Number</TableHeaderCell>
              <TableHeaderCell>Name</TableHeaderCell>
              <TableHeaderCell>Type</TableHeaderCell>
              <TableHeaderCell>Capacity (KG)</TableHeaderCell>
              <TableHeaderCell>Odometer (km)</TableHeaderCell>
              <TableHeaderCell>Acquisition Cost</TableHeaderCell>
              <TableHeaderCell>Status</TableHeaderCell>
              {canAddVehicle && <TableHeaderCell>Actions</TableHeaderCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center">
                  Loading vehicles...
                </TableCell>
              </TableRow>
            ) : filteredVehicles.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center">
                  No vehicles found
                </TableCell>
              </TableRow>
            ) : (
              filteredVehicles.map((vehicle) => (
                <TableRow key={vehicle.registration_number}>
                  <TableCell className="font-medium">
                    {vehicle.registration_number}
                  </TableCell>
                  <TableCell>{vehicle.name}</TableCell>
                  <TableCell>{vehicle.type}</TableCell>
                  <TableCell>{vehicle.capacity}</TableCell>
                  <TableCell>{vehicle.odometer.toLocaleString()}</TableCell>
                  <TableCell>
                    ${vehicle.acquisition_cost.toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <span
                      className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${getStatusColor(vehicle.status)}`}
                    >
                      {vehicle.status}
                    </span>
                  </TableCell>
                  {canAddVehicle && (
                    <TableCell>
                      <div className="flex gap-2">
                        {vehicle.status !== "Retired" && (
                          <Button
                            variant="secondary"
                            onClick={() => handleRetireVehicle(vehicle.registration_number)}
                          >
                            Retire
                          </Button>
                        )}
                        <Button
                          variant="destructive"
                          onClick={() =>
                            handleDeleteVehicle(vehicle.registration_number)
                          }
                        >
                          Delete
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
