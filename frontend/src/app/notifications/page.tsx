"use client"

import { useEffect, useState, useCallback } from "react"
import { Bell, CheckCircle2, Clock, AlertTriangle, ShieldAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { listNotifications, markNotificationRead, type Notification } from "@/lib/api/client"
import { TableSkeleton, ErrorState, EmptyState } from "@/components/ui/data-states"

export default function NotificationsPage() {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadNotifications = useCallback(async () => {
    try {
      setError(null)
      const items = await listNotifications(50)
      setNotifications(items)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load notifications.")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadNotifications()
  }, [loadNotifications])

  const handleMarkRead = async (id: number) => {
    try {
      await markNotificationRead(id)
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n))
      )
    } catch {
      // Ignore
    }
  }

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-4xl mx-auto">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border/60 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">Notifications & Alerts</h1>
          </div>
          <p className="text-xs text-muted-foreground mt-1">
            Real-time compliance triggers, driver license expiry alerts, and critical operational events.
          </p>
        </div>

        <Button variant="outline" size="sm" onClick={loadNotifications} className="text-xs">
          Refresh Alerts
        </Button>
      </div>

      {loading ? (
        <TableSkeleton rows={4} cols={3} />
      ) : error ? (
        <ErrorState message={error} onRetry={loadNotifications} />
      ) : notifications.length === 0 ? (
        <EmptyState
          title="No alerts"
          description="All driver licenses are current and no operational alarms have triggered."
        />
      ) : (
        <div className="space-y-3">
          {notifications.map((n) => {
            const isRead = !!n.read_at
            const payload = (n.payload || {}) as Record<string, any>
            const title = n.type || "System Alert"
            const message = payload.message || payload.description || JSON.stringify(payload)
            return (
              <div
                key={n.id}
                className={`p-4 rounded-xl border transition-all flex items-start justify-between gap-4 ${
                  isRead
                    ? "border-border/60 bg-card/60 opacity-75"
                    : "border-signal/30 bg-signal/5 shadow-xs"
                }`}
              >
                <div className="flex items-start gap-3">
                  <div
                    className={`size-8 rounded-full flex items-center justify-center shrink-0 mt-0.5 ${
                      isRead ? "bg-neutral-100 text-neutral-500" : "bg-signal/20 text-signal"
                    }`}
                  >
                    <AlertTriangle className="size-4" />
                  </div>
                  <div>
                    <h3 className="text-xs font-semibold text-foreground mb-0.5">{title}</h3>
                    <p className="text-xs text-muted-foreground leading-relaxed">{message}</p>
                    <div className="flex items-center gap-3 text-[11px] text-muted-foreground mt-2">
                      <span className="flex items-center gap-1 font-mono">
                        <Clock className="size-3" />
                        {new Date(n.created_at || "").toLocaleString()}
                      </span>
                    </div>
                  </div>
                </div>

                {!isRead && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleMarkRead(n.id || 0)}
                    className="shrink-0 text-xs gap-1.5"
                  >
                    <CheckCircle2 className="size-3.5" /> Mark read
                  </Button>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
