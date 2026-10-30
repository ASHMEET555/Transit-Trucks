"use client"
import { Divider } from "@/components/Divider"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarLink,
  SidebarMenu,
  SidebarMenuItem,
} from "@/components/Sidebar"
import { Gauge, Truck, PersonStanding, Route, Wrench, Fuel, ChartColumn, Settings, Map, ShieldCheck, ScrollText } from "lucide-react"
import * as React from "react"
import { usePathname } from "next/navigation"
import { Logo } from "../../../../public/Logo"
import { UserProfile } from "./UserProfile"
import { useProfile } from "@/lib/ProfileContext"
import { hasRouteAccess } from "@/lib/rbacConfig"

const navigation2 = [
  {
    name: "Dashboard",
    href: "/dashboard",
    icon: Gauge,
    active: false,
    routeKey: "dashboard" as const
  },
  {
    name: "Fleet",
    href: "/fleet",
    icon: Truck,
    active: false,
    routeKey: "fleet" as const
  },
  {
    name: "Drivers",
    href: "/drivers",
    icon: PersonStanding,
    active: false,
    routeKey: "drivers" as const
  },
  {
    name: "Live Map",
    href: "/live-map",
    icon: Map,
    active: false,
    routeKey: "liveMap" as const
  },
  {
    name: "Trips",
    href: "/trips",
    icon: Route,
    active: false,
    routeKey: "trips" as const
  },
  {
    name: "Maintenance",
    href: "/maintenance",
    icon: Wrench,
    active: false,
    routeKey: "maintenance" as const
  },
  {
    name: "Fuel and Expenses",
    href: "/fuel-and-expenses",
    icon: Fuel,
    active: false,
    routeKey: "fuelAndExpenses" as const
  },
  {
    name: "Safety",
    href: "/safety",
    icon: ShieldCheck,
    active: false,
    routeKey: "safety" as const
  },
  {
    name: "Audit",
    href: "/audit",
    icon: ScrollText,
    active: false,
    routeKey: "audit" as const
  },
  {
    name: "Analytics",
    href: "/analytics",
    icon: ChartColumn,
    active: false,
    routeKey: "analytics" as const
  },
  {
    name: "Settings",
    href: "/settings",
    icon: Settings,
    active: false,
    routeKey: "settings" as const
  },
] as const

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const { currentProfile } = useProfile()
  const pathname = usePathname()

  const filteredNavigation = React.useMemo(() => {
    if (!currentProfile) return []
    return navigation2.filter((item) =>
      hasRouteAccess(currentProfile.role, item.routeKey),
    )
  }, [currentProfile])

  return (
    <Sidebar {...props} className="app-sidebar bg-card">
      <SidebarHeader className="px-3 py-4">
        <div className="flex items-center gap-3">
          <span className="flex size-9 items-center justify-center rounded-md bg-card p-1.5 shadow-sm ring-1 ring-border">
            <Logo className="size-6 text-primary dark:text-primary" />
          </span>
          <div>
            <span className="block text-sm font-semibold text-foreground">
              TransitOps
            </span>
          </div>
        </div>
      </SidebarHeader>
      <SidebarContent>
        <div className="px-3">
          <Divider className="my-0 py-0" />
        </div>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu className="space-y-1">
              {filteredNavigation.map((item) => (
                <SidebarMenuItem key={item.name}>
                  <SidebarLink
                    href={item.href}
                    isActive={pathname === item.href}
                    icon={item.icon}
                  >
                    {item.name}
                  </SidebarLink>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <div className="border-t border-border" />
        <UserProfile />
      </SidebarFooter>
    </Sidebar>
  )
}
