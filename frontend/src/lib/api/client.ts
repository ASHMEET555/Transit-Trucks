import { apiRequest, extractItems, PaginatedResponse } from "../api"
import type { components, paths } from "./schema"

export type Vehicle = components["schemas"]["Vehicle"]
export type Driver = components["schemas"]["Driver"]
export type Trip = components["schemas"]["Trip"]
export type Destination = components["schemas"]["Destination"]
export type DashboardKPIs = components["schemas"]["DashboardKPIs"]
export type DriverDashboard = components["schemas"]["DriverDashboard"]
export type MaintenanceLog = components["schemas"]["MaintenanceLog"]
export type Expense = components["schemas"]["Expense"]
export type AuditLog = components["schemas"]["AuditLog"]
export type Notification = components["schemas"]["Notification"]
export type UtilizationResponse = components["schemas"]["UtilizationResponse"]
export type LiveTripsResponse = components["schemas"]["LiveTripsResponse"]
export type LiveTrip = components["schemas"]["LiveTrip"]

// ---------------- DASHBOARD & UTILIZATION ----------------
export async function getDashboardKPIs(): Promise<DashboardKPIs> {
  return apiRequest<DashboardKPIs>("/api/v1/dashboard/kpis")
}

export async function getDriverDashboard(): Promise<DriverDashboard> {
  return apiRequest<DriverDashboard>("/api/v1/dashboard/me")
}

export async function getDashboardUtilization(days: number = 7): Promise<UtilizationResponse> {
  return apiRequest<UtilizationResponse>(`/api/v1/dashboard/utilization?days=${days}`)
}

export function getDashboardExportUrl(): string {
  return "/api/v1/dashboard/export.csv"
}

// ---------------- TRIPS & LIVE MAP ----------------
export async function listTrips(params?: {
  status?: string
  vehicle_reg?: string
  driver_license?: string
  limit?: number
  cursor?: string
}): Promise<PaginatedResponse<Trip>> {
  const q = new URLSearchParams()
  if (params?.status) q.set("status", params.status)
  if (params?.vehicle_reg) q.set("vehicle_reg", params.vehicle_reg)
  if (params?.driver_license) q.set("driver_license", params.driver_license)
  if (params?.limit) q.set("limit", String(params.limit))
  if (params?.cursor) q.set("cursor", params.cursor)
  const qs = q.toString() ? `?${q.toString()}` : ""
  return apiRequest<PaginatedResponse<Trip>>(`/api/v1/trips/${qs}`)
}

export async function createTrip(data: {
  source_name: string
  destination_name: string
  vehicle_reg?: string
  driver_license?: string
  cargo_weight: number
  planned_distance?: number
  revenue?: number
  dispatch?: boolean
}): Promise<Trip> {
  return apiRequest<Trip>("/api/v1/trips", { method: "POST", body: data })
}

export async function dispatchTrip(id: number): Promise<Trip> {
  return apiRequest<Trip>(`/api/v1/trips/${id}/dispatch`, { method: "POST" })
}

export async function completeTrip(id: number, data: {
  final_odometer: number
  fuel_liters?: number
  fuel_cost?: number
}): Promise<Trip> {
  return apiRequest<Trip>(`/api/v1/trips/${id}/complete`, { method: "POST", body: data })
}

export async function cancelTrip(id: number, reason?: string): Promise<Trip> {
  return apiRequest<Trip>(`/api/v1/trips/${id}/cancel`, { method: "POST", body: { reason } })
}

export async function suggestTripAssignment(data: {
  source_name: string
  destination_name: string
  cargo_weight: number
}): Promise<{ vehicle_reg: string; driver_license: string; reason?: string }> {
  return apiRequest("/api/v1/trips/suggest", { method: "POST", body: data })
}

export async function getLiveTrips(): Promise<LiveTripsResponse> {
  return apiRequest<LiveTripsResponse>("/api/v1/trips/live")
}

// ---------------- DESTINATIONS ----------------
export async function listDestinations(): Promise<Destination[]> {
  const res = await apiRequest<Destination[] | { items: Destination[] }>("/api/v1/destinations/")
  return extractItems(res)
}

// ---------------- VEHICLES ----------------
export async function listVehicles(params?: {
  status?: string
  include_inactive?: boolean
  limit?: number
  cursor?: string
}): Promise<PaginatedResponse<Vehicle>> {
  const q = new URLSearchParams()
  if (params?.status) q.set("status", params.status)
  if (params?.include_inactive) q.set("include_inactive", "true")
  if (params?.limit) q.set("limit", String(params.limit))
  if (params?.cursor) q.set("cursor", params.cursor)
  const qs = q.toString() ? `?${q.toString()}` : ""
  return apiRequest<PaginatedResponse<Vehicle>>(`/api/v1/fleet/vehicles${qs}`)
}

export async function createVehicle(data: {
  registration_number: string
  type: string
  max_payload: number
  odometer: number
  acquisition_cost?: number
}): Promise<Vehicle> {
  return apiRequest<Vehicle>("/api/v1/fleet/vehicles", { method: "POST", body: data })
}

export async function updateVehicle(reg: string, data: {
  type?: string
  max_payload?: number
  odometer?: number
}): Promise<Vehicle> {
  return apiRequest<Vehicle>(`/api/v1/fleet/vehicles/${reg}`, { method: "PATCH", body: data })
}

export async function retireVehicle(reg: string): Promise<Vehicle> {
  return apiRequest<Vehicle>(`/api/v1/fleet/vehicles/${reg}/retire`, { method: "POST" })
}

export async function deleteVehicle(reg: string): Promise<{ message: string; soft_deleted?: boolean }> {
  return apiRequest(`/api/v1/fleet/vehicles/${reg}`, { method: "DELETE" })
}

// ---------------- DRIVERS ----------------
export async function listDrivers(params?: {
  status?: string
  license_expiring_within_days?: number
  limit?: number
  cursor?: string
}): Promise<PaginatedResponse<Driver>> {
  const q = new URLSearchParams()
  if (params?.status) q.set("status", params.status)
  if (params?.license_expiring_within_days) {
    q.set("license_expiring_within_days", String(params.license_expiring_within_days))
  }
  if (params?.limit) q.set("limit", String(params.limit))
  if (params?.cursor) q.set("cursor", params.cursor)
  const qs = q.toString() ? `?${q.toString()}` : ""
  return apiRequest<PaginatedResponse<Driver>>(`/api/v1/fleet/drivers${qs}`)
}

export async function createDriver(data: {
  license_number: string
  full_name?: string
  license_category: string
  license_expiry: string
}): Promise<Driver> {
  return apiRequest<Driver>("/api/v1/fleet/drivers", { method: "POST", body: data })
}

export async function updateDriver(lic: string, data: {
  license_category?: string
  license_expiry?: string
}): Promise<Driver> {
  return apiRequest<Driver>(`/api/v1/fleet/drivers/${lic}`, { method: "PATCH", body: data })
}

export async function setDriverAvailable(lic: string): Promise<Driver> {
  return apiRequest<Driver>(`/api/v1/fleet/drivers/${lic}/set-available`, { method: "POST" })
}

export async function setDriverOffDuty(lic: string): Promise<Driver> {
  return apiRequest<Driver>(`/api/v1/fleet/drivers/${lic}/set-off-duty`, { method: "POST" })
}

export async function suspendDriver(lic: string): Promise<Driver> {
  return apiRequest<Driver>(`/api/v1/fleet/drivers/${lic}/suspend`, { method: "POST" })
}

export async function reinstateDriver(lic: string): Promise<Driver> {
  return apiRequest<Driver>(`/api/v1/fleet/drivers/${lic}/reinstate`, { method: "POST" })
}

// ---------------- MAINTENANCE ----------------
export async function listMaintenanceLogs(params?: {
  vehicle_reg?: string
  status?: "Open" | "Closed"
  limit?: number
  cursor?: string
}): Promise<PaginatedResponse<MaintenanceLog>> {
  const q = new URLSearchParams()
  if (params?.vehicle_reg) q.set("vehicle_reg", params.vehicle_reg)
  if (params?.status) q.set("status", params.status)
  if (params?.limit) q.set("limit", String(params.limit))
  if (params?.cursor) q.set("cursor", params.cursor)
  const qs = q.toString() ? `?${q.toString()}` : ""
  return apiRequest<PaginatedResponse<MaintenanceLog>>(`/api/v1/maintenance/${qs}`)
}

export async function createMaintenanceLog(data: {
  vehicle_reg: string
  description: string
  cost: number
  start_date: string
}): Promise<MaintenanceLog> {
  return apiRequest<MaintenanceLog>("/api/v1/maintenance/", { method: "POST", body: data })
}

export async function closeMaintenanceLog(id: number, data: {
  end_date: string
  final_cost?: number
}): Promise<MaintenanceLog> {
  return apiRequest<MaintenanceLog>(`/api/v1/maintenance/${id}/close`, { method: "POST", body: data })
}

// ---------------- FINANCE & EXPENSES ----------------
export async function listExpenses(params?: {
  trip_id?: number
  vehicle_reg?: string
  type?: string
  from?: string
  to?: string
  limit?: number
  cursor?: string
}): Promise<PaginatedResponse<Expense>> {
  const q = new URLSearchParams()
  if (params?.trip_id) q.set("trip_id", String(params.trip_id))
  if (params?.vehicle_reg) q.set("vehicle_reg", params.vehicle_reg)
  if (params?.type) q.set("type", params.type)
  if (params?.from) q.set("from", params.from)
  if (params?.to) q.set("to", params.to)
  if (params?.limit) q.set("limit", String(params.limit))
  if (params?.cursor) q.set("cursor", params.cursor)
  const qs = q.toString() ? `?${q.toString()}` : ""
  return apiRequest<PaginatedResponse<Expense>>(`/api/v1/expenses${qs}`)
}

export async function logExpense(data: {
  trip_id?: number
  vehicle_reg?: string
  type: string
  amount: number
  liters?: number
  date?: string
}): Promise<Expense> {
  return apiRequest<Expense>("/api/v1/expenses", { method: "POST", body: data })
}

export async function getFinanceSummary(): Promise<{
  total_expenses: number
  total_fuel_cost: number
  total_maintenance_cost: number
  total_other_cost: number
  vehicle_costs?: Array<{
    vehicle_reg: string
    total_cost: number
    cost_per_km?: number
  }>
}> {
  return apiRequest("/api/v1/finance/summary")
}

// ---------------- NOTIFICATIONS ----------------
export async function listNotifications(limit: number = 20): Promise<Notification[]> {
  const res = await apiRequest<Notification[] | { items: Notification[] }>(`/api/v1/notifications?limit=${limit}`)
  return extractItems(res)
}

export async function markNotificationRead(id: number): Promise<void> {
  await apiRequest(`/api/v1/notifications/${id}/read`, { method: "POST" })
}

// ---------------- AUDIT ----------------
export async function listAuditLogs(params?: {
  entity?: string
  entity_id?: string
  actor?: string
  from?: string
  to?: string
  limit?: number
  cursor?: string
}): Promise<PaginatedResponse<AuditLog>> {
  const q = new URLSearchParams()
  if (params?.entity) q.set("entity", params.entity)
  if (params?.entity_id) q.set("entity_id", params.entity_id)
  if (params?.actor) q.set("actor", params.actor)
  if (params?.from) q.set("from", params.from)
  if (params?.to) q.set("to", params.to)
  if (params?.limit) q.set("limit", String(params.limit))
  if (params?.cursor) q.set("cursor", params.cursor)
  const qs = q.toString() ? `?${q.toString()}` : ""
  return apiRequest<PaginatedResponse<AuditLog>>(`/api/v1/audit${qs}`)
}
