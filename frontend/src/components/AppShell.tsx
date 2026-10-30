"use client"

import { useProfile } from "@/lib/ProfileContext"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"
import { AppSidebar } from "@/components/live-map/app-sidebar"
import { COMPANIES, WORKSPACES } from "@/config/workspaces"
import { apiRequest, extractItems } from "@/lib/api"
import { Bell, Search } from "lucide-react"

const AVATAR_EMOJI = ["🚚", "🛻", "🚛", "🧭", "📦", "🛣️", "⛽", "🧰"]

function avatarFor(seed: string): string {
  let h = 0
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0
  return AVATAR_EMOJI[h % AVATAR_EMOJI.length]
}

type SearchResult = { kind: "Vehicle" | "Trip" | "Driver"; label: string; detail: string; href: string }

function sessionLabel(id: string | null, options: { id: string; name: string }[], fallback: string): string {
  if (!id) return fallback
  return options.find((o) => o.id === id)?.name ?? fallback
}

function readSession(key: string): string | null {
  if (typeof window === "undefined") return null
  try {
    return window.sessionStorage.getItem(key)
  } catch {
    return null
  }
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { currentProfile, isAuthenticated, isLoaded, logout } = useProfile()
  const isPublicPage = pathname === "/" || pathname === "/login"
  const isAuthPage = pathname === "/login"

  const [query, setQuery] = useState("")
  const [results, setResults] = useState<SearchResult[]>([])
  const [searching, setSearching] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [unreadCount, setUnreadCount] = useState(0)
  const [workspaceId] = useState(() => readSession("transitops.workspace"))
  const [companyId] = useState(() => readSession("transitops.company"))
  const [profileOpen, setProfileOpen] = useState(false)
  const searchBoxRef = useRef<HTMLDivElement>(null)
  const profileBoxRef = useRef<HTMLDivElement>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

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

  useEffect(() => {
    if (!isAuthenticated || isPublicPage) return
    let cancelled = false
    apiRequest<{ items: { read_at: string | null }[] }>("/notifications?limit=50")
      .then((res) => {
        if (!cancelled) setUnreadCount(res.items.filter((n) => !n.read_at).length)
      })
      .catch(() => {
        if (!cancelled) setUnreadCount(0)
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, isPublicPage, pathname])

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    const q = query.trim().toLowerCase()
    if (q.length < 2 || !isAuthenticated) {
      setResults([])
      setSearching(false)
      return
    }
    setSearching(true)
    debounceRef.current = setTimeout(async () => {
      try {
        const [vehicles, trips, drivers] = await Promise.all([
          apiRequest("/fleet/vehicles").then(extractItems<any>).catch(() => [] as any[]),
          apiRequest("/trips/").then(extractItems<any>).catch(() => [] as any[]),
          apiRequest("/fleet/drivers").then(extractItems<any>).catch(() => [] as any[]),
        ])
        const found: SearchResult[] = []
        for (const v of vehicles) {
          const label = String(v.registration_number ?? v.model ?? "")
          if (label.toLowerCase().includes(q) && found.length < 9) {
            found.push({
              kind: "Vehicle",
              label,
              detail: String(v.model ?? v.status ?? ""),
              href: "/fleet",
            })
          }
        }
        for (const t of trips) {
          const hay = `${t.vehicle_reg ?? ""} ${t.source_name ?? ""} ${t.destination_name ?? ""} ${t.id ?? ""}`.toLowerCase()
          if (hay.includes(q) && found.length < 9) {
            found.push({
              kind: "Trip",
              label: `Trip ${t.id} · ${t.vehicle_reg ?? ""}`.trim(),
              detail: `${t.source_name ?? ""} to ${t.destination_name ?? ""}`,
              href: "/trips",
            })
          }
        }
        for (const d of drivers) {
          const label = String(d.name ?? d.license_number ?? "")
          if (label.toLowerCase().includes(q) && found.length < 9) {
            found.push({
              kind: "Driver",
              label,
              detail: String(d.license_number ?? d.status ?? ""),
              href: "/drivers",
            })
          }
        }
        setResults(found)
        setSearchOpen(true)
      } catch {
        setResults([])
      } finally {
        setSearching(false)
      }
    }, 250)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [query, isAuthenticated])

  useEffect(() => {
    function onPointerDown(e: PointerEvent) {
      if (searchBoxRef.current && !searchBoxRef.current.contains(e.target as Node)) {
        setSearchOpen(false)
      }
      if (profileBoxRef.current && !profileBoxRef.current.contains(e.target as Node)) {
        setProfileOpen(false)
      }
    }
    document.addEventListener("pointerdown", onPointerDown)
    return () => document.removeEventListener("pointerdown", onPointerDown)
  }, [])

  if (!isLoaded) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-canvas" role="status">
        <div className="size-8 animate-spin rounded-full border-2 border-primary border-t-transparent motion-reduce:animate-none" />
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
  const workspaceName = sessionLabel(workspaceId, WORKSPACES, "Default workspace")
  const companyName = sessionLabel(companyId, COMPANIES, "TransitOps fleet")

  return (
    <div className="flex min-h-screen bg-canvas">
      <AppSidebar role={role} />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 items-center justify-between gap-4 border-b border-border bg-card px-4 md:px-8">
          <div ref={searchBoxRef} className="relative min-w-0 flex-1 md:max-w-md">
            <form
              role="search"
              aria-label="Search fleet, trips, and drivers"
              onSubmit={(e) => {
                e.preventDefault()
                setSearchOpen(true)
              }}
            >
              <label
                htmlFor="app-search"
                className="flex h-10 w-full items-center gap-2 rounded-xl bg-muted px-3 text-sm text-muted-foreground"
              >
                <Search className="size-4" aria-hidden="true" />
                <input
                  id="app-search"
                  type="search"
                  role="combobox"
                  aria-expanded={searchOpen && results.length > 0}
                  aria-controls="app-search-results"
                  autoComplete="off"
                  placeholder="Search fleet, trips, drivers…"
                  value={query}
                  onChange={(e) => {
                    setQuery(e.target.value)
                    setSearchOpen(true)
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") setSearchOpen(false)
                  }}
                  className="min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
                />
              </label>
            </form>
            {searchOpen && query.trim().length >= 2 && (
              <div className="absolute inset-x-0 top-12 z-[600] overflow-hidden rounded-xl border border-border bg-card shadow-xl">
                {searching ? (
                  <p className="px-4 py-3 text-sm text-muted-foreground" role="status">
                    Searching…
                  </p>
                ) : results.length === 0 ? (
                  <p className="px-4 py-3 text-sm text-muted-foreground">
                    No matches for “{query.trim()}”.
                  </p>
                ) : (
                  <ul id="app-search-results" className="max-h-80 overflow-y-auto py-1">
                    {results.map((r, i) => (
                      <li key={`${r.kind}-${r.label}-${i}`}>
                        <Link
                          href={r.href}
                          onClick={() => setSearchOpen(false)}
                          className="flex items-center gap-3 px-4 py-2 text-sm hover:bg-muted focus-visible:bg-muted focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
                        >
                          <span className="w-14 shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                            {r.kind}
                          </span>
                          <span className="min-w-0">
                            <span className="block truncate font-medium text-foreground">{r.label}</span>
                            {r.detail && (
                              <span className="block truncate text-xs text-muted-foreground">{r.detail}</span>
                            )}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}
          </div>

          <div className="flex items-center gap-3">
            <span
              className="hidden max-w-56 truncate rounded-full border border-border bg-muted px-3 py-1 text-xs text-muted-foreground lg:inline"
              title="Session info only, not an access boundary"
              aria-label={`Session workspace ${workspaceName}, company ${companyName}. Display only.`}
            >
              {workspaceName} · {companyName}
            </span>
            <Link
              href="/notifications"
              aria-label={unreadCount > 0 ? `Notifications, ${unreadCount} unread` : "Notifications, no unread"}
              className="relative grid size-10 place-items-center rounded-full border border-border bg-card text-foreground transition-colors hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            >
              <Bell className="size-4" aria-hidden="true" />
              {unreadCount > 0 && (
                <span
                  className="absolute right-2.5 top-2.5 grid size-4 place-items-center rounded-full bg-signal text-[10px] font-semibold tabular-nums text-signal-foreground"
                  aria-hidden="true"
                >
                  {unreadCount > 9 ? "9+" : unreadCount}
                </span>
              )}
            </Link>

            <div ref={profileBoxRef} className="relative">
              <button
                type="button"
                onClick={() => setProfileOpen((o) => !o)}
                aria-expanded={profileOpen}
                aria-haspopup="menu"
                aria-label={`Account: ${name}, ${role}. Open account menu.`}
                className="flex items-center gap-3 rounded-full border border-border bg-card py-1 pl-4 pr-1 transition-colors hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              >
                <span className="hidden text-right text-xs sm:block">
                  <span className="block font-medium text-foreground">{name}</span>
                  <span className="block text-muted-foreground">{role}</span>
                </span>
                <span
                  className="grid size-8 place-items-center rounded-full bg-sage text-base"
                  role="img"
                  aria-label={`${name} avatar`}
                >
                  {avatarFor(currentProfile?.email ?? name)}
                </span>
              </button>
              {profileOpen && (
                <div
                  role="menu"
                  aria-label="Account"
                  className="absolute right-0 z-[600] mt-2 w-64 overflow-hidden rounded-2xl border border-border bg-card shadow-xl"
                >
                  <div className="border-b border-border px-4 py-3">
                    <p className="truncate text-sm font-medium text-foreground">{name}</p>
                    <p className="truncate text-xs text-muted-foreground">{currentProfile?.email}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {role} · {workspaceName} · {companyName}
                    </p>
                  </div>
                  <Link
                    href="/settings"
                    role="menuitem"
                    onClick={() => setProfileOpen(false)}
                    className="block px-4 py-2.5 text-sm text-foreground hover:bg-muted focus-visible:bg-muted focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
                  >
                    Profile and settings
                  </Link>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setProfileOpen(false)
                      logout()
                    }}
                    className="block w-full px-4 py-2.5 text-left text-sm text-foreground hover:bg-muted focus-visible:bg-muted focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
                  >
                    Sign out
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-4 md:p-8">
          {children}
        </main>
      </div>
    </div>
  )
}
