export type LatLng = [number, number]

export type Stop = { name: string; at: number }

export type Route = {
  id: string
  number: number
  name: string
  from: string
  to: string
  path: LatLng[]
  stops: Stop[]
}

export type Depot = {
  id: string
  name: string
  city: string
  coords: LatLng
}

export const DEPOTS: Depot[] = [
  { id: "chicago", name: "Chicago Logistics Hub", city: "Chicago", coords: [41.8781, -87.6298] },
  { id: "dallas", name: "Dallas Gateway Hub", city: "Dallas", coords: [32.7767, -96.797] },
  { id: "atlanta", name: "Atlanta Distribution Hub", city: "Atlanta", coords: [33.749, -84.388] },
  { id: "denver", name: "Denver Intermodal Hub", city: "Denver", coords: [39.7392, -104.9903] },
  { id: "kc", name: "Kansas City Terminal", city: "Kansas City", coords: [39.0997, -94.5786] },
  { id: "la", name: "Los Angeles Freight Hub", city: "Los Angeles", coords: [34.0522, -118.2437] },
  { id: "reno", name: "Reno Hub", city: "Reno", coords: [39.5296, -119.8138] },
  { id: "seattle", name: "Seattle Terminal", city: "Seattle", coords: [47.6062, -122.3321] },
  // Endpoints
  { id: "slc", name: "Salt Lake City Hub", city: "Salt Lake City", coords: [40.7608, -111.891] },
  { id: "houston", name: "Houston Freight Depot", city: "Houston", coords: [29.7604, -95.3698] },
  { id: "portland", name: "Portland Logistics Hub", city: "Portland", coords: [45.5152, -122.6784] },
  { id: "nyc", name: "New York Central Hub", city: "New York", coords: [40.7128, -74.006] },
  { id: "phoenix", name: "Phoenix Transshipment Depot", city: "Phoenix", coords: [33.4484, -112.074] },
]

export type Fleet = { id: string; name: string }

export type Truck = {
  id: string
  code: string
  plate: string
  routeId: string
  fleetId: string
  driver: { name: string; phone: string }
  capacity: number
  cargoWeight: number
  cargoType?: string
  speed: number
  startProgress: number
  delayed?: boolean
  fuelPct?: number
  reeferTempC?: number | null
}

export type ViewerRole = "operator" | "driver"

export const fleets: Fleet[] = [
  { id: "west", name: "Western Fleet" },
  { id: "central", name: "Central Fleet" },
  { id: "east", name: "Eastern Fleet" },
]

export const routes: Route[] = [
  {
    id: "r1",
    number: 80,
    name: "Reno – Salt Lake City",
    from: "Reno Hub",
    to: "Salt Lake City Hub",
    path: [
      [39.5296, -119.8138],
      [40.8354, -117.7944],
      [40.8324, -115.7631],
      [40.7608, -111.891],
    ],
    stops: [
      { name: "Reno Hub", at: 0 },
      { name: "Winnemucca", at: 0.35 },
      { name: "Elko Depot", at: 0.65 },
      { name: "Salt Lake City Hub", at: 1 },
    ],
  },
  {
    id: "r2",
    number: 15,
    name: "Denver – Los Angeles",
    from: "Denver Intermodal Hub",
    to: "Los Angeles Freight Hub",
    path: [
      [39.7392, -104.9903],
      [39.5501, -107.3248],
      [38.7628, -112.0841],
      [36.1699, -115.1398],
      [34.0522, -118.2437],
    ],
    stops: [
      { name: "Denver Intermodal Hub", at: 0 },
      { name: "Grand Junction", at: 0.28 },
      { name: "Las Vegas Transit", at: 0.72 },
      { name: "Los Angeles Freight Hub", at: 1 },
    ],
  },
  {
    id: "r3",
    number: 65,
    name: "Chicago – Atlanta",
    from: "Chicago Logistics Hub",
    to: "Atlanta Distribution Hub",
    path: [
      [41.8781, -87.6298],
      [39.7684, -86.1581],
      [38.2527, -85.7585],
      [36.1627, -86.7816],
      [33.749, -84.388],
    ],
    stops: [
      { name: "Chicago Logistics Hub", at: 0 },
      { name: "Indianapolis Hub", at: 0.3 },
      { name: "Nashville Terminal", at: 0.68 },
      { name: "Atlanta Distribution Hub", at: 1 },
    ],
  },
  {
    id: "r4",
    number: 45,
    name: "Dallas – Houston",
    from: "Dallas Gateway Hub",
    to: "Houston Freight Depot",
    path: [
      [32.7767, -96.797],
      [31.6368, -96.1477],
      [30.7235, -95.5508],
      [29.7604, -95.3698],
    ],
    stops: [
      { name: "Dallas Gateway Hub", at: 0 },
      { name: "Corsicana Depot", at: 0.28 },
      { name: "Huntsville Yard", at: 0.65 },
      { name: "Houston Freight Depot", at: 1 },
    ],
  },
  {
    id: "r5",
    number: 5,
    name: "Seattle – Portland",
    from: "Seattle Terminal",
    to: "Portland Logistics Hub",
    path: [
      [47.6062, -122.3321],
      [47.0379, -122.9007],
      [46.1404, -122.9407],
      [45.5152, -122.6784],
    ],
    stops: [
      { name: "Seattle Terminal", at: 0 },
      { name: "Tacoma Port", at: 0.2 },
      { name: "Olympia Yard", at: 0.4 },
      { name: "Portland Logistics Hub", at: 1 },
    ],
  },
  {
    id: "r6",
    number: 35,
    name: "Kansas City – Dallas",
    from: "Kansas City Terminal",
    to: "Dallas Gateway Hub",
    path: [
      [39.0997, -94.5786],
      [37.6872, -97.3301],
      [35.4676, -97.5164],
      [32.7767, -96.797],
    ],
    stops: [
      { name: "Kansas City Terminal", at: 0 },
      { name: "Wichita Hub", at: 0.38 },
      { name: "Oklahoma City Depot", at: 0.68 },
      { name: "Dallas Gateway Hub", at: 1 },
    ],
  },
  {
    id: "r7",
    number: 80,
    name: "Chicago – New York",
    from: "Chicago Logistics Hub",
    to: "New York Central Hub",
    path: [
      [41.8781, -87.6298],
      [41.5055, -81.6917],
      [40.4406, -79.9959],
      [40.7128, -74.006],
    ],
    stops: [
      { name: "Chicago Logistics Hub", at: 0 },
      { name: "Cleveland Depot", at: 0.42 },
      { name: "Pittsburgh Terminal", at: 0.65 },
      { name: "New York Central Hub", at: 1 },
    ],
  },
  {
    id: "r8",
    number: 70,
    name: "Denver – Kansas City",
    from: "Denver Intermodal Hub",
    to: "Kansas City Terminal",
    path: [
      [39.7392, -104.9903],
      [39.3643, -101.0494],
      [38.8794, -99.3268],
      [39.0997, -94.5786],
    ],
    stops: [
      { name: "Denver Intermodal Hub", at: 0 },
      { name: "Colby Yard", at: 0.4 },
      { name: "Hays Depot", at: 0.65 },
      { name: "Kansas City Terminal", at: 1 },
    ],
  },
  {
    id: "r9",
    number: 20,
    name: "Atlanta – Dallas",
    from: "Atlanta Distribution Hub",
    to: "Dallas Gateway Hub",
    path: [
      [33.749, -84.388],
      [33.5186, -86.8104],
      [32.2988, -90.1848],
      [32.5252, -93.7502],
      [32.7767, -96.797],
    ],
    stops: [
      { name: "Atlanta Distribution Hub", at: 0 },
      { name: "Birmingham Hub", at: 0.22 },
      { name: "Jackson Yard", at: 0.52 },
      { name: "Shreveport Depot", at: 0.78 },
      { name: "Dallas Gateway Hub", at: 1 },
    ],
  },
  {
    id: "r10",
    number: 10,
    name: "Los Angeles – Phoenix",
    from: "Los Angeles Freight Hub",
    to: "Phoenix Transshipment Depot",
    path: [
      [34.0522, -118.2437],
      [33.7175, -116.2162],
      [33.6598, -114.5964],
      [33.4484, -112.074],
    ],
    stops: [
      { name: "Los Angeles Freight Hub", at: 0 },
      { name: "Indio Transfer", at: 0.35 },
      { name: "Blythe Crossing", at: 0.62 },
      { name: "Phoenix Transshipment Depot", at: 1 },
    ],
  },
]

export const trucks: Truck[] = [
  {
    id: "VH-1001",
    code: "VH-1001",
    plate: "US-8821-TX",
    routeId: "r1",
    fleetId: "west",
    driver: { name: "Marcus Vance", phone: "+1-555-1001" },
    capacity: 24000,
    cargoWeight: 18200,
    cargoType: "Refrigerated",
    speed: 1 / 180,
    startProgress: 0.42,
    delayed: false,
    fuelPct: 82,
    reeferTempC: -20.5,
  },
  {
    id: "VH-1002",
    code: "VH-1002",
    plate: "US-3419-CA",
    routeId: "r2",
    fleetId: "west",
    driver: { name: "Elena Rostova", phone: "+1-555-1002" },
    capacity: 22000,
    cargoWeight: 19800,
    cargoType: "Semi",
    speed: 1 / 140,
    startProgress: 0.65,
    delayed: true,
    fuelPct: 64,
  },
  {
    id: "VH-1003",
    code: "VH-1003",
    plate: "US-5542-IL",
    routeId: "r3",
    fleetId: "central",
    driver: { name: "Devon Reed", phone: "+1-555-1003" },
    capacity: 26000,
    cargoWeight: 21500,
    cargoType: "Refrigerated",
    speed: 1 / 160,
    startProgress: 0.28,
    delayed: false,
    fuelPct: 91,
    reeferTempC: -19.8,
  },
  {
    id: "VH-1004",
    code: "VH-1004",
    plate: "US-9012-GA",
    routeId: "r4",
    fleetId: "central",
    driver: { name: "Sarah Jenkins", phone: "+1-555-1004" },
    capacity: 24000,
    cargoWeight: 14200,
    cargoType: "Dry Van",
    speed: 1 / 120,
    startProgress: 0.82,
    delayed: false,
    fuelPct: 45,
  },
]

export const viewerProfiles = {
  operator: { name: "Dispatch Operations", subtitle: "All 10 continental corridors" },
  driver: { name: "Driver In-Cab View", subtitle: "Assigned VH-1001", truckId: "VH-1001" },
} as const

// Continental US Center
export const MAP_CENTER: LatLng = [39.8283, -98.5795]

export function getRoute(id: string) {
  return routes.find((r) => r.id === id) ?? routes[0]
}

function haversine([lat1, lng1]: LatLng, [lat2, lng2]: LatLng) {
  const R = 6371000
  const toRad = (d: number) => (d * Math.PI) / 180
  const dLat = toRad(lat2 - lat1)
  const dLng = toRad(lng2 - lng1)
  const a =
    Math.sin(dLat / 2) ** 2 + Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLng / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(a))
}

const routeMetrics = new Map(
  routes.map((r) => {
    const cumulative = [0]
    for (let i = 1; i < r.path.length; i++) {
      cumulative.push(cumulative[i - 1] + haversine(r.path[i - 1], r.path[i]))
    }
    return [r.id, { cumulative, length: cumulative[cumulative.length - 1] }]
  }),
)

export function routeLength(routeId: string) {
  return routeMetrics.get(routeId)?.length ?? 800000
}

export function positionAlong(route: Route, progress: number): { latlng: LatLng; heading: number } {
  const rm = routeMetrics.get(route.id)
  if (!rm || rm.cumulative.length === 0) {
    return { latlng: route.path[0] || [39.8283, -98.5795], heading: 0 }
  }
  const { cumulative, length } = rm
  const target = Math.min(Math.max(progress, 0), 1) * length
  let i = 1
  while (i < cumulative.length - 1 && cumulative[i] < target) i++
  const segStart = cumulative[i - 1]
  const segLen = cumulative[i] - segStart || 1
  const t = (target - segStart) / segLen
  const [aLat, aLng] = route.path[i - 1]
  const [bLat, bLng] = route.path[i]
  const heading = (Math.atan2(bLng - aLng, bLat - aLat) * 180) / Math.PI
  return { latlng: [aLat + (bLat - aLat) * t, aLng + (bLng - aLng) * t], heading }
}

export function traveledPath(route: Route, progress: number): LatLng[] {
  const rm = routeMetrics.get(route.id)
  if (!rm) return route.path
  const { cumulative, length } = rm
  const target = Math.min(Math.max(progress, 0), 1) * length
  const points = route.path.filter((_, i) => cumulative[i] < target)
  return [...points, positionAlong(route, progress).latlng]
}

const TRUCK_SPEED_MS = 24.0 // ~86 km/h long haul speed

export function etaMinutes(routeId: string, fromProgress: number, toProgress: number) {
  const meters = Math.max(toProgress - fromProgress, 0) * routeLength(routeId)
  return Math.max(1, Math.round(meters / TRUCK_SPEED_MS / 60))
}

export function nextStop(route: Route, progress: number) {
  return route.stops.find((s) => s.at > progress + 0.001) ?? route.stops[route.stops.length - 1]
}
