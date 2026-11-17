## 🏗️ Architecture Blueprint

The platform strictly decouples structural queries from route layers using an **Abstract Repository Pattern**, allowing developers to swap out storage engines (e.g., switching to PostgreSQL) instantly by changing a single session constructor.

```text
backend/
├── app/
│   ├── config.py         # Dynamic absolute path environments manager
│   ├── main.py           # FastAPI Application factory and routing setup
│   │
│   ├── core/               # Cross-cutting security infrastructure
│   │   ├── security.py     # Native bcrypt hashing & stateless JWT providers
│   │   └── dependencies.py # Gatekeeper RBAC middleware with Admin bypass
│   │
│   ├── db/                # Decoupled Plug-and-Play Database Storage
│   │   ├── base.py        # Strict interface defining required operations
│   │   ├── sqlite_repo.py # Core SQLite query implementations
│   │   ├── session.py     # Decoupled single source of truth for database context
│   │   └── init/          # Standalone setup infrastructure
            ├── dummy_data.json       # Complete boilerplate operational seed records
│   │       ├── init_db.py            # Dynamic multi-table schema build pipeline
│   │       └── seed_db.py            # High-velocity JSON context loader
│   │
│   └── modules/            # Core Business Routines & Schemas
│       ├── auth.py         # OAuth2 compliance forms & token generator
│       ├── fleet.py        # Fleet registries (Vehicles & Drivers)
│       ├── destinations.py # Master Distribution Hub tracking index
│       └── schemas.py      # Strict Pydantic parsing rules and field checks
│
├── database/transitops.db         # Active engine database file
└── .env                           # Dynamic environment configurations file
└── .env.example                   # Environment example
```

---

## 💾 Done Till Now

### 1. Unified Relational Schema (`app/db/init_db.py`)

Built a single-run database layout script introducing structural entities:

* **Users:** System operators mapped cleanly by email credentials and explicit operational capabilities.
* **Destinations:** An interconnected system catalog indexing core delivery hubs and source locations.
* **Vehicles:** Tracking identification plates, max capability metrics, running odometers, and dynamic lifecycle states.
* **Drivers:** Managing identification parameters, active license constraints, expiring timestamps, and historical behavior tracking scores.
* **Trips:** The state-machine centerpiece connecting cargo payload weight demands, planned lines, tracking milestones, and revenue logs.
* **Maintenance Logs:** Workshop logs designed to toggle resource status conditions out of operational pools.
* **Expenses:** Explicit operational line billing (Tolls, Fuel, Surge-fees) linked directly to the parent trip contexts.



### 2. Plug-and-Play Contract Database Layer

* **`base.py`:** Standardized clear interfaces guaranteeing exactly 4 CRUD operations for Drivers, 4 for Vehicles, and 4 for Users alongside detailed methods for Destinations, Expenses, Maintenance, and Trip allocations.
* **`sqlite_repo.py`:** Conceals complex SQL transformations entirely inside abstract functions. No SQL strings escape into routers or controllers.
* **`session.py`:** Eradicates internal Python circular import traps by isolating instance storage bindings completely from `main.py`.

### 3. Identity Assurance & Secure RBAC Gateways

* Replaced vulnerable abstractions with raw, high-velocity native `bcrypt` cryptographic wrappers.
* Implemented `OAuth2PasswordBearer` validation matching FastAPI Swagger UI's internal requirements.
* **Admin Power Bypass:** Injected a global authorization master condition within `RoleChecker`. The `Admin` scope automatically bypasses standard route constraints to dramatically accelerate testing cycles.

### 4. Fully Documented API Endpoints

Exposed a production-ready API layout including:

* `POST /api/auth/register` - Formats, validates, hashes, and registers a system operator.
* `POST /api/auth/login` - Form-urlencoded parser returning instant JWT security tokens.
* `GET/POST/PUT/DELETE /api/fleet/vehicles` - Comprehensive validation for fleet additions (enforcing unique plate criteria).
* `GET/POST/PUT/DELETE /api/fleet/drivers` - Strict management of active operators.
* `GET/POST/DELETE /api/destinations` - Master list controls for managing waypoint hubs.

---

## 🛠️ Setup & Running Execution Flow

### 1. Configure the Environment

Ensure your `.env` variables list absolute system file placements at the backend root level:

```env
DB_PATH=/home/your-user/.../backend/transitops.db
JSON_PATH=/home/your-user/.../backend/dummy_data.json

```

### 2. Generate and Seed the Schema

Fire the database provisioning pipes sequentially to set up fresh records:

```bash
# Build the tables structure
python -m app.db.init.init_db

# Hydrate tables from the mock JSON context
python -m app.db.init.seed_db

```

### 3. Launch the Application Server

Boot up the engine using your dependencies runner:

```bash
uv run uvicorn app.main:app --reload

```
