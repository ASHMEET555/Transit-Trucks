export type LiveHubRef = {
  name: string
  latitude: number | null
  longitude: number | null
}

export type LiveVehicleStatus = "no_position" | "fresh" | "stale" | "old"

export type LiveVehicle = {
  vehicle_reg: string
  trip_id: number
  origin: LiveHubRef
  destination: LiveHubRef
  latitude: number | null
  longitude: number | null
  heading_deg: number | null
  speed_kmh: number | null
  recorded_at: string | null
  age_seconds: number | null
  source: string | null
  progress_m: number | null
  route_length_m: number | null
  progress_pct: number | null
  route_polyline: string | null
  route_provider: string | null
  status: LiveVehicleStatus
  reason?: string | null
}

export type LiveVehiclesResponse = {
  items: LiveVehicle[]
}
