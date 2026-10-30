"use client"

import { useCallback, useEffect, useState } from "react"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useFleetEvents, type RealtimeStatus } from "@/lib/realtime/useFleetEvents"
import type { DriverRow } from "./driverTypes"

export interface UseDriversResult {
  drivers: DriverRow[]
  loading: boolean
  error: string | null
  reload: () => void
  realtimeStatus: RealtimeStatus
}

export function useDrivers(): UseDriversResult {
  const [drivers, setDrivers] = useState<DriverRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    try {
      setError(null)
      const res = await apiRequest<unknown>("/fleet/drivers")
      setDrivers(extractItems<DriverRow>(res))
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load drivers")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    reload()
  }, [reload])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: (event) => {
      if (event.type.startsWith("driver")) reload()
    },
    onReconnect: () => {
      reload()
    },
  })

  return { drivers, loading, error, reload, realtimeStatus }
}
