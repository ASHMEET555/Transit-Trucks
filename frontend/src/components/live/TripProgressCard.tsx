"use client"

import { etaLabel, progressWidthPct } from "@/lib/live/liveUtils.mjs"
import type { LiveVehicle } from "@/lib/live/liveTypes"

// Dark forest trip card for the selected truck. Every number shown comes
// from the backend live endpoint; nothing here is estimated from the client.
export function TripProgressCard({ vehicle }: { vehicle: LiveVehicle | null }) {
  if (!vehicle) {
    return (
      <section
        aria-label="Selected trip"
        className="rounded-3xl bg-primary p-5 text-primary-foreground"
      >
        <p className="text-sm font-medium">No truck selected</p>
        <p className="mt-1 text-sm opacity-70">Select a truck to follow its trip.</p>
      </section>
    )
  }

  const hasPosition = vehicle.latitude != null && vehicle.longitude != null
  const pct = vehicle.progress_pct ?? null
  const remainingM =
    vehicle.route_length_m != null && vehicle.progress_m != null
      ? Math.max(0, vehicle.route_length_m - vehicle.progress_m)
      : null
  const eta = etaLabel(remainingM, vehicle.speed_kmh)

  return (
    <section aria-label={`Trip ${vehicle.trip_id}`} className="rounded-3xl bg-primary p-5 text-primary-foreground">
      <p className="text-xs uppercase tracking-widest opacity-60">Trip {vehicle.trip_id}</p>
      <p className="mt-2 font-serif text-2xl italic leading-snug">
        {vehicle.origin.name}
      </p>
      <p className="font-serif text-2xl italic leading-snug opacity-80">
        <span aria-hidden="true">→ </span>
        {vehicle.destination.name}
      </p>

      {!hasPosition ? (
        <p role="status" className="mt-4 text-sm font-medium">
          No position yet
        </p>
      ) : (
        <p className="mt-4 text-sm opacity-80" role="status">
          {vehicle.vehicle_reg} · {pct != null ? `${pct.toFixed(1)}% of route` : "progress unavailable"}
          {vehicle.route_provider === "approximate" ? " · road route pending" : ""}
        </p>
      )}

      <div
        className="mt-3 h-2 overflow-hidden rounded-full bg-primary-foreground/20"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct != null ? Math.round(pct * 10) / 10 : undefined}
        aria-label="Route progress"
      >
        <div
          className="h-full rounded-full bg-signal transition-[width] duration-700 ease-linear motion-reduce:transition-none"
          style={{ width: `${progressWidthPct(pct)}%` }}
        />
      </div>

      <div className="mt-4 flex items-center justify-between text-sm">
        <span className="inline-flex items-center gap-2 opacity-80">
          <span aria-hidden="true" className="size-2 rounded-full bg-signal" />
          {vehicle.origin.name}
        </span>
        <span className="inline-flex items-center gap-2 opacity-80">
          {vehicle.destination.name}
          <span aria-hidden="true" className="size-2 rounded-full bg-sand" />
        </span>
      </div>

      <p className="mt-3 text-sm opacity-80">
        {eta ? (
          <>
            ETA <span className="font-medium text-primary-foreground">{eta}</span>
          </>
        ) : (
          "ETA unavailable"
        )}
        {hasPosition && pct == null && " · last known progress shown"}
      </p>
    </section>
  )
}
