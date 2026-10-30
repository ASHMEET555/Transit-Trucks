"use client"

import Link from "next/link"
import {
  ArrowRight,
  FileText,
  ShieldAlert,
  Wrench,
  type LucideIcon,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { CardShell } from "@/components/fleet/card-shell"
import type { Trip } from "@/lib/tripTypes"
import { EmptyNote, IconBadge } from "./states"

export const EXPIRY_WINDOW_DAYS = 30

export type FleetDriver = {
  license_number: string
  name: string
  license_expiry_date: string
  status: string
}

export type MaintenanceLog = {
  id: number
  vehicle_reg: string
  description: string
  start_date: string
  end_date: string | null
  status: string
}

const priorityTone: Record<"high" | "medium" | "low", string> = {
  high: "bg-destructive text-destructive-foreground font-semibold",
  medium: "bg-sand text-foreground",
  low: "bg-sage text-foreground",
}

function urgency(expiry: string): "high" | "medium" | "low" {
  const days = Math.ceil(
    (new Date(expiry).getTime() - Date.now()) / 86_400_000,
  )
  if (days <= 7) return "high"
  if (days <= 14) return "medium"
  return "low"
}

function JumpLink({ href, children }: { href: string; children: string }) {
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-1.5 text-sm font-medium text-foreground underline decoration-border underline-offset-4 hover:decoration-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      {children}
      <ArrowRight className="size-4" aria-hidden="true" />
    </Link>
  )
}

function QueueShell({
  icon,
  title,
  countLabel,
  countTone,
  actionHref,
  actionLabel,
  children,
}: {
  icon: LucideIcon
  title: string
  countLabel: string
  countTone: "high" | "medium" | "low"
  actionHref: string
  actionLabel: string
  children: React.ReactNode
}) {
  return (
    <CardShell
      tone="paper"
      className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-3">
          <IconBadge icon={icon} />
          <h3 className="truncate text-[15px] font-medium tracking-tight">
            {title}
          </h3>
        </div>
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
            priorityTone[countTone],
          )}
        >
          {countLabel}
        </span>
      </div>
      <div className="mt-2 flex-1">{children}</div>
      <div className="mt-2 border-t border-current/10 pt-3">
        <JumpLink href={actionHref}>{actionLabel}</JumpLink>
      </div>
    </CardShell>
  )
}

export function ExceptionQueue({
  expiringDrivers,
  openMaintenance,
  draftTrips,
  draftCount,
}: {
  expiringDrivers: FleetDriver[] | null
  openMaintenance: MaintenanceLog[] | null
  draftTrips: Trip[]
  draftCount: number
}) {
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <QueueShell
        icon={ShieldAlert}
        title="Expiring licenses"
        countLabel={
          expiringDrivers == null
            ? "Unknown"
            : `${expiringDrivers.length} in ${EXPIRY_WINDOW_DAYS} days`
        }
        countTone={
          expiringDrivers == null || expiringDrivers.length > 0
            ? "medium"
            : "low"
        }
        actionHref="/drivers"
        actionLabel="Review drivers"
      >
        {expiringDrivers == null ? (
          <EmptyNote message="Not tracked — driver records unavailable for your role." />
        ) : expiringDrivers.length === 0 ? (
          <EmptyNote message="No licenses expiring soon." />
        ) : (
          <ul className="divide-y divide-current/10">
            {expiringDrivers.slice(0, 5).map((driver) => {
              const level = urgency(driver.license_expiry_date)
              return (
                <li key={driver.license_number} className="flex items-center gap-3 py-3">
                  <IconBadge icon={ShieldAlert} className="size-9" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">
                      {driver.name}
                    </p>
                    <p className="truncate text-xs opacity-60">
                      {driver.license_number} · expires{" "}
                      {driver.license_expiry_date}
                    </p>
                  </div>
                  <span
                    className={cn(
                      "shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
                      priorityTone[level],
                    )}
                  >
                    {level}
                  </span>
                </li>
              )
            })}
          </ul>
        )}
      </QueueShell>

      <QueueShell
        icon={Wrench}
        title="Open maintenance"
        countLabel={
          openMaintenance == null ? "Unknown" : `${openMaintenance.length} open`
        }
        countTone={
          openMaintenance == null || openMaintenance.length > 0
            ? "medium"
            : "low"
        }
        actionHref="/maintenance"
        actionLabel="Review maintenance"
      >
        {openMaintenance == null ? (
          <EmptyNote message="Not tracked — maintenance records unavailable for your role." />
        ) : openMaintenance.length === 0 ? (
          <EmptyNote message="No open maintenance logs." />
        ) : (
          <ul className="divide-y divide-current/10">
            {openMaintenance.slice(0, 5).map((log) => (
              <li key={log.id} className="flex items-center gap-3 py-3">
                <IconBadge icon={Wrench} className="size-9" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">
                    {log.description}
                  </p>
                  <p className="truncate text-xs opacity-60">
                    {log.vehicle_reg} · since {log.start_date}
                  </p>
                </div>
                <span className="hidden text-xs opacity-70 md:block">
                  {log.vehicle_reg}
                </span>
                <span
                  className={cn(
                    "shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
                    priorityTone.medium,
                  )}
                >
                  {log.status || "Open"}
                </span>
              </li>
            ))}
          </ul>
        )}
      </QueueShell>

      <QueueShell
        icon={FileText}
        title="Draft backlog"
        countLabel={`${draftCount} drafts`}
        countTone={draftCount > 0 ? "medium" : "low"}
        actionHref="/trips"
        actionLabel="Review trips"
      >
        {draftTrips.length === 0 ? (
          <EmptyNote message="No draft trips waiting." />
        ) : (
          <ul className="divide-y divide-current/10">
            {draftTrips.map((trip) => (
              <li key={trip.id} className="flex items-center gap-3 py-3">
                <IconBadge icon={FileText} className="size-9" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">
                    #{trip.id} {trip.source_name} → {trip.destination_name}
                  </p>
                  <p className="truncate text-xs opacity-60">
                    {trip.cargo_weight.toLocaleString()} kg
                  </p>
                </div>
                <span
                  className={cn(
                    "shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium capitalize",
                    priorityTone.medium,
                  )}
                >
                  Draft
                </span>
              </li>
            ))}
          </ul>
        )}
      </QueueShell>
    </div>
  )
}
