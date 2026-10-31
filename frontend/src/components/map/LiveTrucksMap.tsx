"use client"

import dynamic from "next/dynamic"
import type { LiveTrucksMapProps } from "./LiveTrucksMapInner"

const Inner = dynamic(() => import("./LiveTrucksMapInner").then((m) => m.LiveTrucksMapInner), {
  ssr: false,
  loading: () => (
    <div className="grid size-full place-items-center bg-muted/40" role="status">
      <p className="text-sm text-muted-foreground">Loading live truck map…</p>
    </div>
  ),
})

export function LiveTrucksMap(props: LiveTrucksMapProps) {
  return <Inner {...props} />
}
