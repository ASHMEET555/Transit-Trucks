"use client"

import { useEffect, useState } from "react"
import { trucks, type Truck } from "./transit-data"

export type TruckStatus = "en-route" | "delayed" | "completed"

export type LiveTruck = Truck & {
  progress: number
  status: TruckStatus
  tripsCompleted: number
  /** Seconds remaining at the terminus before the next trip begins. */
  layover: number
}

export type TripLogEntry = { id: string; truckCode: string; routeId: string; at: Date }

const TICK_MS = 250
const LAYOVER_SECONDS = 8

const initial: LiveTruck[] = trucks.map((t) => ({
  ...t,
  progress: t.startProgress,
  status: t.delayed ? "delayed" : "en-route",
  tripsCompleted: 0,
  layover: 0,
}))

export function useFleetSimulation(paused: boolean) {
  const [state, setState] = useState({ fleet: initial, log: [] as TripLogEntry[] })

  useEffect(() => {
    if (paused) return
    const dt = TICK_MS / 1000
    const timer = setInterval(() => {
      setState(({ fleet, log }) => {
        const finished: TripLogEntry[] = []
        const next = fleet.map((truck): LiveTruck => {
          if (truck.status === "completed") {
            const layover = truck.layover - dt
            return layover > 0
              ? { ...truck, layover }
              : { ...truck, layover: 0, progress: 0, status: truck.delayed ? "delayed" : "en-route" }
          }
          const progress = truck.progress + truck.speed * dt
          if (progress >= 1) {
            finished.push({
              id: `${truck.id}-${truck.tripsCompleted + 1}`,
              truckCode: truck.code,
              routeId: truck.routeId,
              at: new Date(),
            })
            return {
              ...truck,
              progress: 1,
              status: "completed",
              tripsCompleted: truck.tripsCompleted + 1,
              layover: LAYOVER_SECONDS,
            }
          }
          return { ...truck, progress }
        })

        return {
          fleet: next,
          log: finished.length ? [...finished, ...log].slice(0, 20) : log,
        }
      })
    }, TICK_MS)

    return () => clearInterval(timer)
  }, [paused])

  return state
}
