import test from "node:test"
import assert from "node:assert/strict"
import fs from "node:fs"
import ts from "typescript"

const fileContent = fs.readFileSync(new URL("../rbacConfig.ts", import.meta.url), "utf8")
const result = ts.transpileModule(fileContent, {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
})

const scope = { exports: {} }
const run = new Function("exports", "module", result.outputText)
run(scope.exports, scope)
const { hasRouteAccess, getAccessibleRoutes } = scope.exports

test("RBAC - Admin role has access to all routes", () => {
  const routes = getAccessibleRoutes("Admin")
  assert.equal(routes.length, 9)
  assert.ok(hasRouteAccess("Admin", "dashboard"))
  assert.ok(hasRouteAccess("Admin", "fleet"))
  assert.ok(hasRouteAccess("Admin", "drivers"))
  assert.ok(hasRouteAccess("Admin", "liveMap"))
  assert.ok(hasRouteAccess("Admin", "trips"))
  assert.ok(hasRouteAccess("Admin", "maintenance"))
  assert.ok(hasRouteAccess("Admin", "fuelAndExpenses"))
  assert.ok(hasRouteAccess("Admin", "analytics"))
  assert.ok(hasRouteAccess("Admin", "settings"))
})

test("RBAC - Fleet Manager role has access to operational routes", () => {
  const routes = getAccessibleRoutes("Fleet Manager")
  assert.equal(routes.length, 9)
  assert.ok(hasRouteAccess("Fleet Manager", "maintenance"))
})

test("RBAC - Driver role has restricted visibility", () => {
  const routes = getAccessibleRoutes("Driver")
  assert.ok(!hasRouteAccess("Driver", "maintenance"))
  assert.ok(!hasRouteAccess("Driver", "fuelAndExpenses"))
  assert.ok(!hasRouteAccess("Driver", "analytics"))
  assert.ok(!hasRouteAccess("Driver", "settings"))
  assert.ok(hasRouteAccess("Driver", "dashboard"))
  assert.ok(hasRouteAccess("Driver", "trips"))
})

test("RBAC - Safety Officer role has restricted visibility", () => {
  const routes = getAccessibleRoutes("Safety Officer")
  assert.ok(hasRouteAccess("Safety Officer", "drivers"))
  assert.ok(!hasRouteAccess("Safety Officer", "maintenance"))
  assert.ok(!hasRouteAccess("Safety Officer", "fuelAndExpenses"))
})

test("RBAC - Financial Analyst role has finance routes visible", () => {
  const routes = getAccessibleRoutes("Financial Analyst")
  assert.ok(hasRouteAccess("Financial Analyst", "fuelAndExpenses"))
  assert.ok(hasRouteAccess("Financial Analyst", "analytics"))
  assert.ok(!hasRouteAccess("Financial Analyst", "drivers"))
  assert.ok(!hasRouteAccess("Financial Analyst", "maintenance"))
})
