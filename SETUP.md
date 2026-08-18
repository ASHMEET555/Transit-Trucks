# TransitOps — Local Setup (Current Codebase)

Use this as the source of truth for running the project after the latest auth, analytics, seed data, and API wiring. Peers can copy sections into `README.md`.

---

## Prerequisites

| Tool | Notes |
|------|--------|
| **Git** | Clone the repo |
| **Python 3.11+** | Backend |
| **[uv](https://docs.astral.sh/uv/)** | Backend dependency + runner |
| **Node.js 18+** and **npm** | Frontend |

---

## 1. Clone

```bash
git clone https://github.com/Reaper-ai/TransitOps_Odoo2026.git
cd TransitOps_Odoo2026
```

---

## 2. Backend setup

### 2.1 Environment file

```bash
cd backend
cp .env.example .env
```

Edit `.env` and set **absolute paths** for your machine:

**Windows (example):**
```env
DB_PATH=E:/Hackathons/odoo/TransitOps_Odoo2026/backend/database/transitops.db
JSON_PATH=E:/Hackathons/odoo/TransitOps_Odoo2026/backend/app/db/init/dummy_data.json
SECRET_KEY=SUPER_SECRET_COMPROMISED_HACKATHON_KEY_CHANGE_THIS
```

**macOS / Linux (example):**
```env
DB_PATH=/Users/YOU/TransitOps_Odoo2026/backend/database/transitops.db
JSON_PATH=/Users/YOU/TransitOps_Odoo2026/backend/app/db/init/dummy_data.json
SECRET_KEY=SUPER_SECRET_COMPROMISED_HACKATHON_KEY_CHANGE_THIS
```

> `JSON_PATH` must point at `backend/app/db/init/dummy_data.json` (expanded seed file).

### 2.2 Install dependencies

```bash
cd backend
uv sync
```

### 2.3 Create tables + seed data

Run from the `backend` folder (so `.env` loads correctly):

```bash
uv run python -m app.db.init.init_db
uv run python -m app.db.init.seed_db
```

- `init_db` creates SQLite tables if missing.
- `seed_db` loads users, destinations, vehicles, drivers, trips, maintenance, and expenses from `dummy_data.json`.

**Fresh reseed (optional):** if you already have old data and want a clean load, stop the API server, delete `backend/database/transitops.db`, then run `init_db` + `seed_db` again.

### 2.4 Start the API

```bash
cd backend
uv run uvicorn app.main:app --reload
```

| URL | Purpose |
|-----|---------|
| http://127.0.0.1:8000 | API root |
| http://127.0.0.1:8000/docs | Swagger UI |

Keep this terminal running.

---

## 3. Frontend setup

Open a **second** terminal:

```bash
cd frontend
npm install
npm run dev
```

App: **http://localhost:3000**

Frontend calls the API at `http://localhost:8000/api` by default (`frontend/src/data/data.ts`). Optional override:

```env
NEXT_PUBLIC_API_URL=http://localhost:8000/api
```

---

## 4. Demo logins

Password for all seeded accounts: **`password123`**

| Email | Role |
|-------|------|
| `manager@transitops.com` | Fleet Manager |
| `safety@transitops.com` | Safety Officer |
| `finance@transitops.com` | Financial Analyst |
| `admin@transitops.com` | Admin |

Extra accounts may exist in the expanded seed (e.g. `user01@transitops.com` …) — same password.

You can also **Create account** from the login page (`/register`) with roles: Fleet Manager, Driver, Safety Officer, Financial Analyst.

---

## 5. Quick smoke check

1. Backend up on `:8000`, frontend on `:3000`.
2. Open http://localhost:3000 → redirected to `/login`.
3. Sign in as `manager@transitops.com` / `password123`.
4. Confirm **Dashboard**, **Fleet**, **Drivers**, **Trips**, **Maintenance**, **Fuel & Expenses**, **Analytics** load data.
5. Optional: open http://127.0.0.1:8000/docs and try `POST /api/auth/login`.

---

## 6. Project layout (relevant paths)

```text
TransitOps_Odoo2026/
├── backend/
│   ├── .env                 # local paths + SECRET_KEY (do not commit secrets)
│   ├── .env.example
│   ├── database/transitops.db
│   └── app/
│       ├── main.py
│       └── db/init/
│           ├── init_db.py
│           ├── seed_db.py
│           └── dummy_data.json
└── frontend/
    ├── package.json
    └── src/
        ├── app/             # pages (login, register, dashboard, analytics, …)
        ├── data/data.ts     # BACKEND_URL
        └── lib/api.ts       # JWT api client
```

---

## 7. Common issues

| Issue | Fix |
|-------|-----|
| Frontend 401 / empty data | Backend not running, or wrong `BACKEND_URL` / CORS host |
| Seed does nothing new | `INSERT OR IGNORE` skips existing keys — delete DB and re-run init + seed |
| `.env` path errors | Use absolute paths; on Windows prefer forward slashes |
| Port 3000 or 8000 in use | Stop the other process or change the port |
| Analytics empty for some roles | Role must have analytics access (e.g. Fleet Manager, Financial Analyst, Admin) |

---

## 8. Daily run (after first setup)

**Terminal 1 — backend**
```bash
cd backend
uv run uvicorn app.main:app --reload
```

**Terminal 2 — frontend**
```bash
cd frontend
npm run dev
```

Then open http://localhost:3000 and sign in.
