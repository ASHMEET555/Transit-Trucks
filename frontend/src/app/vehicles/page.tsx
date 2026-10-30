import VehicleRegistry from "../fleet/_components/VehicleRegistry"

// Alias route: /vehicles renders the same registry, no duplicated logic.
export default function VehiclesPage() {
  return <VehicleRegistry />
}
