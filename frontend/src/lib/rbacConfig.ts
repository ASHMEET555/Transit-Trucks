export type PermissionLevel = "no_access" | "read_only" | "read_write" | "admin"

export interface RoutePermissions {
  dashboard: PermissionLevel
  liveMap: PermissionLevel
  fleet: PermissionLevel
  drivers: PermissionLevel
  trips: PermissionLevel
  maintenance: PermissionLevel
  fuelAndExpenses: PermissionLevel
  analytics: PermissionLevel
  settings: PermissionLevel
}

export interface RoleConfig {
  name: string
  permissions: RoutePermissions
}

/** Keys MUST match backend role strings exactly */
export const RBAC_CONFIG: Record<string, RoleConfig> = {
  Admin: {
    name: "Admin",
    permissions: {
      dashboard: "admin",
      liveMap: "admin",
      fleet: "admin",
      drivers: "admin",
      trips: "admin",
      maintenance: "admin",
      fuelAndExpenses: "admin",
      analytics: "admin",
      settings: "admin",
    },
  },
  "Fleet Manager": {
    name: "Fleet Manager",
    permissions: {
      dashboard: "read_write",
      liveMap: "read_write",
      fleet: "read_write",
      drivers: "read_write",
      trips: "read_write",
      maintenance: "read_write",
      fuelAndExpenses: "read_write",
      analytics: "read_write",
      settings: "read_only",
    },
  },
  Driver: {
    name: "Driver",
    permissions: {
      dashboard: "read_write",
      liveMap: "read_write",
      fleet: "read_only",
      drivers: "read_only",
      trips: "read_write",
      maintenance: "no_access",
      fuelAndExpenses: "no_access",
      analytics: "no_access",
      settings: "no_access",
    },
  },
  "Safety Officer": {
    name: "Safety Officer",
    permissions: {
      dashboard: "read_only",
      liveMap: "read_only",
      fleet: "read_only",
      drivers: "read_write",
      trips: "read_only",
      maintenance: "no_access",
      fuelAndExpenses: "no_access",
      analytics: "no_access",
      settings: "no_access",
    },
  },
  "Financial Analyst": {
    name: "Financial Analyst",
    permissions: {
      dashboard: "read_only",
      liveMap: "read_only",
      fleet: "read_only",
      drivers: "no_access",
      trips: "read_only",
      maintenance: "no_access",
      fuelAndExpenses: "read_write",
      analytics: "read_write",
      settings: "no_access",
    },
  },
}

export function hasRouteAccess(role: string, route: keyof RoutePermissions): boolean {
  const roleConfig = RBAC_CONFIG[role]
  if (!roleConfig) return false
  return roleConfig.permissions[route] !== "no_access"
}

export function getRoutePermission(
  role: string,
  route: keyof RoutePermissions,
): PermissionLevel {
  const roleConfig = RBAC_CONFIG[role]
  if (!roleConfig) return "no_access"
  return roleConfig.permissions[route]
}

export function getAccessibleRoutes(role: string): (keyof RoutePermissions)[] {
  const roleConfig = RBAC_CONFIG[role]
  if (!roleConfig) return []
  return Object.entries(roleConfig.permissions)
    .filter(([, permission]) => permission !== "no_access")
    .map(([route]) => route as keyof RoutePermissions)
}

export function canWrite(role: string, route: keyof RoutePermissions): boolean {
  const level = getRoutePermission(role, route)
  return level === "read_write" || level === "admin"
}
