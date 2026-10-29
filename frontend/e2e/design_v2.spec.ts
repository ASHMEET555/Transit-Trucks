import { test, expect } from "@playwright/test"
import AxeBuilder from "@axe-core/playwright"
import fs from "fs"
import path from "path"

const SCREENSHOT_DIR = path.join(__dirname, "../docs/frontend-design/screenshots")

test.describe("Design V2 - Visual, Accessibility & Keyboard Navigation", () => {
  test.beforeAll(() => {
    fs.mkdirSync(SCREENSHOT_DIR, { recursive: true })
  })

  const viewports = [
    { name: "desktop", width: 1440, height: 900 },
    { name: "mobile", width: 390, height: 844 },
  ]

  for (const vp of viewports) {
    test(`Landing Page (/): ${vp.name} visual & a11y`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height })
      await page.goto("/")
      await page.waitForSelector("#cards", { timeout: 15000 })

      // Screenshot
      const shotPath = path.join(SCREENSHOT_DIR, `landing-${vp.name}.png`)
      await page.screenshot({ path: shotPath, fullPage: true })

      // Axe a11y audit: zero serious or critical violations
      const results = await new AxeBuilder({ page }).analyze()
      const seriousOrCritical = results.violations.filter(
        (v) => v.impact === "serious" || v.impact === "critical"
      )
      expect(seriousOrCritical).toEqual([])
    })

    test(`Login Page (/login): ${vp.name} visual & a11y`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height })
      await page.goto("/login")
      await page.waitForSelector("#fleet-email", { timeout: 15000 })

      // Screenshot
      const shotPath = path.join(SCREENSHOT_DIR, `login-${vp.name}.png`)
      await page.screenshot({ path: shotPath, fullPage: true })

      // Axe a11y audit
      const results = await new AxeBuilder({ page }).analyze()
      const seriousOrCritical = results.violations.filter(
        (v) => v.impact === "serious" || v.impact === "critical"
      )
      expect(seriousOrCritical).toEqual([])
    })
  }

  test("Keyboard-only login flow works", async ({ page }) => {
    await page.goto("/login")
    await page.waitForSelector("#fleet-email", { timeout: 15000 })

    // Focus first input (Workspace select)
    await page.focus("#workspace-select")
    await page.keyboard.press("Tab") // Company select
    await page.keyboard.press("Tab") // First role button (Admin)
    await page.keyboard.press("Tab") // Fleet Manager button
    await page.keyboard.press("Enter") // Select Fleet Manager role

    // Tab down to email
    await page.focus("#fleet-email")
    await page.keyboard.type("fleet@transitops.com")

    await page.keyboard.press("Tab") // Password input
    await page.keyboard.type("truck@555")

    await page.keyboard.press("Tab") // Submit button
    await page.keyboard.press("Enter") // Press submit

    // Confirm redirected to app
    await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 15000 })
    expect(page.url()).toContain("/dashboard")
  })
})
