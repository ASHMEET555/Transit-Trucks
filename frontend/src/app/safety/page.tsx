"use client"

import { useProfile } from "@/lib/ProfileContext"
import { hasRouteAccess } from "@/lib/rbacConfig"
import { ComplianceSection } from "../drivers/_components/complianceSection"
import { useDrivers } from "../drivers/_components/useDrivers"

/** Safety is an alias view over the shared driver compliance section. */
export default function SafetyPage() {
  const { currentProfile, isLoaded } = useProfile()
  const { drivers, loading, error, reload, realtimeStatus } = useDrivers()

  if (isLoaded && !hasRouteAccess(currentProfile?.role || "", "safety")) {
    return (
      <div className="mx-auto max-w-xl px-4 py-16 text-center">
        <h1 className="text-2xl font-semibold text-foreground">Safety and compliance</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Your role does not have access to safety and compliance. Contact an administrator if you
          need access.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold text-foreground">Safety and compliance</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Licence validity, suspensions, and compliance alerts.
        </p>
      </header>
      <ComplianceSection
        drivers={drivers}
        loading={loading}
        error={error}
        realtimeStatus={realtimeStatus}
        onReload={reload}
      />
    </div>
  )
}
