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

type Trip = {
  id: number
  source_name: string
  destination_name: string
  status: string
}

type Expense = {
  id: number
  trip_id: number
  type: string
  liters?: number | null
  cost: number
  date: string
}

export default function FuelAndExpensesPage() {
  const { currentProfile } = useProfile()
  const canManage = canWrite(currentProfile?.role || "", "fuelAndExpenses")
  const [trips, setTrips] = useState<Trip[]>([])
  const [tripId, setTripId] = useState("")
  const [expenses, setExpenses] = useState<Expense[]>([])
  const [error, setError] = useState<string | null>(null)
  const [exportFormat, setExportFormat] = useState<"csv" | "pdf">("csv")
  const [form, setForm] = useState({
    type: "Fuel",
    liters: "",
    cost: "",
    date: new Date().toISOString().slice(0, 10),
  })

  useEffect(() => {
    apiRequest<Trip[]>("/trips/")
      .then((data) => {
        setTrips(data)
        if (data.length) setTripId(String(data[0].id))
      })
      .catch((err) =>
        setError(err instanceof ApiError ? err.detail : "Failed to load trips"),
      )
  }, [])

  async function loadExpenses(id: string) {
    if (!id) return
    try {
      setExpenses(await apiRequest<Expense[]>(`/finance/expenses/${id}`))
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load expenses")
    }
  }

  useEffect(() => {
    if (tripId) loadExpenses(tripId)
  }, [tripId])

  const totals = useMemo(() => {
    const fuel = expenses
      .filter((e) => e.type === "Fuel")
      .reduce((s, e) => s + e.cost, 0)
    const toll = expenses
      .filter((e) => e.type === "Toll")
      .reduce((s, e) => s + e.cost, 0)
    const other = expenses
      .filter((e) => e.type !== "Fuel" && e.type !== "Toll")
      .reduce((s, e) => s + e.cost, 0)
    return { fuel, toll, other, all: fuel + toll + other }
  }, [expenses])

  async function addExpense(e: React.FormEvent) {
    e.preventDefault()
    try {
      await apiRequest("/finance/expenses", {
        method: "POST",
        body: {
          trip_id: Number(tripId),
          type: form.type,
          liters: form.type === "Fuel" ? Number(form.liters) : null,
          cost: Number(form.cost),
          date: form.date,
        },
      })
      setForm({
        type: "Fuel",
        liters: "",
        cost: "",
        date: new Date().toISOString().slice(0, 10),
      })
      await loadExpenses(tripId)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to log expense")
    }
  }

  function exportCsv() {
    const rows = [
      "id,trip_id,type,liters,cost,date",
      ...expenses.map((e) =>
        [e.id, e.trip_id, e.type, e.liters ?? "", e.cost, e.date].join(","),
      ),
    ]
    const blob = new Blob([rows.join("\n")], { type: "text/csv" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `trip-${tripId}-expenses.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  function exportPdf() {
    const doc = new jsPDF()
    
    // Title
    doc.setFontSize(18)
    doc.text(`Trip #${tripId} Expenses`, 14, 22)
    
    // Totals
    doc.setFontSize(12)
    doc.text(`Fuel: $${totals.fuel.toLocaleString()}`, 14, 35)
    doc.text(`Tolls: $${totals.toll.toLocaleString()}`, 14, 42)
    doc.text(`Other: $${totals.other.toLocaleString()}`, 14, 49)
    doc.text(`Total: $${totals.all.toLocaleString()}`, 14, 56)
    
    // Table headers
    doc.setFontSize(10)
    doc.setFillColor(200, 200, 200)
    doc.rect(14, 65, 182, 8, "F")
    doc.text("ID", 16, 70)
    doc.text("Type", 36, 70)
    doc.text("Liters", 66, 70)
    doc.text("Cost", 96, 70)
    doc.text("Date", 136, 70)
    
    // Table rows
    let y = 78
    expenses.forEach((e, index) => {
      if (y > 270) {
        doc.addPage()
        y = 20
      }
      if (index % 2 === 0) {
        doc.setFillColor(245, 245, 245)
        doc.rect(14, y - 5, 182, 8, "F")
      }
      doc.text(String(e.id), 16, y)
      doc.text(e.type, 36, y)
      doc.text(e.liters ? String(e.liters) : "—", 66, y)
      doc.text(`$${e.cost}`, 96, y)
      doc.text(e.date, 136, y)
      y += 8
    })
    
    doc.save(`trip-${tripId}-expenses.pdf`)
  }

  function handleExport() {
    if (exportFormat === "csv") {
      exportCsv()
    } else {
      exportPdf()
    }
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Fuel & Expenses</h1>
          <p className="mt-1 text-gray-600 dark:text-gray-400">
            Trip-linked fuel, tolls, and other costs
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
          <Button variant="secondary" onClick={handleExport} disabled={!expenses.length}>
            Export
          </Button>
        </div>
      </div>

      {error && (
        <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      <div className="max-w-xl">
        <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
          Select Trip *
        </label>
        <Select value={tripId} onValueChange={setTripId}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Choose a trip" />
          </SelectTrigger>
          <SelectContent>
            {trips.map((t) => (
              <SelectItem key={t.id} value={String(t.id)}>
                #{t.id} · {t.source_name} → {t.destination_name} ({t.status})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="mt-1 text-xs text-gray-500">
          Expenses below are linked to the selected trip.
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-4">
        {[
          ["Fuel", totals.fuel],
          ["Tolls", totals.toll],
          ["Other", totals.other],
          ["Total", totals.all],
        ].map(([label, value]) => (
          <div
            key={label as string}
            className="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-950"
          >
            <p className="text-sm text-gray-500">{label}</p>
            <p className="mt-1 text-xl font-semibold">
              ${Number(value).toLocaleString()}
            </p>
          </div>
        ))}
      </div>

      {canManage && (
        <div className="rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
          <h2 className="mb-4 text-lg font-semibold">Log Expense</h2>
          <form
            onSubmit={addExpense}
            className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4"
          >
            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Expense Type *
              </label>
              <Select
                value={form.type}
                onValueChange={(v) =>
                  setForm({ ...form, type: v, liters: v === "Fuel" ? form.liters : "" })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Fuel">Fuel</SelectItem>
                  <SelectItem value="Toll">Toll</SelectItem>
                  <SelectItem value="Maintenance-Surge">
                    Maintenance-Surge
                  </SelectItem>
                  <SelectItem value="Other">Other</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {form.type === "Fuel" && (
              <div>
                <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                  Liters *
                </label>
                <Input
                  type="number"
                  min="0"
                  step="0.01"
                  placeholder="e.g., 40"
                  value={form.liters}
                  onChange={(e) => setForm({ ...form, liters: e.target.value })}
                  required
                />
              </div>
            )}

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Cost *
              </label>
              <Input
                type="number"
                min="0"
                step="0.01"
                placeholder="e.g., 80.00"
                value={form.cost}
                onChange={(e) => setForm({ ...form, cost: e.target.value })}
                required
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Date *
              </label>
              <Input
                type="date"
                value={form.date}
                onChange={(e) => setForm({ ...form, date: e.target.value })}
                required
              />
            </div>

            <div className="lg:col-span-4">
              <Button type="submit">Log Expense</Button>
            </div>
          </form>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
        <Table>
          <TableHead>
            <TableRow>
              <TableHeaderCell>ID</TableHeaderCell>
              <TableHeaderCell>Type</TableHeaderCell>
              <TableHeaderCell>Liters</TableHeaderCell>
              <TableHeaderCell>Cost</TableHeaderCell>
              <TableHeaderCell>Date</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {expenses.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center">
                  No expenses for this trip yet.
                </TableCell>
              </TableRow>
            ) : (
              expenses.map((e) => (
                <TableRow key={e.id}>
                  <TableCell>#{e.id}</TableCell>
                  <TableCell>{e.type}</TableCell>
                  <TableCell>{e.liters ?? "—"}</TableCell>
                  <TableCell>${e.cost}</TableCell>
                  <TableCell>{e.date}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
