export type MaintenanceLog = {
  id: number
  vehicle_reg: string
  description: string
  cost: number
  start_date: string
  end_date?: string | null
  status: string
}

export type ShopVehicle = {
  registration_number: string
  model: string
  status: string
}

export function isOpen(log: MaintenanceLog): boolean {
  return log.status === "Open"
}
