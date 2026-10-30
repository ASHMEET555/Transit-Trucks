"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { AlertTriangle, Bell, CheckCircle2, Info, Truck, Wrench } from "lucide-react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
import { ApiError, apiRequest } from "@/lib/api"
import { cn } from "@/lib/utils"

type Notification = {
  id: number
  type: string
  payload: Record<string, unknown>
  read_at: string | null
  created_at: string
}

function str(v: unknown): string | null {
  return typeof v === "string" && v.length > 0 ? v : null
}

function num(v: unknown): number | null {
  return typeof v === "number" && Number.isFinite(v) ? v : null
}

function describe(n: Notification): { title: string; body: string; href: string } {
  const p = n.payload
  const days = num(p.days_remaining)
  const expiry = str(p.expiry_date)
  const license = str(p.license_number) ?? str(p.driver_license) ?? str(p.license)
  const tripId = num(p.trip_id) ?? (typeof p.id === "number" ? (p.id as number) : null)
  const vehicle = str(p.vehicle_reg) ?? str(p.registration_number)

  if (n.type.startsWith("license_expiring")) {
    const window = n.type.includes("7d") ? "7 days" : n.type.includes("30d") ? "30 days" : null
    return {
      title: `Driver license expiring${days !== null ? ` in ${days} days` : window ? ` within ${window}` : ""}`,
      body: license
        ? `License ${license}${expiry ? ` expires ${expiry}` : ""}. Renew it or suspend the driver before it lapses.`
        : `A driver license${expiry ? ` expires ${expiry}` : " is expiring"}. Review compliance in Drivers.`,
      href: "/drivers",
    }
  }
  if (n.type.startsWith("trip.")) {
    const action = n.type.slice("trip.".length).replace(/_/g, " ")
    return {
      title: tripId !== null ? `Trip #${tripId} ${action}` : `Trip ${action}`,
      body: vehicle ? `Vehicle ${vehicle}. See the trip board for details.` : "See the trip board for details.",
      href: "/trips",
    }
  }
  if (n.type.startsWith("maintenance.")) {
    return {
      title: `Maintenance ${n.type.slice("maintenance.".length).replace(/_/g, " ")}`,
      body: vehicle ? `Vehicle ${vehicle}. Review the open queue.` : "Review the open queue.",
      href: "/maintenance",
    }
  }
  if (n.type.startsWith("vehicle.")) {
    return {
      title: `Vehicle ${n.type.slice("vehicle.".length).replace(/_/g, " ")}`,
      body: vehicle ? `Vehicle ${vehicle}. Open the registry.` : "Open the vehicle registry.",
      href: "/fleet",
    }
  }
  if (n.type.startsWith("expense.")) {
    return {
      title: `Expense ${n.type.slice("expense.".length).replace(/_/g, " ")}`,
      body: "Review the expense ledger.",
      href: "/fuel-and-expenses",
    }
  }
  if (n.type.startsWith("driver.")) {
    return {
      title: `Driver ${n.type.slice("driver.".length).replace(/_/g, " ")}`,
      body: license ? `License ${license}. Open the driver registry.` : "Open the driver registry.",
      href: "/drivers",
    }
  }
  const pretty = n.type.replace(/_/g, " ")
  return { title: pretty.charAt(0).toUpperCase() + pretty.slice(1), body: "Open the dashboard for context.", href: "/dashboard" }
}

function iconFor(type: string) {
  if (type.startsWith("license_expiring")) return { Icon: AlertTriangle, chip: "bg-signal text-signal-foreground" }
  if (type.startsWith("trip.")) return { Icon: Truck, chip: "bg-sage text-primary" }
  if (type.startsWith("maintenance.")) return { Icon: Wrench, chip: "bg-sand text-foreground" }
  if (type.startsWith("expense.")) return { Icon: Info, chip: "bg-mist text-foreground" }
  if (type.includes("complete") || type.includes("read")) return { Icon: CheckCircle2, chip: "bg-sage text-primary" }
  return { Icon: Bell, chip: "bg-muted text-muted-foreground" }
}

export default function NotificationsPage() {
  const [items, setItems] = useState<Notification[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [markingAll, setMarkingAll] = useState(false)
  const [filter, setFilter] = useState<"all" | "unread">("all")

  async function load() {
    setLoading(true)
    try {
      setError(null)
      const res = await apiRequest<{ items: Notification[]; count: number }>("/notifications?limit=50")
      const sorted = [...res.items].sort(
        (a, b) =>
          Number(a.read_at != null) - Number(b.read_at != null) ||
          +new Date(b.created_at) - +new Date(a.created_at),
      )
      setItems(sorted)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load notifications")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function markRead(id: number) {
    try {
      await apiRequest(`/notifications/${id}/read`, { method: "POST" })
      setItems((prev) => prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n)))
    } catch {
      setError("Could not mark that notification as read. Try again.")
    }
  }

  async function markAllRead() {
    const unread = items.filter((n) => !n.read_at)
    if (unread.length === 0) return
    setMarkingAll(true)
    let failed = false
    for (const n of unread) {
      try {
        await apiRequest(`/notifications/${n.id}/read`, { method: "POST" })
      } catch {
        failed = true
        break
      }
    }
    if (failed) setError("Some notifications could not be marked as read.")
    setMarkingAll(false)
    await load()
  }

  const unread = items.filter((n) => !n.read_at).length
  const visible = filter === "unread" ? items.filter((n) => !n.read_at) : items

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Notifications</h1>
          <p className="mt-1 text-sm text-muted-foreground" role="status">
            {loading ? "Loading notifications…" : unread === 0 ? "All caught up." : `${unread} unread, newest first.`}
          </p>
        </div>
        <div className="flex gap-2">
          <div role="group" aria-label="Filter notifications" className="flex overflow-hidden rounded-lg border border-border">
            {(["all", "unread"] as const).map((f) => (
              <button
                key={f}
                type="button"
                onClick={() => setFilter(f)}
                aria-pressed={filter === f}
                className={cn(
                  "px-3 py-1.5 text-sm font-medium capitalize transition-colors focus-visible:outline-2 focus-visible:outline-ring",
                  filter === f ? "bg-primary text-primary-foreground" : "bg-card text-muted-foreground hover:text-foreground",
                )}
              >
                {f}
              </button>
            ))}
          </div>
          <Button variant="secondary" onClick={markAllRead} disabled={unread === 0 || loading || markingAll}>
            {markingAll ? "Marking…" : "Mark all read"}
          </Button>
        </div>
      </header>

      {error && (
        <p role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm">
          {error}
        </p>
      )}

      {loading ? (
        <div className="space-y-3" role="status" aria-label="Loading notifications">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-20 animate-pulse rounded-[28px] bg-muted motion-reduce:animate-none" />
          ))}
        </div>
      ) : visible.length === 0 ? (
        <CardShell className="p-8 text-center">
          <p className="text-sm font-medium">No notifications here.</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {filter === "unread" ? "Everything is read." : "License, trip and maintenance alerts land here."}
          </p>
          <Link href="/dashboard" className="mt-2 inline-block text-sm font-medium text-primary hover:underline">
            Back to dashboard
          </Link>
        </CardShell>
      ) : (
        <ul className="space-y-3">
          {visible.map((n) => {
            const { title, body, href } = describe(n)
            const { Icon, chip } = iconFor(n.type)
            return (
              <CardShell
                as="li"
                key={n.id}
                className={cn("flex items-start gap-4 p-4 sm:p-5", n.read_at && "opacity-70")}
              >
                {!n.read_at && <span aria-label="Unread" className="mt-2 size-2 shrink-0 rounded-full bg-signal" />}
                <span className={cn("grid size-10 shrink-0 place-items-center rounded-full", chip)}>
                  <Icon className="size-5" aria-hidden="true" />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-[15px] font-medium tracking-tight text-foreground">{title}</p>
                  <p className="mt-0.5 text-sm text-muted-foreground">{body}</p>
                  <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                    <span className="font-mono tabular-nums">{new Date(n.created_at).toLocaleString()}</span>
                    <Link href={href} className="font-medium text-primary hover:underline">
                      Open
                    </Link>
                  </p>
                </div>
                {!n.read_at && (
                  <Button variant="secondary" size="sm" onClick={() => markRead(n.id)} className="shrink-0">
                    Mark read
                  </Button>
                )}
              </CardShell>
            )
          })}
        </ul>
      )}
    </div>
  )
}
