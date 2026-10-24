"use client"

import { useEffect, useMemo, useState } from "react"
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
import { BACKEND_URL } from "@/data/data"

type Destination = { name: string; region: string }
type Vehicle = {
  registration_number: string
  model: string
  max_load_capacity: number
  status: string
  odometer: number
}
type Driver = {
  license_number: string
  name: string
  status: string
  license_expiry_date: string
}
type Trip = {
  id: number
  source_name: string
  destination_name: string
  vehicle_reg: string
  driver_license: string
  cargo_weight: number
  planned_distance: number
  revenue: number
  status: string
}

type TripSuggestion = {
  suggested_vehicle_reg: string
  suggested_driver_license: string
  suggested_driver_name: string
  capacity_utilization: number
  heuristic_score: number
}

export default function TripsPage() {
  const { currentProfile } = useProfile()
  const canManage = canWrite(currentProfile?.role || "", "trips")
  const [trips, setTrips] = useState<Trip[]>([])
  const [vehicles, setVehicles] = useState<Vehicle[]>([])
  const [drivers, setDrivers] = useState<Driver[]>([])
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [odometer, setOdometer] = useState("")
  const [completeId, setCompleteId] = useState<number | null>(null)
  const [sourceFilter, setSourceFilter] = useState("All")
  const [destinationFilter, setDestinationFilter] = useState("All")
  const [vehicleFilter, setVehicleFilter] = useState("All")
  const [driverFilter, setDriverFilter] = useState("All")
  const [liveEvent, setLiveEvent] = useState<string | null>(null)
  const [suggestion, setSuggestion] = useState<TripSuggestion | null>(null)
  const [suggesting, setSuggesting] = useState(false)
  const [form, setForm] = useState({
    source_name: "",
    destination_name: "",
    vehicle_reg: "",
    driver_license: "",
    cargo_weight: "",
    planned_distance: "",
    revenue: "0",
  })

  async function load() {
    try {
      setError(null)
      const [t, v, d, dest] = await Promise.all([
        apiRequest<any>("/trips/"),
        apiRequest<any>("/fleet/vehicles"),
        apiRequest<any>("/fleet/drivers"),
        apiRequest<any>("/destinations/"),
      ])
      setTrips(extractItems<Trip>(t))
      setVehicles(extractItems<Vehicle>(v))
      setDrivers(extractItems<Driver>(d))
      setDestinations(extractItems<Destination>(dest))
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load trips")
    }
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    const wsUrl = BACKEND_URL.replace("/api", "/ws/fleet").replace("http", "ws")
    const socket = new WebSocket(wsUrl)
    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as { event?: string }
        if (!msg.event) return
        setLiveEvent(msg.event)
        load()
      } catch {
        // Ignore malformed messages.
      }
    }
    return () => socket.close()
  }, [])

  const availableVehicles = useMemo(
    () => vehicles.filter((v) => v.status === "Available"),
    [vehicles],
  )
  const availableDrivers = useMemo(() => {
    const today = new Date().toISOString().slice(0, 10)
    return drivers.filter(
      (d) => d.status === "Available" && d.license_expiry_date > today,
    )
  }, [drivers])

  const sourceOptions = useMemo(
    () =>
      Array.from(new Set(trips.map((t) => t.source_name))).sort((a, b) =>
        a.localeCompare(b),
      ),
    [trips],
  )
  const destinationOptions = useMemo(
    () =>
      Array.from(new Set(trips.map((t) => t.destination_name))).sort((a, b) =>
        a.localeCompare(b),
      ),
    [trips],
  )
  const vehicleOptions = useMemo(
    () =>
      Array.from(new Set(trips.map((t) => t.vehicle_reg))).sort((a, b) =>
        a.localeCompare(b),
      ),
    [trips],
  )
  const driverOptions = useMemo(
    () =>
      Array.from(new Set(trips.map((t) => t.driver_license))).sort((a, b) =>
        a.localeCompare(b),
      ),
    [trips],
  )

  const filteredTrips = useMemo(() => {
    return trips.filter((t) => {
      if (sourceFilter !== "All" && t.source_name !== sourceFilter) return false
      if (
        destinationFilter !== "All" &&
        t.destination_name !== destinationFilter
      )
        return false
      if (vehicleFilter !== "All" && t.vehicle_reg !== vehicleFilter)
        return false
      if (driverFilter !== "All" && t.driver_license !== driverFilter)
        return false
      return true
    })
  }, [trips, sourceFilter, destinationFilter, vehicleFilter, driverFilter])

  const selectedVehicle = vehicles.find(
    (v) => v.registration_number === form.vehicle_reg,
  )
  const overload =
    selectedVehicle && form.cargo_weight
      ? Number(form.cargo_weight) > selectedVehicle.max_load_capacity
      : false

  async function createTrip(e: React.FormEvent) {
    e.preventDefault()
    try {
      await apiRequest("/trips/", {
        method: "POST",
        body: {
          source_name: form.source_name,
          destination_name: form.destination_name,
          vehicle_reg: form.vehicle_reg,
          driver_license: form.driver_license,
          cargo_weight: Number(form.cargo_weight),
          planned_distance: Number(form.planned_distance),
          revenue: Number(form.revenue),
        },
      })
      setShowForm(false)
      setForm({
        source_name: "",
        destination_name: "",
        vehicle_reg: "",
        driver_license: "",
        cargo_weight: "",
        planned_distance: "",
        revenue: "0",
      })
      setSuggestion(null)
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to create trip")
    }
  }

  async function suggestTripAssignment() {
    if (
      !form.source_name ||
      !form.destination_name ||
      !form.cargo_weight ||
      !form.planned_distance
    ) {
      setError("Select source, destination, cargo weight and planned distance first.")
      return
    }
    setSuggesting(true)
    try {
      const suggested = await apiRequest<TripSuggestion>("/trips/suggest", {
        method: "POST",
        body: {
          source_name: form.source_name,
          destination_name: form.destination_name,
          vehicle_reg: form.vehicle_reg || "pending",
          driver_license: form.driver_license || "pending",
          cargo_weight: Number(form.cargo_weight),
          planned_distance: Number(form.planned_distance),
          revenue: Number(form.revenue || "0"),
        },
      })
      setSuggestion(suggested)
      setForm((prev) => ({
        ...prev,
        vehicle_reg: suggested.suggested_vehicle_reg,
        driver_license: suggested.suggested_driver_license,
      }))
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to suggest assignment")
    } finally {
      setSuggesting(false)
    }
  }

  async function completeTrip() {
    if (completeId == null) return
    try {
      await apiRequest(
        `/trips/${completeId}/complete?final_odometer=${Number(odometer)}`,
        { method: "PUT" },
      )
      setCompleteId(null)
      setOdometer("")
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to complete trip")
    }
  }

  async function cancelTrip(id: number) {
    if (!confirm("Cancel this trip?")) return
    try {
      await apiRequest(`/trips/${id}/cancel`, { method: "PUT" })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to cancel trip")
    }
  }

  const statusColor = (status: string) => {
    switch (status) {
      case "Dispatched":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
      case "Completed":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
      case "Cancelled":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
      default:
        return "bg-gray-100 text-gray-800"
    }
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Trips</h1>
          <p className="mt-2 text-gray-600 dark:text-gray-400">
            Dispatch and trip lifecycle
          </p>
        </div>
        {canManage && (
          <Button onClick={() => setShowForm(!showForm)}>
            {showForm ? "Cancel" : "Create Trip"}
          </Button>
        )}
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {error}
        </div>
      )}
      {liveEvent && (
        <div className="mb-4 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-sm text-blue-700 dark:border-blue-900 dark:bg-blue-950 dark:text-blue-200">
          Live update: <span className="font-medium">{liveEvent}</span>
        </div>
      )}

      {showForm && canManage && (
        <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
          <h2 className="mb-4 text-lg font-semibold">Create New Trip</h2>
          <form
            onSubmit={createTrip}
            className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
          >
            <div>
              <label className="mb-2 block text-sm font-medium">Source *</label>
              <Select
                value={form.source_name}
                onValueChange={(v) => setForm({ ...form, source_name: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select source" />
                </SelectTrigger>
                <SelectContent>
                  {destinations.map((d) => (
                    <SelectItem key={d.name} value={d.name}>
                      {d.name} ({d.region})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium">
                Destination *
              </label>
              <Select
                value={form.destination_name}
                onValueChange={(v) => setForm({ ...form, destination_name: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select destination" />
                </SelectTrigger>
                <SelectContent>
                  {destinations.map((d) => (
                    <SelectItem key={d.name} value={d.name}>
                      {d.name} ({d.region})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium">
                Available Vehicle *
              </label>
              <Select
                value={form.vehicle_reg}
                onValueChange={(v) => setForm({ ...form, vehicle_reg: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select vehicle" />
                </SelectTrigger>
                <SelectContent>
                  {availableVehicles.map((v) => (
                    <SelectItem
                      key={v.registration_number}
                      value={v.registration_number}
                    >
                      {v.registration_number} · {v.model} (
                      {v.max_load_capacity.toLocaleString()} kg)
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium">
                Available Driver *
              </label>
              <Select
                value={form.driver_license}
                onValueChange={(v) => setForm({ ...form, driver_license: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select driver" />
                </SelectTrigger>
                <SelectContent>
                  {availableDrivers.map((d) => (
                    <SelectItem key={d.license_number} value={d.license_number}>
                      {d.name} ({d.license_number})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium">
                Cargo Weight (kg) *
              </label>
              <Input
                required
                type="number"
                min="0"
                placeholder="e.g., 450"
                value={form.cargo_weight}
                onChange={(e) =>
                  setForm({ ...form, cargo_weight: e.target.value })
                }
              />
              {selectedVehicle && (
                <p className="mt-1 text-xs text-gray-500">
                  Max capacity:{" "}
                  {selectedVehicle.max_load_capacity.toLocaleString()} kg
                </p>
              )}
              {overload && (
                <p className="mt-1 text-xs font-medium text-amber-600">
                  Cargo exceeds vehicle capacity
                </p>
              )}
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium">
                Planned Distance (km) *
              </label>
              <Input
                required
                type="number"
                min="0"
                placeholder="e.g., 120"
                value={form.planned_distance}
                onChange={(e) =>
                  setForm({ ...form, planned_distance: e.target.value })
                }
              />
            </div>
            <div>
              <label className="mb-2 block text-sm font-medium">Revenue</label>
              <Input
                type="number"
                min="0"
                step="0.01"
                value={form.revenue}
                onChange={(e) => setForm({ ...form, revenue: e.target.value })}
              />
            </div>
            <div className="flex items-end gap-2 md:col-span-2 lg:col-span-3">
              <Button
                type="button"
                variant="secondary"
                onClick={suggestTripAssignment}
                disabled={suggesting}
              >
                {suggesting ? "Suggesting..." : "Suggest Vehicle & Driver"}
              </Button>
              <Button type="submit" disabled={overload}>
                Dispatch Trip
              </Button>
              <Button
                type="button"
                variant="secondary"
                onClick={() => setShowForm(false)}
              >
                Cancel
              </Button>
            </div>
            {suggestion && (
              <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200 md:col-span-2 lg:col-span-3">
                Suggested: {suggestion.suggested_vehicle_reg} +{" "}
                {suggestion.suggested_driver_name} ({suggestion.suggested_driver_license}) ·
                Capacity utilization {(suggestion.capacity_utilization * 100).toFixed(1)}%
              </div>
            )}
          </form>
        </div>
      )}

      {completeId != null && (
        <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
          <h2 className="mb-4 text-lg font-semibold">
            Complete Trip #{completeId}
          </h2>
          <div className="flex flex-wrap items-end gap-3">
            <div>
              <label className="mb-2 block text-sm font-medium">
                Final Odometer *
              </label>
              <Input
                type="number"
                placeholder="Must be higher than current"
                value={odometer}
                onChange={(e) => setOdometer(e.target.value)}
              />
            </div>
            <Button onClick={completeTrip}>Confirm Complete</Button>
            <Button variant="secondary" onClick={() => setCompleteId(null)}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      <div className="mb-6 grid grid-cols-1 gap-4 rounded-lg border border-gray-200 bg-white p-4 md:grid-cols-2 lg:grid-cols-4 dark:border-gray-800 dark:bg-gray-950">
        <div>
          <label className="mb-2 block text-sm font-medium">Source</label>
          <Select value={sourceFilter} onValueChange={setSourceFilter}>
            <SelectTrigger>
              <SelectValue placeholder="All sources" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All</SelectItem>
              {sourceOptions.map((name) => (
                <SelectItem key={name} value={name}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label className="mb-2 block text-sm font-medium">Destination</label>
          <Select
            value={destinationFilter}
            onValueChange={setDestinationFilter}
          >
            <SelectTrigger>
              <SelectValue placeholder="All destinations" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All</SelectItem>
              {destinationOptions.map((name) => (
                <SelectItem key={name} value={name}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label className="mb-2 block text-sm font-medium">Vehicle</label>
          <Select value={vehicleFilter} onValueChange={setVehicleFilter}>
            <SelectTrigger>
              <SelectValue placeholder="All vehicles" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All</SelectItem>
              {vehicleOptions.map((reg) => (
                <SelectItem key={reg} value={reg}>
                  {reg}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label className="mb-2 block text-sm font-medium">Driver</label>
          <Select value={driverFilter} onValueChange={setDriverFilter}>
            <SelectTrigger>
              <SelectValue placeholder="All drivers" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All</SelectItem>
              {driverOptions.map((license) => {
                const driver = drivers.find((d) => d.license_number === license)
                return (
                  <SelectItem key={license} value={license}>
                    {driver ? `${driver.name} (${license})` : license}
                  </SelectItem>
                )
              })}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
        <Table>
          <TableHead>
            <TableRow>
              <TableHeaderCell>ID</TableHeaderCell>
              <TableHeaderCell>Route</TableHeaderCell>
              <TableHeaderCell>Vehicle</TableHeaderCell>
              <TableHeaderCell>Driver</TableHeaderCell>
              <TableHeaderCell>Cargo</TableHeaderCell>
              <TableHeaderCell>Status</TableHeaderCell>
              <TableHeaderCell>Actions</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredTrips.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center">
                  No trips found
                </TableCell>
              </TableRow>
            ) : (
              filteredTrips.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">#{t.id}</TableCell>
                  <TableCell>
                    {t.source_name} → {t.destination_name}
                  </TableCell>
                  <TableCell>{t.vehicle_reg}</TableCell>
                  <TableCell>{t.driver_license}</TableCell>
                  <TableCell>{t.cargo_weight} kg</TableCell>
                  <TableCell>
                    <span
                      className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${statusColor(t.status)}`}
                    >
                      {t.status}
                    </span>
                  </TableCell>
                  <TableCell>
                    {t.status === "Dispatched" && (
                      <div className="flex gap-2">
                        <Button
                          variant="secondary"
                          onClick={() => setCompleteId(t.id)}
                        >
                          Complete
                        </Button>
                        <Button
                          variant="destructive"
                          onClick={() => cancelTrip(t.id)}
                        >
                          Cancel
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
