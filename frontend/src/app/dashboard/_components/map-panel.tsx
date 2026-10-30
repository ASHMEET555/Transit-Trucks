"use client"

import { useEffect, useRef, useState } from "react"
import { Map as MapIcon, Maximize2, Minimize2 } from "lucide-react"
import { cn } from "@/lib/utils"
import { CardShell } from "@/components/fleet/card-shell"
import { IconBadge } from "./states"

export function MapPanel({ children }: { children: React.ReactNode }) {
  const [expanded, setExpanded] = useState(false)
  const toggleRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!expanded) return
    toggleRef.current?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setExpanded(false)
    }
    document.addEventListener("keydown", onKeyDown)
    const previous = document.body.style.overflow
    document.body.style.overflow = "hidden"
    return () => {
      document.removeEventListener("keydown", onKeyDown)
      document.body.style.overflow = previous
    }
  }, [expanded])

  return (
    <>
      {expanded && (
        <div
          className="fixed inset-0 z-[60] bg-foreground/30"
          aria-hidden="true"
        />
      )}
      <CardShell
        as="section"
        aria-label="Live positions"
        tone="paper"
        className={cn(
          "motion-reduce:transition-none motion-reduce:hover:translate-y-0",
          expanded && "fixed inset-3 z-[70] sm:inset-6",
        )}
      >
        <div className="flex items-center gap-3">
          <IconBadge icon={MapIcon} />
          <div className="min-w-0 flex-1">
            <h2 className="text-[15px] font-medium tracking-tight">
              Live positions
            </h2>
            <p className="mt-0.5 text-sm text-muted-foreground">
              Active trips and destinations on the road network. Markers show
              estimated positions, no GPS source is connected.
            </p>
          </div>
          <button
            ref={toggleRef}
            type="button"
            onClick={() => setExpanded((value) => !value)}
            aria-expanded={expanded}
            aria-label={expanded ? "Exit full screen map" : "Expand map to full screen"}
            className="grid size-9 shrink-0 place-items-center rounded-full ring-1 ring-current/10 transition-colors hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            {expanded ? (
              <Minimize2 className="size-4" aria-hidden="true" />
            ) : (
              <Maximize2 className="size-4" aria-hidden="true" />
            )}
          </button>
        </div>
        <div
          className={cn(
            "mt-4 overflow-hidden rounded-[22px] ring-1 ring-foreground/5",
            expanded
              ? "min-h-0 flex-1"
              : "h-[360px] lg:h-[clamp(380px,calc(100vh-260px),560px)]",
          )}
        >
          {children}
        </div>
      </CardShell>
    </>
  )
}
