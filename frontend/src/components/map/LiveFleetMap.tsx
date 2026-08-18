"use client"

import dynamic from "next/dynamic"
import type { FleetMapInnerProps } from "./FleetMapInner"

const FleetMapInner = dynamic(() => import("./FleetMapInner"), {
  ssr: false,
  loading: () => (
    <div className="fleet-map-loading">
      <div className="fleet-map-loading-inner">
        <span className="fleet-map-live-dot" />
        Loading live map…
      </div>
    </div>
  ),
})

export function LiveFleetMap(props: FleetMapInnerProps) {
  return <FleetMapInner {...props} />
}
