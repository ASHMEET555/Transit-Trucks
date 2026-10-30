"use client"

import dynamic from "next/dynamic"
import type { FleetMapInnerProps } from "./FleetMapInner"

const FleetMapInner = dynamic(
  () => import("./FleetMapInner").then((m) => m.FleetMapInner),
  {
    ssr: false,
    loading: () => (
      <div className="grid size-full place-items-center bg-muted/40" role="status">
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <span className="size-2 animate-pulse rounded-full bg-signal motion-reduce:animate-none" aria-hidden="true" />
          Loading live map…
        </p>
      </div>
    ),
  },
)

export function LiveFleetMap(props: FleetMapInnerProps) {
  return <FleetMapInner {...props} />
}
