import { redirect } from "next/navigation"

// The standalone live map was folded into the overview dashboard.
export default function LiveMapPage() {
  redirect("/dashboard")
}
