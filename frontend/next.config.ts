import type { NextConfig } from "next"

if (process.env.NODE_ENV === "production" && process.env.NEXT_PUBLIC_DATA_MODE === "sample") {
  throw new Error("Build gate: NEXT_PUBLIC_DATA_MODE cannot be set to 'sample' in production build.")
}

const nextConfig: NextConfig = {
  env: {
    NEXT_PUBLIC_E2E: process.env.NEXT_PUBLIC_E2E || "0",
  },
}

export default nextConfig
