# TransitOps

**Smart Transport Operations Platform** — Odoo Hackathon 2026

TransitOps replaces spreadsheet-driven logistics with a centralized system for fleet, drivers, dispatch, live map tracking, maintenance, fuel/expenses, and analytics. Business rules (capacity checks, status cascades, license validity, RBAC) are enforced in the API, not only in the UI.

**Repository:** [Reaper-ai/TransitOps_Odoo2026](https://github.com/Reaper-ai/TransitOps_Odoo2026)

### Team

Built by **Team Reaper-ai** for the Odoo Hackathon 2026.

| Member | Focus |
|--------|--------|
| **Gaurav** | Backend engineering & data infrastructure |
| **Ashmeet** | Integrations & endpoint connections |
| **Agrim** | Frontend UI, client state, and product UX |

---

## Features

### Authentication & access
- Public **landing page** (`/`) for product overview
- Email/password **login** with JWT sessions
- Public **Create account** (`/register`) with role selection
- Role-based navigation and write permissions (Admin bypass for testing)

| Role | Highlights |
|------|------------|
| **Admin** | Full access |
| **Fleet Manager** | Fleet, drivers, trips, live map, maintenance, fuel, analytics |
| **Driver** | Dashboard + trips + live map; limited fleet/driver read |
| **Safety Officer** | Driver compliance focus + live map |
| **Financial Analyst** | Fuel/expenses + analytics + live map |

### Operations
- **Fleet** — vehicle registry (capacity, odometer, cost, status: Available / On Trip / In Shop / Retired)
- **Drivers** — licenses, expiry, safety scores, status
- **Trips** — create/dispatch, complete (odometer), cancel; filters by source, destination, vehicle, driver; smart vehicle/driver suggestions
- **Live Map** — real-world road routes with trucks moving source → destination after dispatch
- **Maintenance** — open/close work orders (vehicle → In Shop / Available)
- **Fuel & Expenses** — Fuel / Toll / Maintenance-Surge / Other; liters only for Fuel; CSV / PDF export

### Insights
- **Dashboard** — KPIs (active/available/in-shop vehicles, active trips, utilization, fuel efficiency, revenue, cost) + vehicle ROI chart + live map embed
- **Analytics** — five full-width charts: ROI, fuel efficiency per truck, driver safety, odometer, route capacity map (edge width ∝ max cargo)
- **Realtime** — WebSocket updates when trips dispatch/complete or maintenance/expense events fire

### Data
- Large seed set in `dummy_data.json` (users, hubs, vehicles, drivers, trips, maintenance, expenses)

---

## Tech stack & setup

### Stack

| Layer | Technology |
|-------|------------|
| Backend | FastAPI, Uvicorn, SQLite (Postgres optional), uv, Pydantic, PyJWT, bcrypt |
| Frontend | Next.js 15, React 19, TypeScript, Tailwind, Tremor UI, Recharts, Leaflet |
| Auth | OAuth2 password flow + Bearer JWT + RBAC middleware |

### Prerequisites
- Git
- Python **3.11+**
- **[uv](https://docs.astral.sh/uv/)**
- Node.js **18+** and npm

### 1. Clone

```bash
git clone https://github.com/Reaper-ai/TransitOps_Odoo2026.git
cd TransitOps_Odoo2026
```

### 2. Backend

```bash
cd backend
cp .env.example .env
```

Set **absolute paths** in `.env`:

```env
DB_PATH=/absolute/path/to/TransitOps_Odoo2026/backend/database/transitops.db
JSON_PATH=/absolute/path/to/TransitOps_Odoo2026/backend/app/db/init/dummy_data.json
SECRET_KEY=SUPER_SECRET_COMPROMISED_HACKATHON_KEY_CHANGE_THIS
```

Windows example:

```env
DB_PATH=E:/Hackathons/odoo/TransitOps_Odoo2026/backend/database/transitops.db
JSON_PATH=E:/Hackathons/odoo/TransitOps_Odoo2026/backend/app/db/init/dummy_data.json
SECRET_KEY=SUPER_SECRET_COMPROMISED_HACKATHON_KEY_CHANGE_THIS
```

```bash
uv sync
uv run python -m app.db.init.init_db
uv run python -m app.db.init.seed_db
uv run uvicorn app.main:app --reload
```

- API: http://127.0.0.1:8000
- Docs: http://127.0.0.1:8000/docs

Fresh reseed: stop the server, delete `backend/database/transitops.db`, then re-run `init_db` + `seed_db`.

### 3. Frontend (second terminal)

```bash
cd frontend
npm install
npm run dev
```

App: http://localhost:3000

- Public landing: http://localhost:3000
- Login: http://localhost:3000/login
- API base defaults to `http://localhost:8000/api` (`frontend/src/data/data.ts`). Optional: `NEXT_PUBLIC_API_URL`.

### Demo logins

Password for all: **`password123`**

| Email | Role |
|-------|------|
| `manager@transitops.com` | Fleet Manager |
| `safety@transitops.com` | Safety Officer |
| `finance@transitops.com` | Financial Analyst |
| `admin@transitops.com` | Admin |

Or use **Create account** on `/register`.

### Daily run

```bash
# Terminal 1
cd backend && uv run uvicorn app.main:app --reload

# Terminal 2
cd frontend && npm run dev
```

---

## Architecture

```text
Browser (Next.js)
    │  JWT Bearer
    ▼
FastAPI routers  (/api/auth, /fleet, /trips, /maintenance, /finance, /dashboard, …)
    │
    ▼
Pydantic schemas + RoleChecker (Admin bypass)
    │
    ▼
Abstract BaseRepository  →  SQLiteRepository (or Postgres)  →  database
```

- **Repository pattern** — routers never embed SQL; storage can be swapped by changing `app/db/session.py`.
- **Business rules** live in modules (trip capacity/license checks, maintenance status cascades, expense type rules).
- **CORS** enabled for local frontend development.
- **Seed pipeline** — `init_db` builds schema; `seed_db` loads `dummy_data.json`.

### Main API groups

| Prefix | Responsibility |
|--------|----------------|
| `/api/auth` | Login, register |
| `/api/fleet` | Vehicles & drivers |
| `/api/destinations` | Hubs |
| `/api/trips` | Dispatch, complete, cancel, suggest |
| `/api/maintenance` | Open / close logs |
| `/api/finance` | Expenses by trip |
| `/api/dashboard` | KPIs + per-vehicle analytics |
| `/ws/fleet` | Realtime fleet events |

---

## Folder structure

```text
TransitOps_Odoo2026/
├── README.md                 # this file
├── SETUP.md                  # detailed local setup notes
├── backend/
│   ├── .env.example
│   ├── pyproject.toml
│   ├── database/transitops.db
│   └── app/
│       ├── main.py           # FastAPI app + routers + websocket
│       ├── core/             # JWT, bcrypt, RoleChecker, realtime
│       ├── db/
│       │   ├── base.py       # repository interface
│       │   ├── sqlite_repo.py
│       │   ├── postgres_repo.py
│       │   ├── session.py    # db singleton
│       │   └── init/         # init_db, seed_db, dummy_data.json
│       └── modules/          # auth, fleet, trips, maintenance, finance, dashboard, …
└── frontend/
    ├── package.json
    ├── README.md             # frontend-focused guide
    └── src/
        ├── app/              # App Router pages (landing, login, dashboard, live-map, …)
        ├── components/       # UI + AppShell + navigation + map
        ├── lib/              # api.ts, ProfileContext, rbacConfig, geo
        ├── data/data.ts      # BACKEND_URL
        └── types/
```

---

## Future

Possible next steps beyond the hackathon demo:

- Explicit **Draft → Dispatch** workflow (create currently auto-dispatches)
- **Email reminders** for license expiry
- **Vehicle document locker** (insurance, registration)
- Richer **compliance banners** and pending-trip KPI cards
- Production hardening (CORS allowlist, secrets management, rate limits)

---

## License / UI kit

Frontend UI is based on the [Tremor](https://tremor.so) Planner template components. See `frontend/README.md` for frontend-only notes.
