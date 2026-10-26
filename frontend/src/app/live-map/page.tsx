"use client"

import { useEffect, useState } from "react"
import { LiveFleetMap } from "@/components/map/LiveFleetMap"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import type { Destination, Trip } from "@/lib/tripTypes"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"

export default function LiveMapPage() {
  const [trips, setTrips] = useState<Trip[]>([])
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [error, setError] = useState<string | null>(null)
  const [liveEvent, setLiveEvent] = useState<string | null>(null)

  async function load() {
    try {
      setError(null)
      const [t, d] = await Promise.all([
        apiRequest<any>("/trips/"),
        apiRequest<any>("/destinations/"),
      ])
      setTrips(extractItems<Trip>(t))
      setDestinations(extractItems<Destination>(d))
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load map data")
    }
  }

  useEffect(() => {
    load()
  }, [])

  const { status } = useFleetEvents({
    onEvent: (event) => {
      setLiveEvent(event.type)
      load()
    },
    onReconnect: () => {
      load()
    },
  })

  const dispatched = trips.filter((t) => t.status === "Dispatched").length

  return (
    <div className="page-shell live-map-page">
      <div className="page-header">
        <div>
          <div className="flex items-center gap-3">
            <p className="page-eyebrow">Operations</p>
            <RealtimeIndicator status={status} />
          </div>
          <h1 className="page-title">Live fleet map</h1>
          <p className="page-subtitle">
            Real-world road routing with live truck movement. Trips appear as
            soon as they&apos;re dispatched.
          </p>
        </div>
        <div className="live-map-stats">
          <div className="stat-chip">
            <span className="fleet-map-live-dot" />
            {dispatched} in transit
          </div>
          {liveEvent && <div className="stat-chip muted">Last: {liveEvent}</div>}
        </div>
      </div>

      {error && <p className="page-error">{error}</p>}

      <div className="live-map-frame">
        <LiveFleetMap
          trips={trips}
          destinations={destinations}
          height="calc(100vh - 220px)"
        />
      </div>
    </div>
  )
}
