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
  type LucideIcon,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { CardShell } from "@/components/fleet/card-shell"
import type { Tone } from "@/components/fleet/types"
import { IconBadge } from "./states"

export type DashboardKPIs = {
  active_vehicles: number
  available_vehicles: number
  in_shop_vehicles: number
  active_trips: number
  drivers_on_duty: number
  fleet_utilization: number
  overall_fuel_efficiency: number | null
  total_operational_cost: number
  total_revenue: number
}

const calmMotion =
  "motion-reduce:transition-none motion-reduce:hover:translate-y-0"

type KpiItem = {
  label: string
  value: string
  caption: string
  icon: LucideIcon
  tone: Tone
  tilt?: "left" | "right"
  span?: boolean
}

function KpiCard({ item, hero }: { item: KpiItem; hero?: boolean }) {
  return (
    <CardShell
      as="div"
      tone={item.tone}
      tilt={item.tilt ?? "none"}
      className={cn(
        item.span && "sm:col-span-2",
        calmMotion,
        item.tilt && "motion-reduce:rotate-0",
      )}
    >
      <div className="flex items-center gap-3">
        <IconBadge icon={item.icon} />
        <dt className="text-[15px] font-medium tracking-tight">{item.label}</dt>
      </div>
      <div className="mt-auto pt-8">
        <dd
          className={cn(
            "font-serif font-semibold tracking-tight tabular-nums",
            hero ? "text-5xl" : "text-4xl",
          )}
        >
          {item.value}
        </dd>
        <p className="mt-1 text-xs opacity-60">{item.caption}</p>
      </div>
    </CardShell>
  )
}

export function KpiStrip({ kpis }: { kpis: DashboardKPIs }) {
  const money = (n: number) => `$${Number(n).toLocaleString()}`
  const margin = kpis.total_revenue - kpis.total_operational_cost
  const costShare =
    kpis.total_revenue > 0
      ? `${((kpis.total_operational_cost / kpis.total_revenue) * 100).toFixed(1)}% of revenue`
      : "No revenue recorded"

  const items: KpiItem[] = [
    {
      label: "Total revenue",
      value: money(kpis.total_revenue),
      caption: `Costs ${money(kpis.total_operational_cost)} · net ${money(margin)}`,
      icon: Banknote,
      tone: "forest",
      span: true,
    },
    {
      label: "Active vehicles",
      value: String(kpis.active_vehicles),
      caption: `${kpis.available_vehicles} available · ${kpis.in_shop_vehicles} in shop`,
      icon: Truck,
      tone: "paper",
    },
    {
      label: "Available vehicles",
      value: String(kpis.available_vehicles),
      caption: `${kpis.drivers_on_duty} drivers on duty`,
      icon: ShieldCheck,
      tone: "sage",
    },
    {
      label: "Vehicles in shop",
      value: String(kpis.in_shop_vehicles),
      caption: `${kpis.active_vehicles} still on the road`,
      icon: Wrench,
      tone: "sand",
    },
    {
      label: "Active trips",
      value: String(kpis.active_trips),
      caption: `${kpis.drivers_on_duty} drivers covering them`,
      icon: Route,
      tone: "paper",
    },
    {
      label: "Drivers on duty",
      value: String(kpis.drivers_on_duty),
      caption: `Across ${kpis.active_trips} active trips`,
      icon: Users,
      tone: "mist",
    },
    {
      label: "Fleet utilization",
      value: `${kpis.fleet_utilization.toFixed(1)}%`,
      caption: "Share of the fleet working now",
      icon: Gauge,
      tone: "sage",
      tilt: "left",
    },
    {
      label: "Fuel efficiency",
      value:
        kpis.overall_fuel_efficiency != null
          ? `${kpis.overall_fuel_efficiency.toFixed(2)} km/L`
          : "Not tracked",
      caption:
        kpis.overall_fuel_efficiency != null
          ? "Fleet-wide average"
          : "No readings reported",
      icon: Fuel,
      tone: "sand",
      tilt: "right",
    },
    {
      label: "Operational cost",
      value: money(kpis.total_operational_cost),
      caption: costShare,
      icon: Wallet,
      tone: "paper",
    },
  ]

  return (
    <section aria-label="Key figures">
      <dl className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {items.map((item, index) => (
          <KpiCard
            key={item.label}
            item={item}
            hero={index === 0}
          />
        ))}
      </dl>
    </section>
  )
}
