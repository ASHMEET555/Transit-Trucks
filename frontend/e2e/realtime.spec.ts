import { test, expect, Page } from "@playwright/test"
import { execSync } from "node:child_process"
import { randomUUID } from "node:crypto"

const API = "http://localhost:8000/api/v1"

test.describe("M31: Multi-Browser Realtime Journey", () => {
  const creds = {
    fm: { email: process.env.E2E_FM_EMAIL, password: process.env.E2E_FM_PASSWORD },
    driver: { email: process.env.E2E_DRIVER_EMAIL, password: process.env.E2E_DRIVER_PASSWORD },
    fa: { email: process.env.E2E_FA_EMAIL, password: process.env.E2E_FA_PASSWORD },
  }

  test.beforeAll(async () => {
    for (const [name, c] of Object.entries(creds)) {
      if (!c.email || !c.password) throw new Error(`Missing E2E credentials for ${name}`)
    }
    // Reset the fixture. If this fails, stop: the test must not run against unknown state.
    execSync(
      `docker exec transit-trucks-postgres-1 psql -U transitops -d transitops -c "UPDATE trips SET status = 'Cancelled' WHERE (vehicle_reg = 'VH-1005' OR driver_license = 'DL-1001') AND status = 'Dispatched'; UPDATE vehicles SET status = 'Available' WHERE registration_number = 'VH-1005'; UPDATE drivers SET status = 'Available', is_active = true WHERE license_number = 'DL-1001';"`,
      { stdio: "pipe" }
    )
  })

  async function login(page: Page, email: string, pass: string) {
    await page.goto("/login")
    if (email.includes("driver")) {
      await page.click('button:has-text("Driver")')
    } else if (email.includes("finance")) {
      await page.click('button:has-text("Financial Analyst")')
    } else if (email.includes("fleet")) {
      await page.click('button:has-text("Fleet Manager")')
    }
    await page.fill('input[type="email"]', email)
    await page.fill('input[type="password"]', pass)
    await page.click('button[type="submit"]')
    await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 15000 })
  }

  test("FM dispatch reaches Driver without reload; FA gets only allowed events", async ({ browser }) => {
    const ctxA = await browser.newContext()
    const ctxB = await browser.newContext()
    const ctxC = await browser.newContext()
    try {
      const pageA = await ctxA.newPage()
      const pageB = await ctxB.newPage()
      const pageC = await ctxC.newPage()

      // Count every main-frame navigation on the Driver page from the start.
      let pageBNavigations = 0
      pageB.on("framenavigated", (frame) => {
        if (frame === pageB.mainFrame()) pageBNavigations++
      })

      await login(pageA, creds.fm.email!, creds.fm.password!)
      await login(pageB, creds.driver.email!, creds.driver.password!)
      await login(pageC, creds.fa.email!, creds.fa.password!)
      await pageA.goto("/trips")
      await pageB.goto("/trips")
      await pageC.goto("/dashboard")
      await pageB.waitForSelector("table", { timeout: 10000 })

      // Baseline: taken after the Driver page is fully loaded, immediately before dispatch.
      const baseline = pageBNavigations

      const dispatchRes = await pageA.request.post(`${API}/trips`, {
        data: {
          source_name: "New York Central Hub",
          destination_name: "Chicago Logistics Hub",
          vehicle_reg: "VH-1005",
          driver_license: "DL-1001",
          cargo_weight: 850,
          planned_distance: 140,
          revenue: 1200,
          dispatch: true,
        },
        headers: { "Idempotency-Key": randomUUID(), "X-Forwarded-For": "10.0.1.50" },
      })
      expect(dispatchRes.status(), await dispatchRes.text()).toBe(201)
      const trip = await dispatchRes.json()
      const tripId = trip.id

      // Driver sees the trip without a reload.
      await expect(pageB.locator(`text=#${tripId}`)).toBeVisible({ timeout: 5000 })
      expect(pageBNavigations, "Driver page navigated during realtime update").toBe(baseline)

      const startOdo = Number(trip.start_odometer) || 95919
      const completeRes = await pageA.request.post(`${API}/trips/${tripId}/complete`, {
        data: { final_odometer: startOdo + 140, fuel_liters: 40, fuel_cost: 120 },
        headers: { "Idempotency-Key": randomUUID(), "X-Forwarded-For": "10.0.1.51" },
      })
      expect(completeRes.status(), await completeRes.text()).toBe(200)

      const readLog = () =>
        pageC.evaluate(() => (window as any).__fleetEventLog ?? []) as Promise<any[]>

      // Positive proof the FA hook is active: an allowed event for this trip arrives.
      await expect
        .poll(async () => (await readLog()).some((e) => e.type === "trip.completed"), { timeout: 5000 })
        .toBe(true)

      const logC = await readLog()
      expect(logC.length, "FA event log is empty; hook not active").toBeGreaterThan(0)

      // Role boundary: FA must not receive dispatch events or any driver PII.
      expect(logC.some((e) => e.type === "trip.dispatched")).toBe(false)
      for (const ev of logC) {
        const s = JSON.stringify(ev)
        expect(s).not.toContain("DL-1001")
        expect(s).not.toMatch(/"(driver_license|license_number|driver_name|email|phone)"/)
      }
    } finally {
      await ctxA.close()
      await ctxB.close()
      await ctxC.close()
    }
  })
})
