"use client"

import { useEffect, useMemo, useState } from "react"
import { CardShell } from "@/components/fleet/card-shell"
import { Input } from "@/components/Input"
import { Pagination } from "@/components/Pagination"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { CardHead } from "./chrome"
import { StatusBadge } from "./StatusBadge"
import type { MaintenanceLog } from "./types"

function formatCost(cost: number): string {
  return `$${Number(cost).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function HistoryTable({ logs }: { logs: MaintenanceLog[] }) {
  const PAGE_SIZE = 10
  const [query, setQuery] = useState("")
  const [page, setPage] = useState(1)

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return logs
    return logs.filter(
      (log) =>
        log.vehicle_reg.toLowerCase().includes(q) ||
        log.description.toLowerCase().includes(q) ||
        String(log.id).includes(q),
    )
  }, [logs, query])

  useEffect(() => {
    setPage(1)
  }, [query, logs])

  const paged = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return filtered.slice(start, start + PAGE_SIZE)
  }, [filtered, page])

  return (
    <div>
      <div className="mb-3 rounded-2xl border border-border bg-muted p-3 sm:p-4">
        <label
          htmlFor="maintenance-history-search"
          className="mb-1.5 block text-sm font-medium text-foreground"
        >
          Search history
        </label>
        <Input
          id="maintenance-history-search"
          type="search"
          placeholder="Filter by vehicle, description, or order number"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>
      <CardShell as="section" aria-label="Closed work orders">
        <CardHead
          title="Closed orders"
          meta={`${filtered.length} ${filtered.length === 1 ? "order" : "orders"}`}
        />
        <div className="mt-4 overflow-x-auto">
          <Table>
            <TableHead>
              <TableRow>
                <TableHeaderCell scope="col">Order</TableHeaderCell>
                <TableHeaderCell scope="col">Vehicle</TableHeaderCell>
                <TableHeaderCell scope="col">Description</TableHeaderCell>
                <TableHeaderCell scope="col">Cost</TableHeaderCell>
                <TableHeaderCell scope="col">Opened</TableHeaderCell>
                <TableHeaderCell scope="col">Closed</TableHeaderCell>
                <TableHeaderCell scope="col">Status</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-center">
                    {logs.length === 0
                      ? "No closed work orders yet"
                      : "No closed work orders match this search"}
                  </TableCell>
                </TableRow>
              ) : (
                paged.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="font-mono tabular-nums">#{log.id}</TableCell>
                    <TableCell className="font-serif font-medium">
                      {log.vehicle_reg}
                    </TableCell>
                    <TableCell>{log.description}</TableCell>
                    <TableCell className="font-mono tabular-nums">
                      {formatCost(log.cost)}
                    </TableCell>
                    <TableCell className="font-mono tabular-nums">
                      {log.start_date}
                    </TableCell>
                    <TableCell className="font-mono tabular-nums">
                      {log.end_date || "—"}
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={log.status} />
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <Pagination
          page={page}
          pageSize={PAGE_SIZE}
          total={filtered.length}
          onPageChange={setPage}
          id="maintenance-history-pagination"
        />
      </CardShell>
    </div>
  )
}
