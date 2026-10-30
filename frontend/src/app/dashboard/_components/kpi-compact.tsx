"use client"

import {
  Banknote,
  Fuel,
  Gauge,
  Route,
  ShieldCheck,
  Truck,
  Users,
  Wallet,
  Wrench,
} from "lucide-react"
import { cn } from "@/lib/utils"
import type { DashboardKPIs } from "./kpi-strip"

type CompactItem = {
  label: string
  value: string
  caption: string
  tile: string
  span?: boolean
}

const PAPER_TILE = "bg-card text-card-foreground ring-1 ring-foreground/5"

export function KpiCompact({ kpis }: { kpis: DashboardKPIs }) {
  const money = (n: number) => `$${Number(n).toLocaleString()}`
  const margin = kpis.total_revenue - kpis.total_operational_cost

  const items: CompactItem[] = [
    {
      label: "Total revenue",
      value: money(kpis.total_revenue),
      caption: `Net ${money(margin)} after costs`,
      tile: "bg-primary text-primary-foreground",
      span: true,
    },
    {
      label: "Active vehicles",
      value: String(kpis.active_vehicles),
      caption: `${kpis.available_vehicles} available`,
      tile: PAPER_TILE,
    },
    {
      label: "In shop",
      value: String(kpis.in_shop_vehicles),
      caption: "Off the road now",
      tile: "bg-sand text-foreground",
    },
    {
      label: "Active trips",
      value: String(kpis.active_trips),
      caption: `${kpis.drivers_on_duty} drivers on duty`,
      tile: PAPER_TILE,
    },
    {
      label: "Drivers on duty",
      value: String(kpis.drivers_on_duty),
      caption: "Covering active trips",
      tile: "bg-mist text-foreground",
    },
    {
      label: "Utilization",
      value: `${kpis.fleet_utilization.toFixed(1)}%`,
      caption: "Share of fleet working",
      tile: "bg-sage text-foreground",
    },
    {
      label: "Fuel efficiency",
      value:
        kpis.overall_fuel_efficiency != null
          ? `${kpis.overall_fuel_efficiency.toFixed(2)} km/L`
          : "Not tracked",
      caption: "Fleet-wide average",
      tile: "bg-sand text-foreground",
    },
    {
      label: "Operational cost",
      value: money(kpis.total_operational_cost),
      caption: `Revenue ${money(kpis.total_revenue)}`,
      tile: PAPER_TILE,
    },
  ]

  const icons = [
    Banknote,
    Truck,
    Wrench,
    Route,
    Users,
    Gauge,
    Fuel,
    Wallet,
    ShieldCheck,
  ]

  return (
    <section aria-label="Key figures">
      <ul className="grid grid-cols-2 gap-2">
        {items.map((item, index) => {
          const Icon = icons[index % icons.length]
          return (
            <li
              key={item.label}
              className={cn(
                "rounded-2xl p-3",
                item.tile,
                item.span && "col-span-2",
              )}
            >
              <p className="flex items-center gap-1.5 text-[11px] font-medium opacity-70">
                <Icon className="size-3.5" aria-hidden="true" />
                {item.label}
              </p>
              <p className="mt-1 font-serif text-2xl font-semibold tracking-tight tabular-nums">
                {item.value}
              </p>
              <p className="mt-0.5 text-[11px] opacity-60">{item.caption}</p>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
