/**
 * Shared live-map helpers (plain JS so both TS components and node --test
 * suites import the exact same code).
 *
 * Source model: every position carries a source. The UI labels it from the
 * source — "Simulated" for simulator data, "Live" for any real source.
 * Never label simulator data as live, and never label real data as simulated.
 */

/**
 * Decode a precision-6 encoded polyline into [[lat, lon], ...].
 * @param {string} s
 * @returns {Array<[number, number]>}
 */
export function decodePolyline6(s) {
  const pts = []
  let lat = 0
  let lon = 0
  let i = 0
  const read = () => {
    let result = 0
    let shift = 0
    for (;;) {
      if (i >= s.length) throw new Error("truncated polyline")
      const b = s.charCodeAt(i++) - 63
      result |= (b & 0x1f) << shift
      shift += 5
      if (b < 0x20) break
    }
    return result & 1 ? ~(result >> 1) : result >> 1
  }
  while (i < s.length) {
    lat += read()
    lon += read()
    pts.push([lat / 1e6, lon / 1e6])
  }
  return pts
}

/**
 * Human age bucket: "4 s", "6 min", "2 h".
 * @param {number} ageSeconds
 */
export function formatAge(ageSeconds) {
  const age = Math.max(0, Math.floor(ageSeconds))
  if (age < 60) return `${age} s`
  if (age < 3600) return `${Math.floor(age / 60)} min`
  return `${Math.floor(age / 3600)} h`
}

/**
 * Exact status pill labels. Inputs are backend-computed only.
 * @param {{source?: string|null, ageSeconds?: number|null, hasPosition: boolean, paused: boolean}} v
 */
export function statusPillLabel({ source, ageSeconds, hasPosition, paused }) {
  if (paused) return "Paused, not updating"
  if (!hasPosition) return "No position yet"
  const age = ageSeconds ?? 0
  const fresh = age < 60
  if (source === "simulator") {
    return fresh
      ? `Simulated · updated ${formatAge(age)} ago`
      : `Simulated · last update ${formatAge(age)} ago`
  }
  // Any real source reads as Live; stale real data is labelled Stale.
  return fresh
    ? `Live · updated ${formatAge(age)} ago`
    : `Stale · last update ${formatAge(age)} ago`
}

/**
 * ETA from backend values only: remaining_m and current speed_kmh.
 * Returns null when speed is unknown (caller renders "ETA unavailable").
 * @param {number|null|undefined} remainingM
 * @param {number|null|undefined} speedKmh
 */
export function etaLabel(remainingM, speedKmh) {
  if (remainingM == null || speedKmh == null || speedKmh <= 0) return null
  const mins = remainingM / ((speedKmh * 1000) / 3600) / 60
  if (mins < 1) return "est. <1 min"
  if (mins < 60) return `est. ${Math.round(mins)} min`
  const h = Math.floor(mins / 60)
  const m = Math.round(mins % 60)
  return m === 0 ? `est. ${h} h` : `est. ${h} h ${m} min`
}

/**
 * Searchable dropdown filter: matches reg, trip id, origin, destination.
 * @param {Array<{vehicle_reg: string, trip_id: number, origin: {name: string}, destination: {name: string}}>} vehicles
 * @param {string} query
 */
export function filterTrucks(vehicles, query) {
  const q = query.trim().toLowerCase()
  if (!q) return vehicles
  return vehicles.filter((v) =>
    [v.vehicle_reg, String(v.trip_id), v.origin?.name, v.destination?.name].some(
      (f) => (f ?? "").toLowerCase().includes(q),
    ),
  )
}

/**
 * Progress bar width: the API progress_pct clamped to [0, 100].
 * @param {number|null|undefined} pct
 */
export function progressWidthPct(pct) {
  if (pct == null || Number.isNaN(pct)) return 0
  return Math.min(100, Math.max(0, pct))
}
export function clusterMarkers(items, project, radiusPx = 40) {
  const clusters = []
  for (const item of items) {
    const p = project(item.latitude, item.longitude)
    let hit = null
    for (const c of clusters) {
      const dx = c.x - p.x
      const dy = c.y - p.y
      if (dx * dx + dy * dy < radiusPx * radiusPx) {
        hit = c
        break
      }
    }
    if (hit) {
      hit.members.push(item)
    } else {
      clusters.push({
        key: item.vehicle_reg,
        lat: item.latitude,
        lon: item.longitude,
        x: p.x,
        y: p.y,
        members: [item],
      })
    }
  }
  return clusters
}
