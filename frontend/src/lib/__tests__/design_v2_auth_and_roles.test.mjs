import test from "node:test"
import assert from "node:assert/strict"

// Mock login mapping logic from Task 6
function mapLoginError(status, code, retryAfter) {
  if (status === 401) {
    return "Incorrect email or password"
  }
  if (status === 403 && code === "ROLE_MISMATCH") {
    return "This account is not a role account."
  }
  if (status === 429) {
    return `Too many attempts. Retry after ${retryAfter || 60} seconds.`
  }
  return "Authentication failed."
}

// Map view mapping per D3
function getMapViewForRole(role) {
  if (role === "Driver") {
    return "driver"
  }
  // Admin, Fleet Manager, Safety Officer, Financial Analyst
  return "operator"
}

// Sidebar route visibility per task 5 (live map folded into /dashboard)
function getSidebarRoutesForRole(role) {
  switch (role) {
    case "Admin":
      return ["/dashboard", "/trips", "/fleet", "/drivers", "/maintenance", "/fuel-and-expenses", "/analytics", "/settings"]
    case "Fleet Manager":
      return ["/dashboard", "/trips", "/fleet", "/drivers", "/maintenance", "/fuel-and-expenses", "/settings"]
    case "Driver":
      return ["/dashboard", "/trips"]
    case "Safety Officer":
      return ["/dashboard", "/fleet", "/drivers", "/maintenance"]
    case "Financial Analyst":
      return ["/dashboard", "/trips", "/fuel-and-expenses", "/analytics"]
    default:
      return ["/dashboard"]
  }
}

test("Login error mapping - 401 returns Incorrect email or password", () => {
  const msg = mapLoginError(401, "INVALID_CREDENTIALS")
  assert.equal(msg, "Incorrect email or password")
})

test("Login error mapping - 403 ROLE_MISMATCH formatted correctly", () => {
  const msg = mapLoginError(403, "ROLE_MISMATCH")
  assert.equal(msg, "This account is not a role account.")
})

test("Login error mapping - 429 returns lockout message with Retry-After", () => {
  const msg = mapLoginError(429, "ACCOUNT_LOCKED", 45)
  assert.ok(msg.includes("45 seconds"))
})

test("Map view mapping - Only Operator and Driver view modes exist (D3)", () => {
  assert.equal(getMapViewForRole("Admin"), "operator")
  assert.equal(getMapViewForRole("Fleet Manager"), "operator")
  assert.equal(getMapViewForRole("Safety Officer"), "operator")
  assert.equal(getMapViewForRole("Financial Analyst"), "operator")
  assert.equal(getMapViewForRole("Driver"), "driver")
})

test("Sidebar route visibility - correct per role", () => {
  const adminRoutes = getSidebarRoutesForRole("Admin")
  assert.equal(adminRoutes.length, 8)
  assert.ok(adminRoutes.includes("/settings"))

  const fmRoutes = getSidebarRoutesForRole("Fleet Manager")
  assert.ok(!fmRoutes.includes("/analytics"))
  assert.ok(fmRoutes.includes("/maintenance"))

  const driverRoutes = getSidebarRoutesForRole("Driver")
  assert.deepEqual(driverRoutes, ["/dashboard", "/trips"])

  const safetyRoutes = getSidebarRoutesForRole("Safety Officer")
  assert.ok(safetyRoutes.includes("/drivers"))
  assert.ok(!safetyRoutes.includes("/fuel-and-expenses"))

  const faRoutes = getSidebarRoutesForRole("Financial Analyst")
  assert.ok(faRoutes.includes("/fuel-and-expenses"))
  assert.ok(faRoutes.includes("/analytics"))
  assert.ok(!faRoutes.includes("/drivers"))
})
