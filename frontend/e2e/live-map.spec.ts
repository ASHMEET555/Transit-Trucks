import { test, expect, Page, BrowserContext } from "@playwright/test"
import AxeBuilder from "@axe-core/playwright"
import { execSync } from "node:child_process"
import { randomUUID } from "node:crypto"
import path from "node:path"

const API = "http://localhost:8000/api/v1"
const LOG_DIR = path.join(__dirname, "../../docs/live-map/logs")

// Nine-point road-style fixture polyline (same shape as the backend fixture).
const FIXTURE_POLYLINE =
  "gd`{nAn}ncfDfd~Onrbt@~flW~`jnA~flW~hwfB~flW~fayB~flW~dkkC~reK~lxcD~hbE~xqwC_qkAvpprF"

function decodeCount(poly: string): number {
  let lat = 0
  let lon = 0
  let i = 0
  let n = 0
  const read = () => {
    let result = 0
    let shift = 0
    for (;;) {
      const b = poly.charCodeAt(i++) - 63
      result |= (b & 0x1f) << shift
      shift += 5
      if (b < 0x20) break
    }
    return result & 1 ? ~(result >> 1) : result >> 1
  }
  while (i < poly.length) {
    lat += read()
    lon += read()
    n++
  }
  return n
}

test.describe("Live map: simulated trucks on road routes", () => {
  test.setTimeout(120000)
  const suffix = String(Date.now() % 1000000)
  const hubA = `E2E Hub A ${suffix}`
  const hubB = `E2E Hub B ${suffix}`
  const regs = [`VH-E2E-${suffix}-1`, `VH-E2E-${suffix}-2`]
  const lics = [`DL-E2E-${suffix}-1`, `DL-E2E-${suffix}-2`]
  const tripIds: number[] = []
  let sharedCtx: BrowserContext
  let fixtureToken = ""
  const creds = {
    email: process.env.E2E_FM_EMAIL,
    password: process.env.E2E_FM_PASSWORD,
  }

  async function login(page: Page) {
    if (!creds.email || !creds.password) throw new Error("Missing E2E FM credentials")
    await page.goto("/login")
    // Default expected role is already Fleet Manager; no role click needed.
    await page.fill('input[type="email"]', creds.email)
    await page.fill('input[type="password"]', creds.password!)
    await page.click('button[type="submit"]')
    await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 15000 })
  }

  // Retry on 429: the file shares one IP budget with the 2 s live polls.
  async function withRetry<T>(fn: () => Promise<{ status: () => number; text: () => Promise<string>; json: () => Promise<T> }>): Promise<T> {
    let last = ""
    for (let attempt = 0; attempt < 4; attempt++) {
      const res = await fn()
      if (res.status() !== 429) {
        expect(res.status(), await res.text()).toBeLessThan(300)
        return res.json()
      }
      last = await res.text()
      await new Promise((r) => setTimeout(r, 8000))
    }
    throw new Error(`still rate limited after retries: ${last}`)
  }

  async function api(page: Page, method: string, route: string, data?: unknown) {
    return withRetry(() =>
      page.request.fetch(`${API}${route}`, {
        method,
        data,
        headers: { "Idempotency-Key": randomUUID() },
      }),
    )
  }

  async function apiToken(req: import("@playwright/test").APIRequestContext, token: string, method: string, route: string, data?: unknown) {
    return withRetry(() =>
      req.fetch(`${API}${route}`, {
        method,
        data,
        headers: { "Idempotency-Key": randomUUID(), Authorization: `Bearer ${token}` },
      }),
    )
  }

  test.beforeAll(async ({ browser, request }) => {
    if (!creds.email || !creds.password) throw new Error("Missing E2E FM credentials")
    // API-only fixture: no UI, so it works while the dev server is still warming up.
    let loginRes = await request.post(`${API}/auth/login`, {
      data: { email: creds.email, password: creds.password },
    })
    for (let attempt = 0; attempt < 3 && loginRes.status() === 429; attempt++) {
      await new Promise((r) => setTimeout(r, 8000))
      loginRes = await request.post(`${API}/auth/login`, {
        data: { email: creds.email, password: creds.password },
      })
    }
    expect(loginRes.status(), await loginRes.text()).toBe(200)
    fixtureToken = ((await loginRes.json()) as { token: string }).token
    const token = fixtureToken
    execSync(
      `docker exec transit-trucks-postgres-1 psql -U transitops -d transitops -c "UPDATE trips SET status = 'Cancelled' WHERE vehicle_reg LIKE 'VH-E2E-%' AND status = 'Dispatched'; DELETE FROM vehicle_positions WHERE vehicle_reg LIKE 'VH-E2E-%'; DELETE FROM route_geometries WHERE origin_id LIKE 'E2E Hub%';"`,
      { stdio: "pipe" },
    )

    const page = await browser.newPage()
    try {
      for (const hub of [hubA, hubB]) {
        await apiToken(request, token, "POST", "/destinations/", { name: hub, region: "Test" })
      }
      // Cache a road-style route so dispatch is a cache hit (no OSRM in CI).
      execSync(
        `docker exec transit-trucks-postgres-1 psql -U transitops -d transitops -c "INSERT INTO route_geometries (origin_id, destination_id, polyline6, distance_m, duration_s, provider) VALUES ('${hubA}', '${hubB}', '${FIXTURE_POLYLINE}', 1480000.5, 49200.0, 'osrm') ON CONFLICT (origin_id, destination_id) DO NOTHING;"`,
        { stdio: "pipe" },
      )
      for (let i = 0; i < 2; i++) {
        await apiToken(request, token, "POST", "/vehicles", {
          registration_number: regs[i],
          model: "E2E",
          type: "Van",
          max_load_capacity: 1000,
          odometer: 0,
          acquisition_cost: 1000,
        })
        await apiToken(request, token, "POST", "/fleet/drivers", {
          license_number: lics[i],
          name: "E2E Driver",
          license_category: "Heavy",
          license_expiry_date: "2030-01-01",
          contact_number: "555",
          safety_score: 90,
        })
        const trip = await apiToken(request, token, "POST", "/trips/", {
          source_name: hubA,
          destination_name: hubB,
          vehicle_reg: regs[i],
          driver_license: lics[i],
          cargo_weight: 100,
          planned_distance: 100,
          revenue: 100,
          dispatch: true,
        })
        tripIds.push(trip.id)
        // Spread trucks along the route so markers do not cluster in tests.
        await apiToken(request, token, "POST", "/telemetry/positions", {
          vehicle_reg: regs[i],
          latitude: i === 0 ? 41.8781 : 39.9,
          longitude: i === 0 ? -87.6298 : -104.0,
          heading_deg: 250,
          speed_kmh: 60,
          recorded_at: new Date().toISOString(),
          source: "simulator",
        })
      }
      // One UI login for the whole file: tests reuse this session.
      sharedCtx = await browser.newContext()
      const lp = await sharedCtx.newPage()
      await login(lp)
      await lp.close()
    } finally {
      await page.close()
    }
  })

  test.afterAll(async () => {
    // Best-effort cleanup in one SQL statement: no HTTP, no rate-limit load.
    try {
      if (tripIds.length > 0) {
        execSync(
          `docker exec transit-trucks-postgres-1 psql -U transitops -d transitops -c "UPDATE trips SET status = 'Cancelled' WHERE id IN (${tripIds.join(",")}); DELETE FROM vehicle_positions WHERE vehicle_reg IN ('${regs[0]}','${regs[1]}'); DELETE FROM route_geometries WHERE origin_id='${hubA}';"`,
          { stdio: "pipe" },
        )
      }
    } finally {
      await sharedCtx.close()
    }
  })

  test("overview: a marker moves between snapshots 5 seconds apart", async () => {
    const page = await sharedCtx.newPage()
    await page.goto("/dashboard")
    await expect(page.locator(".truck-marker").first()).toBeVisible({ timeout: 15000 })
    const box0 = await page.locator(".truck-marker").first().boundingBox()
    expect(box0).not.toBeNull()

    await api(page, "POST", "/telemetry/positions", {
      vehicle_reg: regs[0],
      latitude: 40.5,
      longitude: -92.0,
      heading_deg: 250,
      speed_kmh: 60,
      recorded_at: new Date().toISOString(),
      source: "simulator",
    })
    await page.waitForTimeout(5000)
    const box1 = await page.locator(".truck-marker").first().boundingBox()
    expect(box1).not.toBeNull()
    expect(
      Math.abs(box1!.x - box0!.x) + Math.abs(box1!.y - box0!.y),
      "marker did not move between snapshots",
    ).toBeGreaterThan(2)
    await page.close()
  })

  test("each rendered route has more than 2 points and dropdown changes the card", async () => {
    const page = await sharedCtx.newPage()
    const live = await api(page, "GET", "/live/vehicles")
    const ours = (live.items as any[]).filter((v) => regs.includes(v.vehicle_reg))
    expect(ours.length).toBe(2)
    for (const v of ours) {
      expect(v.route_polyline, `${v.vehicle_reg} missing route polyline`).toBeTruthy()
      expect(decodeCount(v.route_polyline)).toBeGreaterThan(2)
    }

    await page.goto("/dashboard")
    await expect(page.locator(".truck-marker").first()).toBeVisible({ timeout: 15000 })
    await expect(page.locator("path.route-line").first()).toBeAttached({ timeout: 10000 })

    await page.locator('button[aria-haspopup="listbox"]').click()
    await page.getByRole("option", { name: new RegExp(regs[1]) }).click()
    await expect(page.getByLabel(/Trip \d+/)).toContainText(regs[1])
    await page.close()
  })

  for (const vp of [
    { name: "1440px", width: 1440, height: 900 },
    { name: "390px", width: 390, height: 844 },
  ]) {
    test(`overview screenshot and a11y at ${vp.name}`, async () => {
      const page = await sharedCtx.newPage()
      await page.setViewportSize({ width: vp.width, height: vp.height })
      await page.goto("/dashboard")
      await expect(page.locator(".truck-marker").first()).toBeVisible({ timeout: 15000 })
      await page.screenshot({ path: path.join(LOG_DIR, `overview-${vp.name}.png`), fullPage: true })
      const results = await new AxeBuilder({ page }).analyze()
      const serious = results.violations.filter((v) => v.impact === "serious" || v.impact === "critical")
      expect(serious, JSON.stringify(serious.map((v) => v.id))).toEqual([])
      await page.close()
    })
  }
})
