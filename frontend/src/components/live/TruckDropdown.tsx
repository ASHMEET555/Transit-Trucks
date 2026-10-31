"use client"

import { useMemo, useRef, useState } from "react"
import { ChevronDown, Search, Truck } from "lucide-react"
import { cn } from "@/lib/utils"
import { filterTrucks } from "@/lib/live/liveUtils.mjs"
import { statusPillLabel } from "@/lib/live/liveUtils.mjs"
import type { LiveVehicle } from "@/lib/live/liveTypes"

export function TruckDropdown({
  vehicles,
  value,
  paused,
  onChange,
}: {
  vehicles: LiveVehicle[]
  value: string | null
  paused: boolean
  onChange: (reg: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)
  // Snapshot the list when the menu opens so the 2 s live poll cannot
  // remount options (and steal clicks) while the user is choosing.
  const [menuVehicles, setMenuVehicles] = useState<LiveVehicle[] | null>(null)

  const openMenu = () => {
    setMenuVehicles(vehicles)
    setQuery("")
    setOpen(true)
    requestAnimationFrame(() => inputRef.current?.focus())
  }

  const options = useMemo(
    () => filterTrucks(menuVehicles ?? vehicles, query),
    [menuVehicles, vehicles, query],
  )
  const selected = vehicles.find((v) => v.vehicle_reg === value) ?? null

  const pillFor = (v: LiveVehicle) =>
    statusPillLabel({
      source: v.source,
      ageSeconds: v.age_seconds,
      hasPosition: v.latitude != null && v.longitude != null,
      paused,
    })

  return (
    <div className="relative min-w-0">
      <button
        type="button"
        onClick={() => {
          if (open) {
            setOpen(false)
            setMenuVehicles(null)
          } else {
            openMenu()
          }
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="flex w-full items-center gap-2 rounded-xl bg-card px-3 py-2 text-left ring-1 ring-foreground/10 transition-colors hover:bg-muted/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      >
        <Truck className="size-4 shrink-0 text-primary" aria-hidden="true" />
        <span className="min-w-0 flex-1 truncate text-sm font-medium">
          {selected ? selected.vehicle_reg : vehicles.length === 0 ? "No trucks on active trips" : "Select truck"}
        </span>
        {selected && (
          <span className="hidden truncate text-xs text-muted-foreground sm:inline">{pillFor(selected)}</span>
        )}
        <ChevronDown className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")} aria-hidden="true" />
      </button>

      {open && (
        <>
          <button
            type="button"
            aria-label="Close truck list"
            className="fixed inset-0 z-10 cursor-default bg-transparent"
            onClick={() => {
              setOpen(false)
              setMenuVehicles(null)
            }}
          />
          <div className="absolute z-20 mt-2 w-full min-w-64 overflow-hidden rounded-xl bg-card shadow-xl ring-1 ring-foreground/10">
            <div className="flex items-center gap-2 border-b border-border px-3 py-2">
              <Search className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              <input
                ref={inputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search trucks…"
                aria-label="Search trucks"
                className="w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              />
            </div>
            <ul role="listbox" aria-label="Trucks on active trips" className="max-h-64 overflow-auto py-1">
              {options.length === 0 && (
                <li className="px-3 py-2 text-sm text-muted-foreground">No trucks match.</li>
              )}
              {options.map((v) => (
                <li key={v.vehicle_reg} role="option" aria-selected={v.vehicle_reg === value}>
                  <button
                    type="button"
                    onClick={() => {
                      onChange(v.vehicle_reg)
                      setOpen(false)
                      setMenuVehicles(null)
                    }}
                    className={cn(
                      "flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-muted/60 focus-visible:bg-muted/60 focus-visible:outline-none",
                      v.vehicle_reg === value && "bg-muted/60 font-medium",
                    )}
                  >
                    <Truck className="size-4 shrink-0 text-primary" aria-hidden="true" />
                    <span className="min-w-0 flex-1 truncate">{v.vehicle_reg}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {vehicles.find((full) => full.vehicle_reg === v.vehicle_reg)?.status.replace("_", " ")}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </>
      )}
    </div>
  )
}
