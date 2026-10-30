"use client"

import { useEffect, useState, useCallback } from "react"
import { ShieldCheck, Filter, Download, ArrowRight, User } from "lucide-react"
import { Button } from "@/components/ui/button"
import { listAuditLogs, type AuditLog } from "@/lib/api/client"
import { TableSkeleton, ErrorState, EmptyState, ForbiddenState } from "@/components/ui/data-states"
import { useProfile } from "@/lib/ProfileContext"

export default function AuditPage() {
  const { currentProfile } = useProfile()
  const role = currentProfile?.role || ""
  const isAdmin = role === "Admin"

  const [logs, setLogs] = useState<AuditLog[]>([])
  const [entityFilter, setEntityFilter] = useState("all")
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadAudit = useCallback(async () => {
    if (!isAdmin) return
    try {
      setError(null)
      const res = await listAuditLogs({
        entity: entityFilter !== "all" ? entityFilter : undefined,
        limit: 50,
      })
      setLogs(res.items || [])
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load audit logs.")
    } finally {
      setLoading(false)
    }
  }, [isAdmin, entityFilter])

  useEffect(() => {
    if (isAdmin) {
      loadAudit()
    } else {
      setLoading(false)
    }
  }, [isAdmin, loadAudit])

  if (!isAdmin) {
    return (
      <div className="p-8 max-w-4xl mx-auto">
        <ForbiddenState message="The audit event ledger is restricted to Administrator personnel only." />
      </div>
    )
  }

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border/60 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">Security Audit Trail</h1>
            <span className="text-xs font-mono px-2 py-0.5 rounded-md bg-primary/10 text-primary font-semibold">
              Tamper-evident
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-1">
            Immutable log of user actions, trip state transitions, security events, and entity modifications.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <select
            value={entityFilter}
            onChange={(e) => setEntityFilter(e.target.value)}
            className="text-xs px-3 py-1.5 rounded-lg border border-border bg-background"
          >
            <option value="all">All Entities</option>
            <option value="trips">Trips</option>
            <option value="vehicles">Vehicles</option>
            <option value="drivers">Drivers</option>
            <option value="maintenance">Maintenance</option>
            <option value="users">Users</option>
          </select>
          <Button variant="outline" size="sm" onClick={loadAudit} className="text-xs">
            Refresh
          </Button>
        </div>
      </div>

      {loading ? (
        <TableSkeleton rows={8} cols={5} />
      ) : error ? (
        <ErrorState message={error} onRetry={loadAudit} />
      ) : logs.length === 0 ? (
        <EmptyState title="No audit entries" description="No audit events found for selected filters." />
      ) : (
        <div className="rounded-xl border border-border/80 bg-card overflow-hidden shadow-xs">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-neutral-50/70 border-b border-border/80 text-muted-foreground font-semibold">
                <tr>
                  <th className="p-3.5">Timestamp (UTC)</th>
                  <th className="p-3.5">Actor</th>
                  <th className="p-3.5">Action</th>
                  <th className="p-3.5">Entity</th>
                  <th className="p-3.5">Entity ID</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/60">
                {logs.map((log) => (
                  <tr key={log.id} className="hover:bg-neutral-50/50">
                    <td className="p-3.5 font-mono text-muted-foreground">
                      {new Date(log.at || "").toLocaleString()}
                    </td>
                    <td className="p-3.5 font-medium text-foreground">
                      <div className="flex items-center gap-1.5">
                        <User className="size-3 text-muted-foreground" />
                        <span>{log.actor_email || log.actor || "System"}</span>
                      </div>
                    </td>
                    <td className="p-3.5">
                      <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-neutral-100 text-neutral-800 font-semibold">
                        {log.action}
                      </span>
                    </td>
                    <td className="p-3.5 text-foreground capitalize">{log.entity}</td>
                    <td className="p-3.5 font-mono text-foreground">{log.entity_id}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
