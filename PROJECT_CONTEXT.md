# TransitOps — Full Project Context

> Single source of truth for the Odoo Hackathon 2026 project.  
> Last updated: 2026-07-12  
> Stack: **FastAPI (backend)** + **Next.js / Tremor Planner (frontend)** + **SQLite**

---

## 1. What We Are Building

**TransitOps** is an end-to-end **Smart Transport Operations Platform** for an **8-hour hackathon**.

### Problem
Logistics teams still run on spreadsheets and manual logbooks. That causes:

- Scheduling conflicts (same vehicle/driver double-booked)
- Underutilized fleet
- Missed maintenance
- Expired driver licenses going unnoticed
- Inaccurate expense / fuel tracking
- No operational visibility (KPIs, ROI, utilization)

### Goal
A centralized platform covering the full lifecycle:

**Vehicle registry → Driver management → Dispatch → Maintenance → Fuel/Expenses → Analytics**

while **enforcing business rules** in the backend (not only in the UI).

### Hackathon constraints
- Duration: **8 hours**
- Must ship a working end-to-end demo workflow
- Bonus features are optional polish (PDF, docs locker, email reminders, etc.)

---

## 2. Target Users (RBAC Roles)

| Role | Responsibility |
|------|----------------|
| **Fleet Manager** | Fleet assets, maintenance, vehicle lifecycle, operational efficiency |
| **Driver** | Create trips, assign vehicles/drivers, monitor active deliveries |
| **Safety Officer** | Driver compliance, license validity, safety scores |
| **Financial Analyst** | Expenses, fuel, maintenance costs, profitability |
| **Admin** *(codebase extension)* | Master bypass for all RBAC checks (speeds up testing) |

Frontend nav / actions must hide or show based on role.

---

## 3. Functional Requirements (from PDF)

### 3.1 Authentication
- Secure login (email + password)
- JWT-based session
- RBAC on protected routes
- Only authenticated users access the app

### 3.2 Dashboard KPIs
Show at least:

- Active Vehicles
- Available Vehicles
- Vehicles in Maintenance
- Active Trips
- Pending Trips
- Drivers On Duty
- Fleet Utilization (%)

Filters: vehicle type, status, region.

### 3.3 Vehicle Registry
Fields: Registration Number *(unique)*, Model/Name, Type, Max Load Capacity, Odometer, Acquisition Cost, Status.

**Statuses:** `Available` | `On Trip` | `In Shop` | `Retired`

### 3.4 Driver Management
Fields: Name, License Number, License Category, License Expiry Date, Contact Number, Safety Score, Status.

**Statuses:** `Available` | `On Trip` | `Off Duty` | `Suspended`

### 3.5 Trip Management
Create trip with: source, destination, available vehicle, available driver, cargo weight, planned distance.

**Lifecycle:** `Draft` → `Dispatched` → `Completed` / `Cancelled`

### 3.6 Maintenance
Creating an open maintenance log → vehicle status becomes **In Shop** (removed from dispatch pool).  
Closing maintenance → restore to **Available** (unless Retired).

### 3.7 Fuel & Expense Management
- Fuel logs: liters, cost, date
- Other expenses: tolls, maintenance surge, etc.
- Compute total operational cost (Fuel + Maintenance) per vehicle

### 3.8 Reports & Analytics
- Fuel Efficiency = Distance / Fuel
- Fleet Utilization
- Operational Cost
- Vehicle ROI = `(Revenue − (Maintenance + Fuel)) / Acquisition Cost`
- CSV export required; PDF optional

---

## 4. Mandatory Business Rules (must be enforced in API)

1. Vehicle registration number is **unique**
2. **Retired** / **In Shop** vehicles never appear in dispatch selection
3. Drivers with **expired licenses** or **Suspended** cannot be assigned
4. Driver or vehicle already **On Trip** cannot be assigned again
5. `CargoWeight ≤ MaxLoadCapacity`
6. Dispatching a trip → vehicle + driver → **On Trip**
7. Completing a trip → vehicle + driver → **Available**
8. Cancelling a dispatched trip → vehicle + driver → **Available**
9. Opening maintenance → vehicle → **In Shop**
10. Closing maintenance → vehicle → **Available** (unless retired)

### Example golden workflow (demo script)
1. Register vehicle `Van-05`, capacity 500 kg, status Available  
2. Register driver Alex with valid license  
3. Create trip with cargo 450 kg  
4. System allows (450 ≤ 500)  
5. Vehicle + Driver become On Trip  
6. Complete trip with final odometer + fuel  
7. Both become Available again  
8. Create maintenance (Oil Change) → vehicle In Shop, hidden from dispatch  
9. Reports update operational cost + fuel efficiency  

---

## 5. Expected Database Entities

| Entity | Key / Notes |
|--------|-------------|
| Users | email PK, password_hash, role |
| Destinations | name PK, region |
| Vehicles | registration_number PK |
| Drivers | license_number PK |
| Trips | autoincrement id, FKs to destinations/vehicle/driver |
| Maintenance Logs | open/closed work orders |
| Expenses / Fuel Logs | tied to `trip_id`; types: Fuel, Toll, Maintenance-Surge, Other |

---

## 6. Mandatory Deliverables vs Bonus

### Mandatory
- Responsive web UI
- Auth + RBAC
- CRUD Vehicles & Drivers
- Trip management with validations
- Automatic status transitions
- Maintenance workflow
- Fuel & expense tracking
- Dashboard with KPIs
- Charts / visual analytics
- Search, filters, sorting
- Dark mode

### Bonus
- PDF export
- Email reminders for expiring licenses
- Vehicle document management (insurance/registration locker)
- (Extra from structural plan) CSV streamers, compliance banners, postgres swap demo

### Mockup / UI references
- Excalidraw: https://link.excalidraw.com/l/65VNwvy7c4X/1FHGDNgD2td
- Tremor Planner template: https://planner.tremor.so/quotes/overview

---

## 7. Monorepo Reality (what is actually in the repo)

```text
TransitOps_Odoo2026/
├── backend/                 # FastAPI + SQLite (plug-and-play repository)
│   ├── app/
│   │   ├── main.py
│   │   ├── core/            # security.py, dependencies.py
│   │   ├── db/
│   │   │   ├── base.py      # Abstract repository contract
│   │   │   ├── sqlite_repo.py
│   │   │   ├── session.py   # db = SQLiteRepository()
│   │   │   └── init/        # init_db.py, seed_db.py, dummy_data.json
│   │   └── modules/         # auth, fleet, destinations, trips, maintenance, schemas
│   ├── database/transitops.db
│   ├── .env.example
│   └── pyproject.toml       # uv-managed deps
│
├── frontend/                # Next.js 15 + Tremor Planner template (NOT Vite React)
│   ├── src/app/             # App Router
│   ├── src/components/      # Tremor-style UI primitives + charts
│   └── src/data/data.ts     # Dummy quote data (template leftover)
│
├── TransitOps Smart Transport Operations Platform.pdf
└── README.md
```

### Important stack note
The original structural blueprint described **Vite + React Router**.  
**Actual frontend is Next.js 15 (App Router) + Tremor Planner + Tailwind + Recharts + next-themes.**  
Build against the real stack, not the Vite blueprint.

---

## 8. Backend Architecture Insights

### Pattern: Abstract Repository
- `BaseRepository` defines all data operations (CRUD + queries)
- `SQLiteRepository` implements them with SQL
- Routers only talk to `db` from `session.py` — **no SQL in business routes**
- Swap storage later by changing `session.py` (Postgres / memory) without touching modules

### Security
- Native `bcrypt` hashing (not passlib runtime path)
- JWT (`HS256`), 60 min expiry, `SECRET_KEY` from `.env`
- `OAuth2PasswordBearer` → Swagger-friendly login (`tokenUrl=api/auth/login`)
- `RoleChecker(allowed_roles)` dependency; **Admin always bypasses**

### Env (`.env.example`)
```env
DB_PATH=/path/to/.../backend/database/transitops.db
JSON_PATH=/path/to/.../backend/app/db/init/dummy_data.json
SECRET_KEY="..."
```

### How to run backend
```bash
cd backend
# set .env paths first
python -m app.db.init.init_db
python -m app.db.init.seed_db
uv run uvicorn app.main:app --reload
```

---

## 9. Backend API Map (current)

| Method | Path | Auth / Roles | Status |
|--------|------|--------------|--------|
| POST | `/api/auth/login` | Public (form: username=email, password) | Done |
| POST | `/api/auth/register` | Public | Done |
| GET/POST/PUT/DELETE | `/api/fleet/vehicles` | Auth; write = Fleet Manager | Done |
| GET/POST/PUT/DELETE | `/api/fleet/drivers` | Auth; write = Fleet Manager / Safety Officer | Done |
| GET/POST/DELETE | `/api/destinations` | Auth; write = Fleet Manager | Done |
| GET/POST | `/api/trips/` | Auth; create = Fleet Manager / Driver | Done (partial) |
| PUT | `/api/trips/{id}/complete?final_odometer=` | Auth | Done |
| GET/POST | `/api/maintenance/` | Auth; write = Fleet Manager | Done |
| PUT | `/api/maintenance/{id}/close` | Fleet Manager | Done |
| — | Expenses / Fuel routes | — | **Missing** |
| — | Dashboard / Analytics routes | — | **Missing** |
| — | Trip cancel | — | **Missing** |
| — | Draft → Dispatch transition | — | **Missing** (create jumps to Dispatched) |
| — | Available-only vehicle/driver pools | — | **Missing** (filter client-side or add endpoints) |
| — | Documents / CSV / PDF | — | **Missing** |

### Trip create behavior today
On successful validation, trip is created with status **`Dispatched`** immediately and assets go **On Trip**.  
PDF lifecycle includes **Draft** first — decide whether to keep auto-dispatch or add Draft + explicit dispatch.

### Known backend bugs / smells
1. **Duplicate decorator** on login in `auth.py` (`@router.post("/login")` appears twice)
2. `config.py` from the blueprint does **not** exist; paths loaded ad-hoc via `dotenv` in multiple files
3. Seed `dummy_data.json` has **placeholder** password hashes (`$2b$12$xyz...`) — real bcrypt hashes needed for demo logins
4. `pyproject.toml` lists `passlib[bcrypt]` + code imports raw `bcrypt` — ensure `bcrypt` package is installed
5. Complete-trip takes `final_odometer` as query param (no fuel liters yet on complete)

---

## 10. Repository Contract (DB methods)

Already defined on `BaseRepository` / implemented in `SQLiteRepository`:

**CRUD:** drivers (4), vehicles (4), users (4)  
**Expenses:** `add_expense`, `get_expenses_by_trip` *(repo only — no API module)*  
**Destinations:** create, get_all, delete  
**Maintenance:** create, update, get_all, get_by_vehicle  
**Trips:** create, update_status, get_all, filters by driver/vehicle/source/destination  
**Status patches:** `update_vehicle_status`, `update_driver_status`

### Extra methods that may still be useful later
- `get_all_expenses` / expenses by vehicle
- `get_available_vehicles` / `get_available_drivers` (status + license filters)
- Aggregations for KPIs / ROI / fuel efficiency
- Document blob/path storage methods (bonus)

---

## 11. Frontend Reality

### What exists
- Tremor **Planner** SaaS shell (sidebar, breadcrumbs, theme provider, dark mode)
- Dummy pages under `/quotes/overview`, `/monitoring`, `/audits`
- UI kit: Button, Input, Select, Table, Badge, Drawer, Charts (Bar/Combo), MetricsCards
- `next-themes` dark mode already wired in root layout
- Recharts available for analytics charts

### What does **not** exist yet (critical gaps)
- No TransitOps routes (`/dashboard`, `/fleet`, `/drivers`, `/trips`, `/maintenance`, `/finance`)
- No Login page / AuthContext / JWT storage
- No Axios/fetch API service layer wired to FastAPI
- No RBAC-aware sidebar
- Branding still template defaults (`siteConfig.name = "Planner"`, sidebar says "Innovex Systems", user "Emma Stone")
- **`src/lib/` is missing** — many components import `@/lib/utils` (`cx`, `focusRing`). This must be restored or the frontend will not build cleanly
- No `services/api` / endpoints mapping

### Screenshots shared (WhatsApp)
UI shell shown as **TransitOps** with nav: Dashboard / Fleet / Drivers and footer role **Fleet Manager**, empty dashboard body.  
That branding is **not fully reflected in the current checked-in frontend** (still Planner/Innovex). Treat screenshots as target direction / teammate WIP to align with.

### How to run frontend
```bash
cd frontend
pnpm install
pnpm run dev
# http://localhost:3000
```

---

## 12. Gap Analysis — Built vs Remaining

### Backend — largely Phase 1–2 complete
| Area | State |
|------|-------|
| Schema + seed pipeline | Done |
| Abstract SQLite repo | Done |
| Auth JWT + RBAC | Done |
| Fleet CRUD | Done |
| Destinations | Done |
| Trip dispatch + complete + validations | Mostly done |
| Maintenance open/close cascades | Done |
| Expenses module (API) | **Not started** |
| Dashboard/KPI compiler | **Not started** |
| Cancel trip | **Not started** |
| Fuel on trip completion | **Partial / missing** |
| Export CSV/PDF | **Not started** |
| Documents locker | **Not started** |
| License expiry checker / banners | **Not started** |
| Postgres / mock swap | Not started (SQLite only; architecture ready) |

### Frontend — still mostly template
| Area | State |
|------|-------|
| Layout shell + dark mode | Done (template) |
| Charts components | Done (reusable) |
| TransitOps rebrand + nav | **Needed** |
| Auth UI + protected routes | **Needed** |
| Fleet / Drivers pages | **Needed** |
| Trip dispatch UI | **Needed** |
| Maintenance UI | **Needed** |
| Expenses / Finance UI | **Needed** |
| Dashboard KPIs live data | **Needed** |
| Search / sort / filters | Template patterns exist; wire to real data |
| Restore `src/lib/utils` | **Needed for build** |

---

## 13. Suggested Build Order (when work is assigned)

Keep the 8-hour plan, adjusted to current progress:

1. **Stabilize foundations**
   - Fix auth login double-decorator
   - Seed real bcrypt users
   - Restore frontend `lib/utils`
   - CORS already `*`; point frontend to `http://localhost:8000`

2. **Wire frontend spine**
   - AuthContext + login
   - API client with Bearer interceptor
   - Rebrand sidebar → Dashboard / Fleet / Drivers / Trips / Maintenance / Finance
   - Role-gated links

3. **Close backend gaps for demo**
   - Expenses CRUD routes
   - Cancel trip + status restore
   - Dashboard analytics endpoint(s)
   - Optional: available pools endpoint

4. **Module UIs**
   - Fleet + Drivers tables/forms
   - Trip create with available-only dropdowns + weight warning
   - Maintenance actions
   - Expense forms

5. **Polish / bonuses if time**
   - Charts on dashboard
   - CSV export
   - License expiry badges
   - Document uploads
   - PDF

---

## 14. Role × Feature Matrix (for UI gating)

| Feature | Fleet Manager | Driver | Safety Officer | Financial Analyst | Admin |
|---------|---------------|--------|----------------|-------------------|-------|
| View dashboard | ✓ | ✓ | ✓ | ✓ | ✓ |
| Vehicles CRUD | ✓ | read | read | read | ✓ |
| Drivers CRUD | ✓ | read | ✓ write | read | ✓ |
| Destinations write | ✓ | — | — | — | ✓ |
| Create / dispatch trips | ✓ | ✓ | — | — | ✓ |
| Complete trips | ✓ | ✓ | — | — | ✓ |
| Maintenance | ✓ | — | — | — | ✓ |
| Expenses / finance | ✓ | — | — | ✓ | ✓ |
| Compliance focus | — | — | ✓ | — | ✓ |

---

## 15. KPI Formulas (implement on backend)

```text
Fleet Utilization % =
  (vehicles with status On Trip) / (total non-Retired vehicles) * 100

Fuel Efficiency =
  sum(trip distances or odometer deltas) / sum(fuel liters)

Operational Cost (per vehicle) =
  sum(Fuel costs) + sum(Maintenance costs) [+ tolls if included]

Vehicle ROI =
  (Revenue - (Maintenance + Fuel)) / AcquisitionCost
```

---

## 16. Seed Snapshot (dummy_data.json)

Useful demo entities already defined:

- Users: manager / safety / finance emails *(hashes need regeneration)*
- Destinations: Warehouse Alpha, Distribution Hub Beta, Port Logistics Gamma
- Vehicles: VAN-05 (Available), TRK-12 (On Trip), VAN-09 (In Shop)
- Drivers: Alex (Available), Bob (On Trip), Charlie (Suspended + expired license 2025-01-01)
- Trip #1: TRK-12 / Bob, Dispatched
- Maintenance #1: VAN-09 oil/brakes, Open
- Expenses: Fuel 60L/$90 + Toll $35 on trip 1

Charlie + VAN-09 are perfect for **negative validation demos**.

---

## 17. Decisions Already Taken (do not reverse casually)

1. **SQLite now**, abstract repo for future Postgres — not dual-write
2. Frontend is **Next.js Tremor Planner**, not Vite SPA
3. Business rules live in **FastAPI modules**, not only React
4. Admin role exists as testing superuser
5. Trip create currently **auto-dispatches** (skips Draft) — change only intentionally

---

## 18. Waiting For Next Instruction

Context loaded. Remaining work will be assigned by the user (modules not yet built).  
When tasks arrive: plan first → implement → keep this file updated if architecture decisions change.

### Quick “where to look”

| Concern | Path |
|---------|------|
| App entry / routers | `backend/app/main.py` |
| JWT / hash | `backend/app/core/security.py` |
| RBAC | `backend/app/core/dependencies.py` |
| DB contract | `backend/app/db/base.py` |
| SQL impl | `backend/app/db/sqlite_repo.py` |
| Pydantic models | `backend/app/modules/schemas.py` |
| Trip rules | `backend/app/modules/trips.py` |
| Frontend shell | `frontend/src/app/layout.tsx` |
| Sidebar | `frontend/src/components/ui/navigation/AppSidebar.tsx` |
| Problem statement | `TransitOps Smart Transport Operations Platform.pdf` |

---

*End of project context.*
