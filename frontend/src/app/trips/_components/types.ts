export type TripStatus = "Draft" | "Dispatched" | "Completed" | "Cancelled"

export const TRIP_STATUSES: TripStatus[] = [
  "Draft",
  "Dispatched",
  "Completed",
  "Cancelled",
]

export interface Trip {
  id: number
  source_name: string
  destination_name: string
  vehicle_reg: string
  driver_license: string
  cargo_weight: number
  planned_distance: number
  revenue: number
  status: string
  start_odometer?: number | null
  final_odometer?: number | null
  fuel_liters?: number | null
  fuel_cost?: number | null
  created_at?: string
  updated_at?: string
  completed_at?: string | null
}

export interface VehicleOption {
  registration_number: string
  model: string
  max_load_capacity: number
  status: string
  odometer: number
}

export interface DriverOption {
  license_number: string
  name: string
  status: string
  license_expiry_date: string
}

export interface DestinationOption {
  name: string
  region: string
}

export interface RankedPair {
  vehicle_reg: string
  vehicle_model: string
  vehicle_type: string
  driver_license: string
  driver_name: string
  capacity_utilization: number
  score: number
}

export interface IneligibleCandidate {
  type: string
  id: string
  reason: string
}

export interface SuggestResponse {
  source_name?: string
  destination_name?: string
  cargo_weight: number
  ranked_pairs: RankedPair[]
  ineligible: IneligibleCandidate[]
  suggested_vehicle_reg: string
  suggested_driver_license: string
  suggested_driver_name: string
  capacity_utilization: number
  heuristic_score: number
}

export interface AuditEntry {
  id: number
  entity: string
  entity_id: string
  action: string
  actor_email?: string | null
  actor?: string | null
  details?: Record<string, unknown> | null
  at: string
}

export interface ExpenseRow {
  id: number
  trip_id?: number | null
  vehicle_reg?: string | null
  type: string
  liters?: number | null
  cost: number
  amount: number
  date: string
}

const REASON_LABELS: Record<string, string> = {
  VEHICLE_UNAVAILABLE: "Vehicle unavailable",
  OVERLOADED: "Overloaded for this cargo",
  DRIVER_UNAVAILABLE: "Driver unavailable",
  LICENSE_EXPIRED: "License expired",
  LICENSE_CATEGORY_MISMATCH: "License category mismatch",
}

export function ineligibilityLabel(reason: string): string {
  return (
    REASON_LABELS[reason] ||
    reason
      .toLowerCase()
      .split("_")
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(" ")
  )
}

export function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return "—"
  return value.toLocaleString()
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
