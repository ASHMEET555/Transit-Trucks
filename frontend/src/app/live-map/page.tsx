"use client"

import { useEffect, useState } from "react"
import { LiveFleetMap } from "@/components/map/LiveFleetMap"
import { ApiError, apiRequest } from "@/lib/api"
import type { Destination, Trip } from "@/lib/tripTypes"
import { BACKEND_URL } from "@/data/data"

export default function LiveMapPage() {
  const [trips, setTrips] = useState<Trip[]>([])
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [error, setError] = useState<string | null>(null)
  const [liveEvent, setLiveEvent] = useState<string | null>(null)

  async function load() {
    try {
      setError(null)
      const [t, d] = await Promise.all([
        apiRequest<Trip[]>("/trips/"),
        apiRequest<Destination[]>("/destinations/"),
      ])
      setTrips(t)
      setDestinations(d)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to load map data")
    }
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    const wsUrl = BACKEND_URL.replace("/api", "/ws/fleet").replace("http", "ws")
    const socket = new WebSocket(wsUrl)
    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as { event?: string }
        if (!msg.event) return
        setLiveEvent(msg.event)
        load()
      } catch {
        // ignore
      }
    }
    return () => socket.close()
  }, [])

  const dispatched = trips.filter((t) => t.status === "Dispatched").length

  return (
    <div className="page-shell live-map-page">
      <div className="page-header">
        <div>
          <p className="page-eyebrow">Operations</p>
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
