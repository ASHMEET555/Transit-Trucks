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
import { ApiError, apiRequest } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite } from "@/lib/rbacConfig"
import jsPDF from "jspdf"

type Vehicle = {
  registration_number: string
  model: string
  status: string
}

type MaintenanceLog = {
  id: number
  vehicle_reg: string
  description: string
  cost: number
  start_date: string
  end_date?: string | null
  status: string
}

export default function MaintenancePage() {
  const { currentProfile } = useProfile()
  const canManage = canWrite(currentProfile?.role || "", "maintenance")
  const [logs, setLogs] = useState<MaintenanceLog[]>([])
  const [vehicles, setVehicles] = useState<Vehicle[]>([])
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [vehicleSearch, setVehicleSearch] = useState("")
  const [showVehicleList, setShowVehicleList] = useState(false)
  const [exportFormat, setExportFormat] = useState<"csv" | "pdf">("csv")
  const [form, setForm] = useState({
    vehicle_reg: "",
    description: "",
    cost: "0.00",
    start_date: "",
  })

  async function load() {
    try {
      setError(null)
      const [m, v] = await Promise.all([
        apiRequest<MaintenanceLog[]>("/maintenance/"),
        apiRequest<Vehicle[]>("/fleet/vehicles"),
      ])
      setLogs(m)
      setVehicles(v)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load")
    }
  }

  useEffect(() => {
    load()
  }, [])

  const eligible = useMemo(
    () =>
      vehicles.filter(
        (v) => v.status !== "On Trip" && v.status !== "Retired",
      ),
    [vehicles],
  )

  const filteredVehicles = useMemo(() => {
    const q = vehicleSearch.trim().toLowerCase()
    if (!q) return eligible
    return eligible.filter(
      (v) =>
        v.registration_number.toLowerCase().includes(q) ||
        v.model.toLowerCase().includes(q) ||
        v.status.toLowerCase().includes(q),
    )
  }, [eligible, vehicleSearch])

  function selectVehicle(reg: string) {
    setForm((f) => ({ ...f, vehicle_reg: reg }))
    setVehicleSearch(reg)
    setShowVehicleList(false)
  }

  async function openOrder(e: React.FormEvent) {
    e.preventDefault()
    if (!form.vehicle_reg) {
      setError("Please search and select a vehicle.")
      return
    }
    try {
      await apiRequest("/maintenance/", {
        method: "POST",
        body: {
          vehicle_reg: form.vehicle_reg,
          description: form.description,
          cost: Number(form.cost),
          start_date:
            form.start_date || new Date().toISOString().slice(0, 10),
        },
      })
      setShowForm(false)
      setVehicleSearch("")
      setForm({
        vehicle_reg: "",
        description: "",
        cost: "0.00",
        start_date: "",
      })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to add record")
    }
  }

  async function closeOrder(id: number) {
    try {
      await apiRequest(`/maintenance/${id}/close`, {
        method: "PUT",
        body: { end_date: new Date().toISOString().slice(0, 10) },
      })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to close record")
    }
  }

  function exportCsv() {
    const rows = [
      "id,vehicle_reg,description,cost,start_date,end_date,status",
      ...logs.map((log) =>
        [log.id, log.vehicle_reg, log.description, log.cost, log.start_date, log.end_date || "", log.status].join(","),
      ),
    ]
    const blob = new Blob([rows.join("\n")], { type: "text/csv" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = "maintenance-records.csv"
    a.click()
    URL.revokeObjectURL(url)
  }

  function exportPdf() {
    const doc = new jsPDF()
    
    // Title
    doc.setFontSize(18)
    doc.text("Maintenance Records", 14, 22)
    
    // Table headers
    doc.setFontSize(10)
    doc.setFillColor(200, 200, 200)
    doc.rect(14, 35, 182, 8, "F")
    doc.text("ID", 16, 40)
    doc.text("Vehicle", 36, 40)
    doc.text("Description", 66, 40)
    doc.text("Cost", 126, 40)
    doc.text("Start Date", 146, 40)
    doc.text("End Date", 176, 40)
    
    // Table rows
    let y = 48
    logs.forEach((log, index) => {
      if (y > 270) {
        doc.addPage()
        y = 20
      }
      if (index % 2 === 0) {
        doc.setFillColor(245, 245, 245)
        doc.rect(14, y - 5, 182, 8, "F")
      }
      doc.text(String(log.id), 16, y)
      doc.text(log.vehicle_reg, 36, y)
      doc.text(log.description.substring(0, 20), 66, y)
      doc.text(`$${log.cost}`, 126, y)
      doc.text(log.start_date, 146, y)
      doc.text(log.end_date || "—", 176, y)
      y += 8
    })
    
    doc.save("maintenance-records.pdf")
  }

  function handleExport() {
    if (exportFormat === "csv") {
      exportCsv()
    } else {
      exportPdf()
    }
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Maintenance Management</h1>
          <p className="mt-2 text-gray-600 dark:text-gray-400">
            View and manage vehicle maintenance records here.
          </p>
        </div>
        <div className="flex gap-2">
          <Select value={exportFormat} onValueChange={(v: "csv" | "pdf") => setExportFormat(v)}>
            <SelectTrigger className="w-[120px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="csv">CSV</SelectItem>
              <SelectItem value="pdf">PDF</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="secondary" onClick={handleExport} disabled={!logs.length}>
            Export
          </Button>
          {canManage && (
            <Button
              onClick={() => {
                setShowForm(!showForm)
                setError(null)
              }}
            >
              {showForm ? "Cancel" : "Add Record"}
            </Button>
          )}
        </div>
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-950">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-red-900 dark:text-red-100">
              {error}
            </p>
            <button
              onClick={() => setError(null)}
              className="text-sm font-medium text-red-700 hover:text-red-900"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {showForm && canManage && (
        <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
          <h2 className="mb-4 text-lg font-semibold">
            Add Maintenance Record
          </h2>
          <form
            onSubmit={openOrder}
            className="grid grid-cols-1 gap-4 md:grid-cols-2"
          >
            <div className="relative">
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Vehicle Registration *
              </label>
              <Input
                required
                value={vehicleSearch}
                placeholder="Search vehicle..."
                onChange={(e) => {
                  setVehicleSearch(e.target.value)
                  setForm((f) => ({ ...f, vehicle_reg: "" }))
                  setShowVehicleList(true)
                }}
                onFocus={() => setShowVehicleList(true)}
                autoComplete="off"
              />
              {showVehicleList && (
                <div className="absolute z-20 mt-1 max-h-48 w-full overflow-auto rounded-md border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900">
                  {filteredVehicles.length === 0 ? (
                    <p className="px-3 py-2 text-sm text-gray-500">
                      No matching vehicles
                    </p>
                  ) : (
                    filteredVehicles.map((v) => (
                      <button
                        key={v.registration_number}
                        type="button"
                        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
                        onClick={() => selectVehicle(v.registration_number)}
                      >
                        <span className="font-medium">
                          {v.registration_number}
                        </span>
                        <span className="text-gray-500">
                          {v.model} · {v.status}
                        </span>
                      </button>
                    ))
                  )}
                </div>
              )}
              {form.vehicle_reg && (
                <p className="mt-1 text-xs text-emerald-600">
                  Selected: {form.vehicle_reg}
                </p>
              )}
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Description *
              </label>
              <Input
                required
                minLength={5}
                placeholder="e.g., Oil change, brake replacement"
                value={form.description}
                onChange={(e) =>
                  setForm({ ...form, description: e.target.value })
                }
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Cost *
              </label>
              <Input
                required
                type="number"
                min="0"
                step="0.01"
                value={form.cost}
                onChange={(e) => setForm({ ...form, cost: e.target.value })}
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Start Date *
              </label>
              <Input
                required
                type="date"
                value={form.start_date}
                onChange={(e) =>
                  setForm({ ...form, start_date: e.target.value })
                }
              />
            </div>

            <div className="flex gap-2 md:col-span-2">
              <Button type="submit">Add Record</Button>
              <Button
                type="button"
                variant="secondary"
                onClick={() => {
                  setShowForm(false)
                  setShowVehicleList(false)
                }}
              >
                Cancel
              </Button>
            </div>
          </form>
        </div>
      )}

      <div className="rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
        <Table>
          <TableHead>
            <TableRow>
              <TableHeaderCell>Vehicle Registration</TableHeaderCell>
              <TableHeaderCell>Description</TableHeaderCell>
              <TableHeaderCell>Cost</TableHeaderCell>
              <TableHeaderCell>Start Date</TableHeaderCell>
              <TableHeaderCell>End Date</TableHeaderCell>
              <TableHeaderCell>Status</TableHeaderCell>
              {canManage && <TableHeaderCell>Actions</TableHeaderCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center">
                  No maintenance records found
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="font-medium">
                    {log.vehicle_reg}
                  </TableCell>
                  <TableCell>{log.description}</TableCell>
                  <TableCell>${Number(log.cost).toLocaleString()}</TableCell>
                  <TableCell>{log.start_date}</TableCell>
                  <TableCell>{log.end_date || "—"}</TableCell>
                  <TableCell>
                    <span
                      className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        log.status === "Open"
                          ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                          : "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                      }`}
                    >
                      {log.status}
                    </span>
                  </TableCell>
                  {canManage && (
                    <TableCell>
                      {log.status === "Open" && (
                        <Button
                          variant="secondary"
                          onClick={() => closeOrder(log.id)}
                        >
                          Close
                        </Button>
                      )}
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
