# TransitOps Frontend

Next.js client for the **TransitOps** Smart Transport Operations Platform (Odoo Hackathon 2026).

This app talks to the FastAPI backend over JWT (`/api/*`). Full project docs and end-to-end setup live in the **[root README](../README.md)**.

---

## What this frontend covers

- **Login / Register** — JWT session; create account with role selection
- **App shell** — sidebar + RBAC-aware navigation
- **Dashboard** — KPIs and vehicle ROI chart
- **Fleet / Drivers / Trips / Maintenance / Fuel & Expenses** — CRUD and workflows wired to the API
- **Analytics** — ROI, fuel efficiency, driver safety, odometer, route capacity map (independent filters per chart)
- **Settings** — profile / preferences surface

---

## Stack

| Piece | Version / notes |
|-------|-----------------|
| Next.js | 15 (App Router) |
| React | 19 |
| TypeScript | yes |
| Styling | Tailwind CSS 3 |
| UI | Tremor-style components (Button, Input, Select, Table, Sidebar, BarChart, …) |
| Charts | Recharts via Tremor `BarChart` + custom SVG route map |
| Icons | Lucide / Remixicon |
| Themes | `next-themes` (light/dark) |

API base URL: `src/data/data.ts` → `http://localhost:8000/api` (or `NEXT_PUBLIC_API_URL`).

Auth helper: `src/lib/api.ts` (Bearer token, login/register helpers).  
Roles: `src/lib/rbacConfig.ts`.

---

## Prerequisites

- Node.js **18+** and **npm**
- Backend running on **http://127.0.0.1:8000** (see root README)

---

## Setup

From the repo root:

```bash
cd frontend
npm install
npm run dev
```

Open **http://localhost:3000**.

You will be redirected to `/login` until authenticated.

### Demo accounts

Password: **`password123`**

| Email | Role |
|-------|------|
| `manager@transitops.com` | Fleet Manager |
| `safety@transitops.com` | Safety Officer |
| `finance@transitops.com` | Financial Analyst |
| `admin@transitops.com` | Admin |

Or use **Create account** → `/register`.

---

## Scripts

```bash
npm run dev      # development server (port 3000)
npm run build    # production build
npm run start    # serve production build
npm run lint     # ESLint
```

---

## Folder map

```text
frontend/
├── public/
├── src/
│   ├── app/                    # routes
│   │   ├── login/
│   │   ├── register/
│   │   ├── dashboard/
│   │   ├── fleet/
│   │   ├── drivers/
│   │   ├── trips/
│   │   ├── maintenance/
│   │   ├── fuel-and-expenses/
│   │   ├── analytics/
│   │   └── settings/
│   ├── components/
│   │   ├── AppShell.tsx        # auth gate + layout
│   │   ├── ui/navigation/      # sidebar, breadcrumbs, profile
│   │   └── …                   # Tremor primitives + charts
│   ├── data/data.ts            # BACKEND_URL
│   ├── lib/
│   │   ├── api.ts              # fetch + JWT
│   │   ├── ProfileContext.tsx
│   │   └── rbacConfig.ts
│   └── types/
├── package.json
└── README.md
```

---

## Notes

- Start the **backend first**, then this app.
- If pages show unauthorized / empty data, check the API is up and `BACKEND_URL` matches.
- UI components originated from the [Tremor Planner](https://tremor.so) template; product branding and pages are TransitOps-specific.
- For architecture, seed data, and backend commands, see the [main project README](../README.md).
