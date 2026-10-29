import { defineConfig, devices } from "@playwright/test"

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 45000,
  expect: {
    timeout: 5000,
  },
  use: {
    baseURL: process.env.FRONTEND_URL || "http://localhost:3000",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: [
    {
      command: "cd ..\\backend-go && go run ./cmd/server",
      url: "http://localhost:8000/healthz",
      reuseExistingServer: true,
      timeout: 45000,
      env: {
        DATABASE_URL: process.env.DATABASE_URL || "postgres://transitops:transitops@localhost:5432/transitops?sslmode=disable",
        PORT: "8000",
        ENV: "local",
        SESSION_SECRET: "my-long-random-session-secret-key-transit-trucks-2026!",
        ALLOWED_ORIGINS: "http://localhost:3000,http://127.0.0.1:3000",
        COOKIE_DOMAIN: "localhost",
        TRUSTED_PROXY_CIDRS: "127.0.0.1/32,::1/128",
      },
    },
    {
      command: "npm run dev",
      url: "http://localhost:3000",
      reuseExistingServer: true,
      timeout: 45000,
      env: {
        NEXT_PUBLIC_E2E: "1",
        PORT: "3000",
      },
    },
  ],
})
