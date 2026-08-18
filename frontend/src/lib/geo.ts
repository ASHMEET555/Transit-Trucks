export type LatLng = { lat: number; lng: number }

const REGION_CENTERS: Record<string, LatLng> = {
  North: { lat: 42.5, lng: -83.0 },
  South: { lat: 32.0, lng: -84.5 },
  East: { lat: 40.7, lng: -74.0 },
  West: { lat: 34.0, lng: -118.2 },
  Central: { lat: 39.8, lng: -98.5 },
  Coastal: { lat: 30.3, lng: -81.6 },
  Midwest: { lat: 41.9, lng: -87.6 },
  Mountain: { lat: 39.7, lng: -104.9 },
  Gulf: { lat: 29.8, lng: -95.4 },
  "Great Lakes": { lat: 43.0, lng: -87.9 },
}

const KNOWN_HUBS: Record<string, LatLng> = {
  "Warehouse Alpha": { lat: 42.3314, lng: -83.0458 },
  "Distribution Hub Beta": { lat: 33.749, lng: -84.388 },
  "Port Logistics Gamma": { lat: 40.7128, lng: -74.006 },
  "Crossdock Delta": { lat: 34.0522, lng: -118.2437 },
  "Cold Storage Epsilon": { lat: 39.0997, lng: -94.5786 },
  "Rail Yard Zeta": { lat: 32.0809, lng: -81.0912 },
  "Airport Cargo Eta": { lat: 41.8781, lng: -87.6298 },
  "Industrial Park Theta": { lat: 39.7392, lng: -104.9903 },
  "Fulfillment Center Iota": { lat: 29.7604, lng: -95.3698 },
  "Bonded Warehouse Kappa": { lat: 43.0389, lng: -87.9065 },
  "Intermodal Hub Lambda": { lat: 44.9778, lng: -93.265 },
  "Regional DC Mu": { lat: 35.2271, lng: -80.8431 },
  "Maple Transit Hub": { lat: 43.6532, lng: -79.3832 },
  "Westfield Hub": { lat: 37.7749, lng: -122.4194 },
  "Oakwood Warehouse": { lat: 47.6062, lng: -122.3321 },
  "Blue Ridge Depot": { lat: 38.0293, lng: -78.4767 },
  "Prairie Crossdock": { lat: 41.2565, lng: -95.9345 },
  "Gateway Intermodal": { lat: 38.627, lng: -90.1994 },
  "Harbor Depot Pi": { lat: 32.7157, lng: -117.1611 },
  "Summit Crossdock": { lat: 39.9612, lng: -82.9988 },
  "Valley Transfer Nexus": { lat: 36.1699, lng: -115.1398 },
  "Desert Staging Yard": { lat: 33.4484, lng: -112.074 },
  "Stonegate DC": { lat: 39.9526, lng: -75.1652 },
}

function hashString(value: string): number {
  let hash = 0
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash << 5) - hash + value.charCodeAt(i)
    hash |= 0
  }
  return Math.abs(hash)
}

export function getDestinationCoords(
  name: string,
  region = "Central",
): LatLng {
  if (KNOWN_HUBS[name]) return KNOWN_HUBS[name]
  const center = REGION_CENTERS[region] ?? REGION_CENTERS.Central
  const hash = hashString(name)
  const latOffset = ((hash % 1000) / 1000 - 0.5) * 4
  const lngOffset = (((hash >> 10) % 1000) / 1000 - 0.5) * 6
  return {
    lat: center.lat + latOffset,
    lng: center.lng + lngOffset,
  }
}

const routeCache = new Map<string, LatLng[]>()

function cacheKey(a: LatLng, b: LatLng): string {
  return `${a.lat.toFixed(4)},${a.lng.toFixed(4)}->${b.lat.toFixed(4)},${b.lng.toFixed(4)}`
}

export async function fetchRoadRoute(
  from: LatLng,
  to: LatLng,
): Promise<LatLng[]> {
  const key = cacheKey(from, to)
  const cached = routeCache.get(key)
  if (cached) return cached

  const coords = `${from.lng},${from.lat};${to.lng},${to.lat}`
  const url = `https://router.project-osrm.org/route/v1/driving/${coords}?overview=full&geometries=geojson`

  try {
    const res = await fetch(url)
    if (!res.ok) throw new Error("OSRM failed")
    const data = (await res.json()) as {
      routes?: Array<{ geometry?: { coordinates?: [number, number][] } }>
    }
    const line = data.routes?.[0]?.geometry?.coordinates
    if (!line?.length) throw new Error("No route")
    const path = line.map(([lng, lat]) => ({ lat, lng }))
    routeCache.set(key, path)
    return path
  } catch {
    const fallback = [from, to]
    routeCache.set(key, fallback)
    return fallback
  }
}

function segmentLength(a: LatLng, b: LatLng): number {
  const dx = b.lng - a.lng
  const dy = b.lat - a.lat
  return Math.sqrt(dx * dx + dy * dy)
}

export function routeLength(path: LatLng[]): number {
  let total = 0
  for (let i = 1; i < path.length; i += 1) {
    total += segmentLength(path[i - 1], path[i])
  }
  return total
}

/** Approximate route distance in km (lat/lng degree units → km). */
export function routeDistanceKm(path: LatLng[]): number {
  return routeLength(path) * 111
}

export function bearingBetween(from: LatLng, to: LatLng): number {
  const dLng = to.lng - from.lng
  const dLat = to.lat - from.lat
  if (Math.abs(dLng) < 1e-8 && Math.abs(dLat) < 1e-8) return 0
  return (Math.atan2(dLng, dLat) * 180) / Math.PI
}

/** Smooth, realistic animation duration — ~3 min per 100 km, min 4 min, max 20 min. */
export function estimateAnimationDurationMs(
  path: LatLng[],
  plannedDistanceKm?: number,
): number {
  const km = plannedDistanceKm && plannedDistanceKm > 0
    ? plannedDistanceKm
    : routeDistanceKm(path)
  const ms = km * 1800
  return Math.max(240_000, Math.min(1_200_000, ms))
}

export function interpolateAlongRoute(
  path: LatLng[],
  progress: number,
): { position: LatLng; bearing: number } {
  if (path.length === 0) {
    return { position: { lat: 0, lng: 0 }, bearing: 0 }
  }
  if (path.length === 1) {
    return { position: path[0], bearing: 0 }
  }

  const clamped = Math.max(0, Math.min(1, progress))
  const total = routeLength(path)
  if (total === 0) {
    return { position: path[0], bearing: 0 }
  }

  const target = total * clamped
  let walked = 0
  let position = path[0]
  let segmentBearing = bearingBetween(path[0], path[1])

  for (let i = 1; i < path.length; i += 1) {
    const seg = segmentLength(path[i - 1], path[i])
    if (walked + seg >= target) {
      const t = seg === 0 ? 0 : (target - walked) / seg
      position = {
        lat: path[i - 1].lat + (path[i].lat - path[i - 1].lat) * t,
        lng: path[i - 1].lng + (path[i].lng - path[i - 1].lng) * t,
      }
      segmentBearing = bearingBetween(path[i - 1], path[i])
      break
    }
    walked += seg
    if (i === path.length - 1) {
      position = path[i]
      segmentBearing = bearingBetween(path[i - 1], path[i])
    }
  }

  const lookAhead = Math.min(1, clamped + 0.015)
  const lookTarget = total * lookAhead
  walked = 0
  let lookPosition = position

  for (let i = 1; i < path.length; i += 1) {
    const seg = segmentLength(path[i - 1], path[i])
    if (walked + seg >= lookTarget) {
      const t = seg === 0 ? 0 : (lookTarget - walked) / seg
      lookPosition = {
        lat: path[i - 1].lat + (path[i].lat - path[i - 1].lat) * t,
        lng: path[i - 1].lng + (path[i].lng - path[i - 1].lng) * t,
      }
      break
    }
    walked += seg
    if (i === path.length - 1) lookPosition = path[i]
  }

  const bearing =
    clamped >= 0.999
      ? segmentBearing
      : bearingBetween(position, lookPosition)

  return { position, bearing }
}

export const DEMO_ROUTES: Array<{
  id: number
  label: string
  from: LatLng
  to: LatLng
  color: string
}> = [
  {
    id: 1,
    label: "TX-401",
    from: { lat: 29.7604, lng: -95.3698 },
    to: { lat: 32.7767, lng: -96.797 },
    color: "#06b6d4",
  },
  {
    id: 2,
    label: "CA-118",
    from: { lat: 34.0522, lng: -118.2437 },
    to: { lat: 32.7157, lng: -117.1611 },
    color: "#3b82f6",
  },
  {
    id: 3,
    label: "IL-290",
    from: { lat: 41.8781, lng: -87.6298 },
    to: { lat: 39.7684, lng: -86.1581 },
    color: "#8b5cf6",
  },
]
