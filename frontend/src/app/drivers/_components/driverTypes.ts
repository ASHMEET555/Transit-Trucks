"use client"

export type DriverRow = {
  license_number: string
  name: string
  license_category: string
  license_expiry_date: string
  contact_number: string
  safety_score: number
  status: string
}

export type ExpiryState = "expired" | "expiring" | "valid"

export function todayKey(): string {
  return new Date().toISOString().slice(0, 10)
}

export function plusDaysKey(days: number): string {
  return new Date(Date.now() + days * 86400000).toISOString().slice(0, 10)
}

/** Classify a YYYY-MM-DD licence expiry date. Missing dates count as valid. */
export function expiryStateOf(expiryDate: string, today: string, horizon: string): ExpiryState {
  if (!expiryDate) return "valid"
  if (expiryDate < today) return "expired"
  if (expiryDate <= horizon) return "expiring"
  return "valid"
}

export type SafetyBand = "High" | "Adequate" | "Needs review"

/** Short display label for a recorded safety score. The score stays the source value. */
export function safetyBand(score: number): SafetyBand {
  if (score >= 90) return "High"
  if (score >= 70) return "Adequate"
  return "Needs review"
}

/** Suspend and reinstate are limited to Safety Officer and Admin roles. */
export function canModerateDriver(role: string | undefined): boolean {
  return role === "Admin" || role === "Safety Officer"
}

/** Friendly next-step copy for 409 conflicts on suspend/reinstate. */
export function conflictCopy(action: "suspend" | "reinstate", detail: string): string {
  const base =
    action === "suspend"
      ? "This driver is on a trip and cannot be suspended. Wait until the trip ends, then try again."
      : "Only suspended drivers can be reinstated. Refresh the list to see the current status, then try again."
  return detail && detail !== base ? `${base} Details: ${detail}` : base
}
