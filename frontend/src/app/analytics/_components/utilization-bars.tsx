"use client"

import { useMemo } from "react"
import { ArrowUpRight, Gauge } from "lucide-react"
import { cn } from "@/lib/utils"
import { CardShell } from "@/components/fleet/card-shell"
import {
  EmptyNote,
  IconBadge,
} from "../../dashboard/_components/states"

type TypedVehicle = {
  type: string
  status: string
}

export function UtilizationBars({ vehicles }: { vehicles: TypedVehicle[] }) {
  const groups = useMemo(() => {
    const byType = new Map<string, { total: number; active: number }>()
    for (const vehicle of vehicles) {
      const entry = byType.get(vehicle.type) ?? { total: 0, active: 0 }
      entry.total += 1
      if (vehicle.status === "On Trip") entry.active += 1
      byType.set(vehicle.type, entry)
    }
    return Array.from(byType.entries())
      .map(([label, counts]) => ({
        label,
        value:
          counts.total > 0
            ? Number(((counts.active / counts.total) * 100).toFixed(1))
            : 0,
      }))
      .sort((a, b) => b.value - a.value)
  }, [vehicles])

  const max = groups.length > 0 ? Math.max(...groups.map((g) => g.value)) : 0
  const highlight = groups.findIndex((g) => g.value === max)

  return (
    <CardShell
      as="section"
      aria-label="Utilization by vehicle type"
      tone="sage"
      tilt="left"
      className="motion-reduce:transition-none motion-reduce:hover:translate-y-0 motion-reduce:rotate-0"
    >
      <header className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <IconBadge icon={Gauge} />
          <h2 className="text-[15px] font-medium tracking-tight">
            Utilization by vehicle type
          </h2>
        </div>
        <span className="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs ring-1 ring-current/15">
          Live snapshot <ArrowUpRight className="size-3" aria-hidden="true" />
        </span>
      </header>
      {groups.length > 0 ? (
        <div
          className="mt-6 flex flex-1 items-end gap-2 sm:gap-3"
          role="list"
          aria-label="Utilization by vehicle type"
        >
          {groups.map((group, i) => {
            const active = i === highlight && max > 0
            return (
              <div
                key={group.label}
                role="listitem"
                className="flex h-full min-h-44 flex-1 flex-col items-center justify-end gap-2"
              >
                {active ? (
                  <span className="whitespace-nowrap rounded-lg bg-card px-2 py-1 text-[11px] font-medium text-card-foreground shadow-md ring-1 ring-foreground/5">
                    {group.value}%
                  </span>
                ) : null}
                <div
                  className={cn(
                    "w-full max-w-12 rounded-xl transition-all duration-500 motion-reduce:transition-none",
                    active
                      ? "hatch bg-primary"
                      : "bg-sage/70 group-hover/card:bg-sage",
                  )}
                  style={{
                    height: `${max > 0 ? (group.value / max) * 100 : 0}%`,
                  }}
                >
                  <span className="sr-only">
                    {`${group.label}: ${group.value}% on trip`}
                  </span>
                </div>
                <span
                  className={cn(
                    "max-w-full truncate text-xs",
                    active ? "font-semibold" : "opacity-60",
                  )}
                >
                  {group.label}
                </span>
              </div>
            )
          })}
        </div>
      ) : (
        <EmptyNote message="No vehicle data for utilization." />
      )}
    </CardShell>
  )
}
