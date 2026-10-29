# Frontend Design V2 Migration Report

## 1. Routes Built & Tested
- `/`: Public landing page showcasing TransitOps hero, live cover dashboard preview, capability cards, and footer.
- `/login`: Full split-screen login page matching `code.html` layout with static Workspace selector, Company selector, 5-role segmented button group (`Admin`, `Fleet Manager`, `Driver`, `Safety Officer`, `Financial Analyst`), email, and password. Fully localized imagery from `frontend/public/images/`.
- `/dashboard`: Unified operations dashboard with role-aware metrics, quick actions, fleet distribution overview, and real-time alerts.
- `/trips`: Freight trips ledger with complete dispatch capabilities, table layout, status badges, and realtime updates via WebSocket / SSE.
- `/live-map`: Real-time telematics and live tracking map rendered via Leaflet (`FleetMapInner`), converted from city buses to freight trucks, depots, and cargo routes.
- `/vehicles`: Vehicle registry and asset status monitoring.
- `/maintenance`: Scheduled maintenance, defect tracking, and shop work orders.
- `/finance`: Operational expense tracking, fuel logging, and cost breakdown.
- `/safety`: Safety violation tracking and compliance logs.
- `/analytics`: Aggregated KPI reports, fuel efficiency trends, and dispatch utilization.
- `/audit`: Security audit trails and tamper-evident event logs.

## 2. Screenshot Paths
All screenshots captured at 1440x900 desktop viewport (and 390x844 mobile viewport) under `frontend/docs/frontend-design/screenshots/`:
- `landing-desktop.png`: Desktop landing page
- `landing-mobile.png`: Mobile landing page
- `login-desktop.png`: Desktop login page
- `login-mobile.png`: Mobile login page
- `role-admin.png`: Administrator view (`/dashboard`)
- `role-fleet.png`: Fleet Manager view (`/dashboard`)
- `role-driver.png`: Driver view (`/trips`)
- `role-safety.png`: Safety Officer view (`/dashboard`)
- `role-finance.png`: Financial Analyst view (`/dashboard`)

## 3. Decisions (D1 - D10)
- **D1 Brand**: TransitOps used consistently across all components, layout headers, footers, meta titles, and aria attributes.
- **D2 Routes**: Preserved existing route hierarchy (`/`, `/login`, `/dashboard`, `/trips`, `/live-map`, `/vehicles`, `/maintenance`, `/finance`, `/safety`, `/analytics`, `/audit`).
- **D3 Data Wiring**: Kept strictly un-wired from production backend for Phase 5 design step. Components use sanitized truck & freight models; real API adapters remain ready for Step 2.
- **D4 Workspaces & Companies**: Static options defined in `src/config/workspaces.ts`:
  - Workspaces: Default Workspace (`default`), US West Operations (`us-west`), US East Operations (`us-east`), Central Logistics (`central`).
  - Companies: TransitOps Fleet (`transitops-fleet`), Pacific Freight Lines (`pacific-freight`), Summit Express Logistics (`summit-express`).
- **D5 Sample Data**: Stripped all fake metrics (e.g. `1.2M`, `98.6%`, `1,248`, `$48.2k`), driver names (`Kenji`, `Ryotaro`), and testimonials. Converted bus/passenger domain to commercial freight trucks.
- **D6 Login Capabilities**: Sign up, social auth, and "Forgot password" links removed. Displayed explicit notice: *"Accounts are created by your administrator."*
- **D7 External Dependencies**: Removed all remote CDNs (Tailwind CDN, Google Fonts, lh3.googleusercontent.com). Loaded Inter & Newsreader via `next/font/google` and local freight assets under `frontend/public/images/`.
- **D8 Role Separation**: AppShell navigation dynamically filters routes based on verified user role (`useAuth` / `getMeRequest`). Segmented role button enables explicit `expected_role` validation against backend.
- **D9 Clean Scrutiny**: Verified 0 forbidden strings across the codebase (`Verdant`, `OmniFleet`, `Kenji`, `Ryotaro`, `Emi S`, `Tokyo`, `Kyoto`, `Maya Okafor`, `98.6%`, `1.2M`, `1,248`, `$48.2k`, `googleapis`, `lh3.googleusercontent`, `cdn.tailwindcss`, `localStorage`, `sessionStorage`).
- **D10 Zero Hex Colors**: Replaced all raw hex literals in components with OKLCH design system tokens and Tailwind semantic utilities.

## 4. Color Palette & Typography Tokens
- `--primary`: `oklch(0.3 0.05 160)` (deep forest green)
- `--primary-foreground`: `oklch(0.98 0.01 160)`
- `--background`: `oklch(0.985 0.005 100)`
- `--foreground`: `oklch(0.2 0.02 160)`
- `--card`: `oklch(1 0 0)`
- `--card-foreground`: `oklch(0.2 0.02 160)`
- `--muted`: `oklch(0.95 0.01 160)`
- `--muted-foreground`: `oklch(0.45 0.02 160)`
- `--signal`: `oklch(0.68 0.17 48)`
- `--signal-foreground`: `oklch(0.99 0 0)`
- `--destructive`: `oklch(0.55 0.22 25)`
- `--sage`: `oklch(0.94 0.02 155)`
- `--sand`: `oklch(0.94 0.02 85)`
- `--mist`: `oklch(0.95 0.01 210)`
- `--chart-1` through `--chart-5`: Semantic telemetry charting hues.
- Typography: Inter (`font-sans`) for operational telemetry, UI labels, data grids; Newsreader (`font-serif`) for brand headings and cover titles.

## 5. Remaining Sample Data in Step 1
- `src/lib/data/transit-data.ts`: Mock routes, stops, and truck coordinates for the live simulation preview component.
- `src/components/fleet/card-variants.tsx`: Representative status badge states for truck cards (Idle, In Transit, Maintenance Required).
- Note: All operational tables (`/trips`) and auth logic continue to use real backend data.

## 6. Backend Changes Summary
- `api/openapi.yaml`:
  - Extended `PostApiV1AuthLoginJSONBody` with optional `workspace`, `company`, and `expected_role` (enum: `Admin`, `Fleet Manager`, `Driver`, `Safety Officer`, `Financial Analyst`).
  - Added `403` error response for role mismatch.
- `backend-go/internal/api/types.gen.go`: Regenerated OpenAPI types for Go.
- `backend-go/internal/app/app.go`:
  - Added payload decoding with 64-char bounds checking for `workspace` and `company`.
  - Added `expected_role` validation (returns `422` on unknown role).
  - Enforced role match check against authenticated user's actual role in database (returns `403 ROLE_MISMATCH` if mismatched).
- `backend-go/internal/app/auth_expected_role_test.go`: Added test cases for unknown payload fields, invalid role enums, role mismatch (403), matching role (200), and backwards compatibility without expected role (200).
- `backend-go/cmd/seed/main.go`:
  - Replaced hardcoded password fallback with `SEED_PASSWORD` / `INITIAL_ADMIN_PASSWORD` / default `TransitOps2026!`.
  - Added `password_hash = EXCLUDED.password_hash`, `role`, `is_active`, and `driver_license` to `ON CONFLICT (email) DO UPDATE SET` list so re-seeding always updates user hashes.
- `backend-go/cmd/seed/seed_test.go`: Added re-seed test asserting password hash updates on conflict.

## 7. Verification Summary
- `backend_test.log`: PASS (`go test -count=1 ./...`)
- `npm_test.log`: PASS (18/18 unit tests passing)
- `npm_typecheck.log`: PASS (`tsc --noEmit`)
- `npm_lint.log`: PASS (0 errors)
- `playwright.log`: PASS (6/6 tests passing, including M31 realtime journey and design_v2 a11y & visual tests)
- Axe Accessibility Audit: 0 serious or critical violations across Desktop & Mobile views.
