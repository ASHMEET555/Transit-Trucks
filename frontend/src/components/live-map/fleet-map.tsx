"use client"

import dynamic from "next/dynamic"
import type { FleetMapInnerProps } from "../map/FleetMapInner"

const FleetMapInner = dynamic(
  () => import("../map/FleetMapInner").then((m) => m.FleetMapInner),
  {
    ssr: false,
    loading: () => (
      <div className="grid size-full place-items-center bg-muted/40" role="status">
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <span className="size-2 animate-pulse rounded-full bg-signal motion-reduce:animate-none" aria-hidden="true" />
          Loading live freight map…
        </p>
      </div>
    ),
  },
)

export type FleetMapProps = FleetMapInnerProps

export default function FleetMap(props: FleetMapProps) {
  return <FleetMapInner {...props} />
}
