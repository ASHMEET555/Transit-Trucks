"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import {
  Bell,
  CalendarDays,
  LayoutGrid,
  LogOut,
  Search,
  Settings,
  Truck,
  User,
  Users,
  Wrench,
} from "lucide-react"
import { FleetCard } from "@/components/fleet/fleet-card"
import { coverCards } from "@/lib/data/sample-fleet"

const rail = [
  { icon: LayoutGrid, label: "Overview", href: "/dashboard" },
  { icon: Truck, label: "Vehicles", href: "/fleet" },
  { icon: User, label: "Drivers", href: "/drivers" },
  { icon: Users, label: "Clients", href: "/clients" },
  { icon: Wrench, label: "Maintenance", href: "/maintenance" },
  { icon: CalendarDays, label: "Schedule", href: "/trips" },
]

function Rail() {
  const pathname = usePathname()
  return (
    <nav
      aria-label="Dashboard navigation"
      className="hidden w-16 shrink-0 flex-col items-center gap-2 rounded-full bg-card py-4 shadow-sm ring-1 ring-foreground/5 lg:flex"
    >
      {rail.map(({ icon: Icon, label, href }) => {
        const active = pathname === href || (href === "/dashboard" && pathname === "/")
        return (
          <Link
            key={label}
            href={href}
            aria-label={label}
            aria-current={active ? "page" : undefined}
            className={
              active
                ? "grid size-11 place-items-center rounded-full bg-foreground text-background transition-colors"
                : "grid size-11 place-items-center rounded-full text-foreground/70 transition-colors hover:bg-muted hover:text-foreground"
            }
          >
            <Icon className="size-[18px]" aria-hidden="true" />
          </Link>
        )
      })}
      <span className="my-2 h-px w-6 bg-border" aria-hidden="true" />
      <Link
        href="/settings"
        aria-label="Settings"
        className="grid size-11 place-items-center rounded-full text-foreground/70 transition-colors hover:bg-muted hover:text-foreground"
      >
        <Settings className="size-[18px]" aria-hidden="true" />
      </Link>
      <Link
        href="/login"
        aria-label="Log out"
        className="mt-auto grid size-11 place-items-center rounded-full text-foreground/70 transition-colors hover:bg-muted hover:text-foreground"
      >
        <LogOut className="size-[18px]" aria-hidden="true" />
      </Link>
    </nav>
  )
}

export function CoverDashboard() {
  return (
    <section id="cover" aria-labelledby="cover-title" className="rounded-[40px] bg-canvas p-3 sm:p-5 lg:p-6">
      <div className="flex gap-5">
        <Rail />
        <div className="min-w-0 flex-1">
          <div className="flex flex-col gap-4 px-1 pb-5 pt-1 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 id="cover-title" className="text-2xl font-semibold tracking-tight sm:text-3xl">
                Good morning, <span className="font-serif font-normal italic">Dispatch.</span>
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Here is what is moving across active depots today.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <label className="flex h-12 flex-1 items-center gap-2 rounded-full bg-card pl-5 pr-1.5 shadow-sm ring-1 ring-foreground/5 md:w-80 md:flex-none">
                <span className="sr-only">Search fleet</span>
                <input
                  type="search"
                  placeholder="Search vehicle, driver, route…"
                  className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                />
                <span className="grid size-9 place-items-center rounded-full bg-foreground text-background" aria-hidden="true">
                  <Search className="size-4" />
                </span>
              </label>
              <Link
                href="/notifications"
                aria-label="Notifications"
                className="grid size-12 place-items-center rounded-full bg-card shadow-sm ring-1 ring-foreground/5 transition-colors hover:bg-muted"
              >
                <Bell className="size-[18px]" aria-hidden="true" />
              </Link>
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <FleetCard data={coverCards.active} />
            <FleetCard data={coverCards.ontime} />
            <FleetCard data={coverCards.idle} className="sm:col-span-2 lg:col-span-1" />
            <FleetCard data={coverCards.utilization} className="sm:col-span-2" />
            <FleetCard data={coverCards.fuel} className="sm:col-span-2 lg:col-span-1" />
            <FleetCard data={coverCards.route} className="sm:col-span-2 lg:col-span-1" />
            <FleetCard data={coverCards.maintenance} className="sm:col-span-2" />
          </div>
        </div>
      </div>
    </section>
  )
}
