export type Trip = {
  id: number
  source_name: string
  destination_name: string
  vehicle_reg: string
  driver_license: string
  cargo_weight: number
  planned_distance: number
  revenue: number
  status: string
}

export type Destination = {
  name: string
  region: string
}
