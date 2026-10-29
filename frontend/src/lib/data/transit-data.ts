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
  /** Fraction of the route covered per simulated second. */
  speed: number
  startProgress: number
  delayed?: boolean
}

// Map view mode: Operator or Driver
export type ViewerRole = "operator" | "driver"

export const fleets: Fleet[] = [
  { id: "north", name: "North Depot" },
  { id: "harbor", name: "Harbor Hub" },
]

export const routes: Route[] = [
  {
    id: "r1",
    number: 1,
    name: "Terminal Corridor",
    from: "Ferry Freight Terminal",
    to: "Mission Distribution",
    path: [
      [37.7955, -122.3937],
      [37.7897, -122.401],
      [37.7845, -122.4075],
      [37.7793, -122.414],
      [37.7752, -122.4192],
      [37.77, -122.42],
      [37.7648, -122.4196],
      [37.7586, -122.4189],
    ],
    stops: [
      { name: "Ferry Terminal", at: 0 },
      { name: "Montgomery Way", at: 0.22 },
      { name: "Powell Yard", at: 0.42 },
      { name: "Van Ness Depot", at: 0.64 },
      { name: "16th Street Logistics", at: 0.84 },
      { name: "Mission Distribution", at: 1 },
    ],
  },
  {
    id: "r2",
    number: 2,
    name: "Civic Logistics Loop",
    from: "Civic Yard",
    to: "Western Depot",
    path: [
      [37.7793, -122.4193],
      [37.7835, -122.417],
      [37.787, -122.4225],
      [37.7905, -122.4285],
      [37.788, -122.434],
      [37.7838, -122.433],
      [37.7808, -122.4262],
      [37.7793, -122.4193],
    ],
    stops: [
      { name: "Civic Yard", at: 0 },
      { name: "Polk Hub", at: 0.18 },
      { name: "Heights Way", at: 0.45 },
      { name: "Western Logistics", at: 0.68 },
      { name: "Valley Transfer", at: 0.86 },
      { name: "Civic Yard", at: 1 },
    ],
  },
  {
    id: "r3",
    number: 3,
    name: "Northern Waterfront",
    from: "North Beach Pier",
    to: "Embarcadero Terminal",
    path: [
      [37.8027, -122.4148],
      [37.8005, -122.41],
      [37.7975, -122.408],
      [37.794, -122.406],
      [37.792, -122.4],
      [37.788, -122.396],
    ],
    stops: [
      { name: "North Beach Pier", at: 0 },
      { name: "Washington Warehouse", at: 0.3 },
      { name: "Central Dock", at: 0.6 },
      { name: "Embarcadero Terminal", at: 1 },
    ],
  },
  {
    id: "r4",
    number: 4,
    name: "Industrial Corridor",
    from: "Rail Freight Depot",
    to: "Potrero Yard",
    path: [
      [37.7765, -122.3945],
      [37.781, -122.3995],
      [37.7765, -122.4055],
      [37.771, -122.408],
      [37.765, -122.402],
    ],
    stops: [
      { name: "Rail Freight Depot", at: 0 },
      { name: "Townsend Hub", at: 0.35 },
      { name: "Potrero Yard", at: 1 },
    ],
  },
  {
    id: "r5",
    number: 5,
    name: "Cross-Town Transfer",
    from: "Lower Yard",
    to: "Southwest Depot",
    path: [
      [37.773, -122.431],
      [37.7745, -122.425],
      [37.772, -122.424],
      [37.769, -122.429],
      [37.766, -122.43],
      [37.7625, -122.4348],
    ],
    stops: [
      { name: "Lower Yard", at: 0 },
      { name: "Duboce Transfer", at: 0.4 },
      { name: "Church Hub", at: 0.7 },
      { name: "Southwest Depot", at: 1 },
    ],
  },
]

export const trucks: Truck[] = [
  {
    id: "t1",
    code: "TRK-A1",
    plate: "KX-5417",
    routeId: "r2",
    fleetId: "north",
    driver: { name: "Marcus L.", phone: "415-779-6588" },
    capacity: 25000,
    cargoWeight: 18200,
    speed: 1 / 70,
    startProgress: 0.35,
  },
  {
    id: "t2",
    code: "TRK-A2",
    plate: "KX-2208",
    routeId: "r1",
    fleetId: "north",
    driver: { name: "Priya N.", phone: "415-310-4421" },
    capacity: 25000,
    cargoWeight: 21500,
    speed: 1 / 80,
    startProgress: 0.12,
  },
  {
    id: "t3",
    code: "TRK-B4",
    plate: "HB-9031",
    routeId: "r1",
    fleetId: "harbor",
    driver: { name: "Diego A.", phone: "415-552-0193" },
    capacity: 32000,
    cargoWeight: 14800,
    speed: 1 / 85,
    startProgress: 0.66,
  },
  {
    id: "t4",
    code: "TRK-B7",
    plate: "HB-4470",
    routeId: "r3",
    fleetId: "harbor",
    driver: { name: "Hana S.", phone: "415-208-7765" },
    capacity: 25000,
    cargoWeight: 9800,
    speed: 1 / 50,
    startProgress: 0.86,
  },
  {
    id: "t5",
    code: "TRK-C2",
    plate: "KX-6612",
    routeId: "r4",
    fleetId: "north",
    driver: { name: "Owen B.", phone: "415-874-3302" },
    capacity: 32000,
    cargoWeight: 28400,
    speed: 1 / 110,
    startProgress: 0.48,
    delayed: true,
  },
  {
    id: "t6",
    code: "TRK-C5",
    plate: "HB-1184",
    routeId: "r5",
    fleetId: "harbor",
    driver: { name: "Leila H.", phone: "415-617-9028" },
    capacity: 18000,
    cargoWeight: 8400,
    speed: 1 / 45,
    startProgress: 0.93,
  },
  {
    id: "t7",
    code: "TRK-A6",
    plate: "KX-3049",
    routeId: "r2",
    fleetId: "north",
    driver: { name: "Tom B.", phone: "415-449-1276" },
    capacity: 25000,
    cargoWeight: 16200,
    speed: 1 / 75,
    startProgress: 0.82,
  },
]

export const viewerProfiles = {
  operator: { name: "Dispatch Operations", subtitle: "All freight corridors" },
  driver: { name: "Driver In-Cab View", subtitle: "Assigned TRK-A2", truckId: "t2" },
} as const

export const MAP_CENTER: LatLng = [37.7815, -122.4155]

export function getRoute(id: string) {
  return routes.find((r) => r.id === id)!
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
  return routeMetrics.get(routeId)!.length
}

export function positionAlong(route: Route, progress: number): { latlng: LatLng; heading: number } {
  const { cumulative, length } = routeMetrics.get(route.id)!
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
  const { cumulative, length } = routeMetrics.get(route.id)!
  const target = Math.min(Math.max(progress, 0), 1) * length
  const points = route.path.filter((_, i) => cumulative[i] < target)
  return [...points, positionAlong(route, progress).latlng]
}

const TRUCK_SPEED_MS = 8.5

export function etaMinutes(routeId: string, fromProgress: number, toProgress: number) {
  const meters = Math.max(toProgress - fromProgress, 0) * routeLength(routeId)
  return Math.max(1, Math.round(meters / TRUCK_SPEED_MS / 60))
}

export function nextStop(route: Route, progress: number) {
  return route.stops.find((s) => s.at > progress + 0.001) ?? route.stops[route.stops.length - 1]
}
