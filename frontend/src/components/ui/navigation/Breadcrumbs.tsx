"use client"

import { ChevronRight } from "lucide-react"
import Link from "next/link"
import { usePathname } from "next/navigation"

const labels: Record<string, string> = {
  dashboard: "Dashboard",
  fleet: "Fleet",
  drivers: "Drivers",
  trips: "Trips",
  maintenance: "Maintenance",
  "fuel-and-expenses": "Fuel & Expenses",
  analytics: "Analytics",
  settings: "Settings",
  login: "Login",
}

export function Breadcrumbs() {
  const pathname = usePathname()
  const segment = pathname.split("/").filter(Boolean)[0] || "dashboard"
  const label = labels[segment] || segment

  return (
    <nav aria-label="Breadcrumb" className="ml-2">
      <ol role="list" className="flex items-center space-x-3 text-sm">
        <li className="flex">
          <Link
            href="/dashboard"
            className="text-muted-foreground transition hover:text-foreground"
          >
            Home
          </Link>
        </li>
        <ChevronRight
          className="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <li className="flex">
          <span className="text-foreground">{label}</span>
        </li>
      </ol>
    </nav>
  )
}
