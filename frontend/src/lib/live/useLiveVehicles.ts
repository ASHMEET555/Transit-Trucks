"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { BACKEND_URL } from "@/data/data"
import type { LiveVehicle } from "./liveTypes"

function liveUrl(): string {
  const base = BACKEND_URL.endsWith("/api/v1")
    ? BACKEND_URL.slice(0, -"/api/v1".length)
    : BACKEND_URL
  return `${base}/api/v1/live/vehicles`
}

export type LiveState = {
  vehicles: LiveVehicle[]
  lastSyncAt: string | null
  loading: boolean
  error: string | null
  refetch: () => void
}

// Polls GET /api/v1/live/vehicles every 2s while enabled, with ETag support
// (304 keeps current data). Refetch immediately on trip.* / vehicle.* events
// by calling refetch() from the event handler.
export function useLiveVehicles({ enabled = true }: { enabled?: boolean } = {}): LiveState {
  const [vehicles, setVehicles] = useState<LiveVehicle[]>([])
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const etagRef = useRef<string | null>(null)

  const refetch = useCallback(async () => {
    try {
      const headers: Record<string, string> = {}
      if (etagRef.current) headers["If-None-Match"] = etagRef.current
      const response = await fetch(liveUrl(), { headers, credentials: "include" })
      if (response.status === 304) return
      if (response.status === 401) {
        if (typeof window !== "undefined" && !window.location.pathname.includes("/login")) {
          window.location.href = "/login"
        }
        throw new Error("Unauthorized - please sign in again")
      }
      if (!response.ok) throw new Error(`Live request failed (${response.status})`)
      const tag = response.headers.get("ETag")
      if (tag) etagRef.current = tag
      const data = (await response.json()) as { items?: LiveVehicle[] }
      setVehicles(Array.isArray(data.items) ? data.items : [])
      setLastSyncAt(new Date().toISOString())
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load live vehicles")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!enabled) {
      setLoading(false)
      return
    }
    setLoading(true)
    void refetch()
    const timer = setInterval(() => void refetch(), 2000)
    return () => clearInterval(timer)
  }, [enabled, refetch])

  return { vehicles, lastSyncAt, loading, error, refetch }
}
