"use client"

import { useProfile } from "@/lib/ProfileContext"
import { usePathname, useRouter } from "next/navigation"
import Link from "next/link"
import { useEffect } from "react"
import { AppSidebar } from "@/components/live-map/app-sidebar"
import { Bell, Search } from "lucide-react"

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { currentProfile, isAuthenticated, isLoaded } = useProfile()
  const isPublicPage = pathname === "/" || pathname === "/login"
  const isAuthPage = pathname === "/login"

  useEffect(() => {
    if (!isLoaded) return
    if (!isPublicPage && !isAuthenticated) {
      router.replace("/login")
    }
    if (isAuthPage && isAuthenticated) {
      if (currentProfile?.role === "Driver") {
        router.replace("/trips")
      } else {
        router.replace("/dashboard")
      }
    }
  }, [isAuthenticated, isLoaded, isPublicPage, isAuthPage, currentProfile, router])

  if (!isLoaded) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-canvas">
        <div className="size-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (isPublicPage) {
    return <>{children}</>
  }

  if (!isAuthenticated) {
    return null
  }

  const role = currentProfile?.role || "Fleet Manager"
  const name = currentProfile?.name || "Operator"
  const initials = currentProfile?.initials || "OP"

  return (
    <div className="flex min-h-screen bg-canvas">
      <AppSidebar role={role} />
      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar with search, notifications, user profile */}
        <header className="flex h-16 items-center justify-between gap-4 border-b border-border bg-card px-4 md:px-8">
          <div className="flex min-w-0 flex-1 items-center gap-2 md:max-w-md">
            <label className="flex h-10 w-full items-center gap-2 rounded-xl bg-muted px-3 text-sm text-muted-foreground">
              <Search className="size-4" aria-hidden="true" />
              <input
                type="search"
                placeholder="Search fleet, trips, drivers…"
                className="min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
              />
            </label>
          </div>

          <div className="flex items-center gap-3">
            <Link
              href="/notifications"
              aria-label="Notifications"
              className="relative grid size-10 place-items-center rounded-xl border border-border bg-card text-foreground transition-colors hover:bg-muted"
            >
              <Bell className="size-4" aria-hidden="true" />
              <span className="absolute right-2.5 top-2.5 size-2 rounded-full bg-signal" aria-hidden="true" />
            </Link>

            <div className="flex items-center gap-3 rounded-xl border border-border bg-card py-1.5 pl-3 pr-2">
              <div className="hidden text-right text-xs sm:block">
                <p className="font-medium text-foreground">{name}</p>
                <p className="text-muted-foreground">{role}</p>
              </div>
              <span className="grid size-8 place-items-center rounded-lg bg-primary text-xs font-semibold text-primary-foreground">
                {initials}
              </span>
            </div>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-1 overflow-auto p-4 md:p-8">
          {children}
        </main>
      </div>
    </div>
  )
}
