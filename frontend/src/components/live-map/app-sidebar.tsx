import Link from "next/link"
import { usePathname } from "next/navigation"
import { Building2, CircleUser, FileSpreadsheet, House, LogOut, MapPinned, Settings, Truck, Users, Wrench } from "lucide-react"
import { cn } from "@/lib/utils"

interface SidebarItem {
  label: string
  icon: typeof House
  href: string
}

const ALL_ITEMS: SidebarItem[] = [
  { label: "Overview", icon: House, href: "/dashboard" },
  { label: "Live map", icon: MapPinned, href: "/live-map" },
  { label: "Trips", icon: Building2, href: "/trips" },
  { label: "Vehicles", icon: Truck, href: "/fleet" },
  { label: "Drivers", icon: Users, href: "/drivers" },
  { label: "Maintenance", icon: Wrench, href: "/maintenance" },
  { label: "Expenses", icon: FileSpreadsheet, href: "/fuel-and-expenses" },
  { label: "Analytics", icon: CircleUser, href: "/analytics" },
  { label: "Settings", icon: Settings, href: "/settings" },
]

export function getSidebarItemsForRole(role: string): SidebarItem[] {
  switch (role) {
    case "Admin":
      return ALL_ITEMS
    case "Fleet Manager":
      return ALL_ITEMS.filter((i) => i.href !== "/analytics")
    case "Driver":
      return ALL_ITEMS.filter((i) => ["/dashboard", "/live-map", "/trips"].includes(i.href))
    case "Safety Officer":
      return ALL_ITEMS.filter((i) => ["/dashboard", "/live-map", "/fleet", "/drivers", "/maintenance"].includes(i.href))
    case "Financial Analyst":
      return ALL_ITEMS.filter((i) => ["/dashboard", "/live-map", "/trips", "/fuel-and-expenses", "/analytics"].includes(i.href))
    default:
      return ALL_ITEMS.filter((i) => ["/dashboard", "/live-map"].includes(i.href))
  }
}

export function AppSidebar({ role = "Fleet Manager" }: { role?: string }) {
  const pathname = usePathname()
  const items = getSidebarItemsForRole(role)

  return (
    <aside aria-label="TransitOps navigation" className="hidden w-20 shrink-0 flex-col items-center border-r border-border bg-card py-6 md:flex">
      <Link href="/dashboard" aria-label="TransitOps home" className="grid size-10 place-items-center rounded-xl bg-primary text-primary-foreground">
        <svg viewBox="0 0 24 24" className="size-5" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
          <path d="M4 7l8 10 8-10" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </Link>

      <nav aria-label="App" className="mt-10 flex flex-1 flex-col items-center gap-3">
        {items.map(({ label, icon: Icon, href }) => {
          const active = pathname === href || (href !== "/dashboard" && pathname?.startsWith(href))
          return (
            <Link
              key={label}
              href={href}
              aria-label={label}
              aria-current={active ? "page" : undefined}
              className={cn(
                "relative grid size-11 place-items-center rounded-2xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                active && "bg-muted text-primary",
              )}
            >
              <Icon className="size-5" aria-hidden="true" />
              {active && <span className="absolute -bottom-1 size-1.5 rounded-full bg-signal" aria-hidden="true" />}
            </Link>
          )
        })}
      </nav>

      <div className="flex flex-col items-center gap-3">
        <Link href="/settings" aria-label="Settings" className="grid size-11 place-items-center rounded-2xl text-muted-foreground hover:bg-muted hover:text-foreground">
          <Settings className="size-5" aria-hidden="true" />
        </Link>
        <Link href="/login" aria-label="Sign out" className="grid size-11 place-items-center rounded-2xl text-signal hover:bg-muted">
          <LogOut className="size-5" aria-hidden="true" />
        </Link>
      </div>
    </aside>
  )
}
