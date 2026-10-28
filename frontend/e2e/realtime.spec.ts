import { test, expect, Page } from "@playwright/test"
import { execSync } from "node:child_process"
import { randomUUID } from "node:crypto"

test.describe("M31: Multi-Browser Realtime Journey", () => {
  const fmEmail = process.env.E2E_FM_EMAIL
  const fmPassword = process.env.E2E_FM_PASSWORD
  const driverEmail = process.env.E2E_DRIVER_EMAIL
  const driverPassword = process.env.E2E_DRIVER_PASSWORD
  const faEmail = process.env.E2E_FA_EMAIL
  const faPassword = process.env.E2E_FA_PASSWORD

  test.beforeAll(async () => {
    if (!fmEmail || !fmPassword || !driverEmail || !driverPassword || !faEmail || !faPassword) {
      throw new Error(
        "Missing required E2E credentials in environment variables (E2E_FM_EMAIL, E2E_FM_PASSWORD, E2E_DRIVER_EMAIL, E2E_DRIVER_PASSWORD, E2E_FA_EMAIL, E2E_FA_PASSWORD)"
      )
    }

    // Ensure vehicle and driver are available for dispatch before test begins
    try {
      execSync(
        `docker exec transit-trucks-postgres-1 psql -U transitops -d transitops -c "UPDATE trips SET status = 'Cancelled' WHERE (vehicle_reg = 'VH-1005' OR driver_license = 'DL-1001') AND status = 'Dispatched'; UPDATE vehicles SET status = 'Available' WHERE registration_number = 'VH-1005'; UPDATE drivers SET status = 'Available' WHERE license_number = 'DL-1001';"`
      )
    } catch (e) {
      // In case docker exec is not available locally, proceed with existing state
      console.warn("DB setup warning:", e)
    }
  })

  async function login(page: Page, email: string, pass: string) {
    await page.goto("/login")
    await page.fill('input[type="email"]', email)
    await page.fill('input[type="password"]', pass)
    await page.click('button[type="submit"]')
    await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 15000 })
  }

  test("dispatches trip from FM, receives realtime update in Driver without reload, preserves FA PII boundary", async ({
    browser,
  }) => {
    // 1. Create three independent browser contexts
    const contextA = await browser.newContext()
    const contextB = await browser.newContext()
    const contextC = await browser.newContext()

    const pageA = await contextA.newPage()
    const pageB = await contextB.newPage()
    const pageC = await contextC.newPage()

    try {
      // 2. Log in each user
      await login(pageA, fmEmail!, fmPassword!)
      await login(pageB, driverEmail!, driverPassword!)
      await login(pageC, faEmail!, faPassword!)

      // 3. Open pages that use useFleetEvents
      await pageA.goto("/trips")
      await pageB.goto("/trips")
      await pageC.goto("/dashboard")

      // Wait for Driver trip list to mount
      await pageB.waitForSelector("table", { timeout: 10000 })

      // 4. Attach navigation listener on Page B to ensure no page reload occurs
      let pageBNavigations = 0
      pageB.on("framenavigated", (frame) => {
        if (frame === pageB.mainFrame()) {
          pageBNavigations++
        }
      })
      const initialNavCount = pageBNavigations

      // 5. Context A dispatches a trip to the Driver's vehicle and driver
      const dispatchRes = await pageA.request.post("http://localhost:8000/api/v1/trips", {
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
        headers: {
          "Idempotency-Key": randomUUID(),
          "X-Forwarded-For": "10.0.1.50",
        },
      })
      if (dispatchRes.status() !== 201) {
        console.error("Dispatch failed:", await dispatchRes.text())
      }
      expect(dispatchRes.status()).toBe(201)
      const createdTrip = await dispatchRes.json()
      const tripId = createdTrip.id

      // 6. Assert: Context B shows the new trip within 5 s, without a page reload
      const tripLocator = pageB.locator(`text=#${tripId}`)
      await expect(tripLocator).toBeVisible({ timeout: 5000 })
      expect(pageBNavigations).toBe(initialNavCount)

      // 7. Assert: Context C receives no event whose payload references the driver's license
      const logsC = await pageC.evaluate(() => (window as any).__fleetEventLog || [])
      for (const ev of logsC) {
        const payloadStr = JSON.stringify(ev)
        expect(payloadStr).not.toContain("DL-1001")
      }

      // 8. Assert: Context C does receive trip.completed or expense.created when Context A completes the trip
      const startOdo = Number(createdTrip.start_odometer) || 95919
      const completeRes = await pageA.request.post(`http://localhost:8000/api/v1/trips/${tripId}/complete`, {
        data: {
          final_odometer: startOdo + 140,
          fuel_liters: 40,
          fuel_cost: 120,
        },
        headers: {
          "Idempotency-Key": randomUUID(),
          "X-Forwarded-For": "10.0.1.51",
        },
      })
      if (completeRes.status() !== 200) {
        console.error("Complete failed:", completeRes.status(), await completeRes.text())
      }
      expect(completeRes.status()).toBe(200)

      await expect
        .poll(
          async () => {
            const currentLogs = await pageC.evaluate(() => (window as any).__fleetEventLog || [])
            return currentLogs.some(
              (e: any) => e.type === "trip.completed" || e.type === "expense.created"
            )
          },
          { timeout: 5000 }
        )
        .toBe(true)
    } finally {
      await contextA.close()
      await contextB.close()
      await contextC.close()
    }
  })
})
