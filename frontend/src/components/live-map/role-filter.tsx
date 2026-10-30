"use client"

import { Building2, Navigation as SteeringWheel } from "lucide-react"
import { cn } from "@/lib/utils"

export type ViewerRole = "operator" | "driver"

const roles: { value: ViewerRole; label: string; icon: typeof Building2 }[] = [
  { value: "operator", label: "Operator view", icon: Building2 },
  { value: "driver", label: "Driver view", icon: SteeringWheel },
]

export function RoleFilter({ value, onChange }: { value: ViewerRole; onChange: (r: ViewerRole) => void }) {
  return (
    <div role="group" aria-label="View map as" className="flex items-center gap-1 rounded-2xl border border-border bg-card p-1 shadow-sm">
      {roles.map(({ value: v, label, icon: Icon }) => {
        const active = v === value
        return (
          <button
            key={v}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(v)}
            className={cn(
              "flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
              active ? "bg-signal text-signal-foreground shadow-sm" : "text-muted-foreground hover:bg-muted hover:text-foreground",
            )}
          >
            <Icon className="size-4" aria-hidden="true" />
            {label}
          </button>
        )
      })}
    </div>
  )
}
