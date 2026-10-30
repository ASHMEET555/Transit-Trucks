"use client"

import { useEffect, useState } from "react"
import { Button } from "@/components/Button"
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
import { ApiError, apiRequest } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"

type AuditLog = {
  id: number
  entity: string
  entity_id: string
  action: string
  actor_email?: string | null
  actor?: string | null
  details?: Record<string, unknown>
  at: string
}

export default function AuditPage() {
  const { currentProfile } = useProfile()
  const isAdmin = currentProfile?.role === "Admin"
  const PAGE_SIZE = 10
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [page, setPage] = useState(1)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [loadedOnce, setLoadedOnce] = useState(false)
  const [entity, setEntity] = useState("")
  const [actor, setActor] = useState("")

  async function load() {
    setLoading(true)
    try {
      setError(null)
      const base = new URLSearchParams()
      if (entity.trim()) base.set("entity", entity.trim())
      if (actor.trim()) base.set("actor", actor.trim())
      base.set("limit", "100")
      const all: AuditLog[] = []
      let cursor: string | null = null
      for (;;) {
        const q = new URLSearchParams(base)
        if (cursor) q.set("cursor", cursor)
        const res = await apiRequest<{ items: AuditLog[]; next_cursor: string | null }>(
          `/audit?${q.toString()}`,
        )
        all.push(...res.items)
        if (!res.next_cursor) break
        cursor = res.next_cursor
      }
      setLogs(all)
      setPage(1)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load audit logs")
    } finally {
      setLoading(false)
      setLoadedOnce(true)
    }
  }

  useEffect(() => {
    if (isAdmin) load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isAdmin])

  const paged = logs.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  if (!isAdmin) {
    return (
      <div className="mx-auto max-w-md rounded-2xl border border-border bg-card p-8 text-center">
        <h1 className="font-serif text-xl text-foreground">Restricted</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Audit logs are visible to admin accounts only. Ask an admin if you need access.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-serif text-2xl text-foreground">Security audit log</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Tamper-evident trail of state changes across vehicles, drivers, trips, and expenses.
          </p>
        </div>
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            load()
          }}
        >
          <Input
            placeholder="Entity (trips)"
            value={entity}
            onChange={(e) => setEntity(e.target.value)}
            className="w-36"
            aria-label="Filter by entity"
          />
          <Input
            placeholder="Actor email"
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            className="w-44"
            aria-label="Filter by actor"
          />
          <Button type="submit" variant="secondary" disabled={loading}>
            Filter
          </Button>
        </form>
      </header>

      {error && (
        <p role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm">
          {error}
        </p>
      )}

      <CardShell as="section" aria-label="Audit entries">
        <header className="flex flex-wrap items-center justify-between gap-3">
          <h3 className="text-[15px] font-medium tracking-tight">Audit entries</h3>
          <p className="text-xs tabular-nums text-muted-foreground">
            {loading && !loadedOnce
              ? "Loading…"
              : `${logs.length} ${logs.length === 1 ? "entry" : "entries"}`}
          </p>
        </header>
        <div className="mt-4 overflow-x-auto">
        {loading && !loadedOnce ? (
          <div className="space-y-2 p-4" role="status" aria-label="Loading audit logs">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="h-10 animate-pulse rounded-lg bg-muted motion-reduce:animate-none" />
            ))}
          </div>
        ) : logs.length === 0 ? (
          <p className="p-8 text-center text-sm text-muted-foreground">
            No audit entries match these filters.
          </p>
        ) : (
          <Table>
            <TableHead>
              <TableRow>
                <TableHeaderCell>ID</TableHeaderCell>
                <TableHeaderCell>At</TableHeaderCell>
                <TableHeaderCell>Entity</TableHeaderCell>
                <TableHeaderCell>Action</TableHeaderCell>
                <TableHeaderCell>Actor</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {paged.map((l) => (
                <TableRow key={l.id}>
                  <TableCell className="font-mono text-xs">{l.id}</TableCell>
                  <TableCell className="text-xs">{new Date(l.at).toLocaleString()}</TableCell>
                  <TableCell className="font-mono text-xs">
                    {l.entity}:{l.entity_id}
                  </TableCell>
                  <TableCell>{l.action}</TableCell>
                  <TableCell className="text-xs">{l.actor_email ?? l.actor ?? "system"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <Pagination
          page={page}
          pageSize={PAGE_SIZE}
          total={logs.length}
          onPageChange={setPage}
          id="audit-pagination"
        />
        </div>
      </CardShell>
    </div>
  )
}
