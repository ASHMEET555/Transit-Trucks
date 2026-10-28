import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  env: {
    NEXT_PUBLIC_E2E: process.env.NEXT_PUBLIC_E2E || "0",
  },
}

export default nextConfig
