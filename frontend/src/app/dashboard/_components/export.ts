"use client"

import { BACKEND_URL } from "@/data/data"

export type DashboardExportParams = {
  from?: string
  to?: string
  vehicle_reg?: string
}

export async function downloadDashboardCsv(params: DashboardExportParams = {}) {
  const query = new URLSearchParams()
  if (params.from) query.set("from", params.from)
  if (params.to) query.set("to", params.to)
  if (params.vehicle_reg) query.set("vehicle_reg", params.vehicle_reg)
  const suffix = query.toString() ? `?${query.toString()}` : ""
  const res = await fetch(`${BACKEND_URL}/dashboard/export.csv${suffix}`, {
    credentials: "include",
  })
  if (!res.ok) throw new Error(`Export failed (${res.status})`)
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = "dashboard.csv"
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

function escapeCsvCell(value: string | number | null | undefined): string {
  const text = value == null ? "" : String(value)
  return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

export function downloadClientCsv(
  filename: string,
  headers: string[],
  rows: Array<Array<string | number | null | undefined>>,
) {
  const lines = [
    headers.map(escapeCsvCell).join(","),
    ...rows.map((row) => row.map(escapeCsvCell).join(",")),
  ]
  const blob = new Blob([lines.join("\n")], { type: "text/csv" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
