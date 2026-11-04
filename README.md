# TransitOps — Autonomous Fleet Intelligence & Telematics Superintelligence

[![Meta Global AI Developer Hackathon](https://img.shields.io/badge/Meta_Global_AI_Developer_Hackathon-2026-0668E1?style=for-the-badge&logo=meta&logoColor=white)](https://cerebralvalley.ai/e/meta-ai-developer-hackathon)
[![Theme](https://img.shields.io/badge/Theme-Push_the_Frontier_%26_Build_for_Your_World-10B981?style=for-the-badge)](https://cerebralvalley.ai/e/meta-ai-developer-hackathon)
[![Track](https://img.shields.io/badge/Track-Startup_Track-8B5CF6?style=for-the-badge)](https://cerebralvalley.ai/e/meta-ai-developer-hackathon)
[![Models](https://img.shields.io/badge/Meta_Models-Muse_Wonder_1.0_%7C_Muse_Spark_%7C_SAM_3.1-FF6B6B?style=for-the-badge)](https://ai.meta.com/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)

> Built for the **Meta Global AI Developer Hackathon (2026)**.  
> **TransitOps** is an enterprise-grade Autonomous Fleet Intelligence & Telematics Platform powered by **Meta Muse Wonder 1.0**, **Meta Muse Spark**, and **SAM 3.1** via Meta Model API, deployed on distributed serverless inference with a hardened Go telematics kernel and Next.js 15 polymorphic frontend.

---

## ⚡ Quick Start: How to Run

### Prerequisites
- **Go 1.22+**
- **Node.js 18+** & **npm**
- **PostgreSQL 14+** (running locally on port 5432 or remote DSN)
- **Git**

```bash
# Clone the repository
git clone https://github.com/ASHMEET555/Transit-Trucks.git
cd Transit-Trucks
```

---

### Step 1: Database Setup & Migrations

In PowerShell / Bash, set your PostgreSQL connection string and apply database migrations:

```powershell
cd backend-go
$env:POSTGRES_DSN="postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"

# Apply all migrations (000001 through 000008)
go run ./cmd/migrate up

# Seed 8 continental depots, 10 long-haul corridors, 50 trucks, 40 drivers, and pre-cache OSRM highway routes
go run ./cmd/seed
```

> **Default Seed Credentials**:
> - **Admin**: `admin@transitops.com` / `TransitOps2026!`
> - **Fleet Manager**: `fleet@transitops.com` / `TransitOps2026!`
> - **Safety Officer**: `safety@transitops.com` / `TransitOps2026!`
> - **Financial Analyst**: `finance@transitops.com` / `TransitOps2026!`
> - **Driver**: `driver1@transitops.com` / `TransitOps2026!`

---

### Step 2: Terminal 1 — Start the Backend Go Server

```powershell
cd backend-go
$env:POSTGRES_DSN="postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
$env:SESSION_SECRET="dev-session-secret-must-be-32-chars-long!"
$env:ENV="local"
$env:PORT="8000"
$env:ALLOWED_ORIGINS="http://localhost:3000"
$env:COOKIE_DOMAIN="localhost"
$env:SIM_ENABLED="1"
$env:OSRM_URL="https://router.project-osrm.org"

# Start the high-throughput Go telemetry server (listens on :8000)
go run ./cmd/server
```

---

### Step 3: Terminal 2 — Start the Telemetry & Kinematics Simulator

```powershell
cd backend-go
$env:BACKEND_URL="http://localhost:8000"
$env:POSTGRES_DSN="postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
$env:SIM_ENABLED="1"
$env:SIM_ADMIN_EMAIL="admin@transitops.com"
$env:SIM_ADMIN_PASSWORD="TransitOps2026!"

# Drives 20 dispatched trucks along real interstate road geometries with live fuel, odometer, reefer temps & HOS tracking
go run ./cmd/simulator
```

---

### Step 4: Terminal 3 — Start the Next.js Frontend

```powershell
cd frontend
npm install
npm run dev
```

Open your browser to:
- **Landing & Modular Cards**: [http://localhost:3000](http://localhost:3000)
- **Live Fleet Telematics Map**: [http://localhost:3000/live-map](http://localhost:3000/live-map)
- **Operator Dashboard**: [http://localhost:3000/dashboard](http://localhost:3000/dashboard)

---

## 📸 Platform Overview

### 1. Operations Landing & Modular Card Architecture
![TransitOps Dashboard Preview](frontend/public/images/dashboard-preview.png)

### 2. Live Continental Telematics & Highway Road Navigation Map
![TransitOps Live Fleet Telematics Map](frontend/public/images/live-map-preview.png)

```
+--------------------------------------------------------------------------------------------------------------------+
|  TransitOps Telematics & Fleet Intelligence                                                   Status: Operational  |
+--------------------------------------------------------------------------------------------------------------------+
|  [Active Vehicles: 142]  |  [On-Time Delivery: 97.4%]  |  [Idle Hours: 312h]  |  [Meta Muse Wonder Copilot: Active] |
+--------------------------------------------------------------------------------------------------------------------+
|  [40px Dynamic Clusters] |  [Real Interstate Routes]   |  [Reefer Snowflake Badges]  |  [HOS Compliance Engine]    |
+--------------------------------------------------------------------------------------------------------------------+
```

---

## 🎛️ Core Feature Tiles

<table>
  <tr>
    <td width="33%" valign="top">
      <h3>🚚 Full Trucking Management</h3>
      <ul>
        <li><strong>End-to-End Asset Control:</strong> Semis, reefers, box trucks, flatbeds, and heavy haulers.</li>
        <li><strong>Kinematics Physics:</strong> Cap acceleration at 1.5 m/s² and braking at 2.5 m/s².</li>
        <li><strong>Rotated Semi SVG:</strong> 360° heading direction, status rings, and animated selection pulses.</li>
      </ul>
    </td>
    <td width="33%" valign="top">
      <h3>🗺️ Live Telematics & Highway Routes</h3>
      <ul>
        <li><strong>Real Road Geometry:</strong> Precision-6 OSRM polylines across 10 continental US corridors.</li>
        <li><strong>Cluster Awareness:</strong> Real-time 40px radius clustering for dense depot zones.</li>
        <li><strong>Dark Forest Cards:</strong> Est. ETA, live speed, miles remaining, delay minutes, and tank level.</li>
      </ul>
    </td>
    <td width="33%" valign="top">
      <h3>⛽ Fuel & Expense Intelligence</h3>
      <ul>
        <li><strong>Dynamic Fuel Burn:</strong> Automated depletion calculation (1.1–1.5 km/L) from live road distance.</li>
        <li><strong>Ledger Auditing:</strong> Trip-by-trip fuel, toll, and maintenance expense allocations.</li>
        <li><strong>Export Readiness:</strong> One-click CSV and compliance audit reporting.</li>
      </ul>
    </td>
  </tr>
  <tr>
    <td width="33%" valign="top">
      <h3>🛡️ Safety & FMCSA HOS Engine</h3>
      <ul>
        <li><strong>Hours-of-Service (HOS):</strong> Strict 11-hour driving limit tracking with 10-hour off-duty resets.</li>
        <li><strong>Automated Alerts:</strong> Speeding (>105 km/h), late arrival (>30m), low fuel (<15%), and near HOS limit.</li>
        <li><strong>Driver Scorecards:</strong> Real-time safety scores, violation tracking, and incident logging.</li>
      </ul>
    </td>
    <td width="33%" valign="top">
      <h3>🔐 Enterprise Role-Based Access (RBAC)</h3>
      <ul>
        <li><strong>Admin:</strong> Unrestricted fleet, system, user, and route administration.</li>
        <li><strong>Fleet Manager:</strong> Vehicle dispatch, maintenance, trips, and operations.</li>
        <li><strong>Driver:</strong> Restricted in-cab trip route, assigned stops, and defect logs.</li>
        <li><strong>Safety & Finance:</strong> Anonymized driver PII, safety auditing, and ROI analytics.</li>
      </ul>
    </td>
    <td width="33%" valign="top">
      <h3>❄️ Cold-Chain & Maintenance Ops</h3>
      <ul>
        <li><strong>Reefer Telemetry:</strong> Real-time cargo temperature monitoring (-20°C target) with drift alerts.</li>
        <li><strong>Service Queues:</strong> Work orders for brake pads, reefer units, and battery overhaul.</li>
        <li><strong>Status Cascades:</strong> Auto-switches vehicle state between Available, On Trip, and In Shop.</li>
      </ul>
    </td>
  </tr>
</table>

---

## 🏆 Meta Global AI Developer Hackathon Submission

### Category & Tracks
- **Track**: **Startup Track** — Production-ready, resilient physical-system control and enterprise dispatch.
- **Themes**:
  1. **Push the Frontier**: Bringing Meta foundation models into physical systems — dynamic kinematics control, sensor fusion, predictive failure prevention, and hours-of-service safety telemetry.
  2. **Build for Your World**: Modernizing the trillion-dollar freight logistics industry, eliminating spreadsheet chaos, and giving dispatchers calm, superintelligent operational clarity.

### Meta AI Model Stack Integration
| Meta Model / Technology | Architectural Role in TransitOps | Inference Partner / Runtime |
|---|---|---|
| **Meta Muse Wonder 1.0** | **Autonomous Fleet Dispatch Copilot & Predictive Maintenance**: Reason over multi-sensor telemetry (fuel burn curves, vibration telemetry, ambient delta) to anticipate breakdowns 48h before failure. | Baseten Serverless Endpoints (2M TPM) |
| **Meta Muse Spark** | **Real-Time Highway Incident Reasoner**: Microsecond routing re-evaluations when highway closures, weather hazards, or reefer temperature deviations occur. | Fireworks.ai Ultra-Low Latency |
| **SAM 3.1 (Segment Anything Model 3.1)** | **Depot Satellite & Yard Vision**: Automatic semantic segmentation of trailer parking bays, dock congestion, and container alignment at logistics hubs. | Modal Serverless GPU Sandboxes |
| **Muse Code Agent** | **Continuous Schema & Telematics Synthesis**: Accelerated pipeline generation for source-agnostic telematics ingestion (OBD-II, CAN bus, simulator stream). | Meta Developer Suite |

---

## 🏛️ System Architecture

```mermaid
flowchart TD
    subgraph UI ["Client Presentation Layer (Next.js 15 + Leaflet + Tailwind)"]
        Landing["Landing & Hero (/)" ]
        LiveMap["Live Telematics Map (/live-map)"]
        RoutePanel["Left Filter Panel & Dark Forest Card"]
        TruckMarkers["Custom Semi-Truck SVG Markers (Rotated 360°, Status Rings)"]
    end

    subgraph GoCore ["High-Throughput Telemetry Kernel (Go / backend-go)"]
        Server["HTTP / WebSocket API (:8000)"]
        Ingest["Source-Agnostic Position Ingestion Engine"]
        Kinematics["Kinematics Engine (accel ≤ 1.5 m/s², brake ≤ 2.5 m/s²)"]
        HOS["FMCSA Hours of Service (11h Limit / 10h Off-Duty)"]
        RouteCache["OSRM Road Geometry Cache Engine"]
    end

    subgraph MetaAI ["Meta Superintelligence Layer"]
        MuseWonder["Meta Muse Wonder 1.0 (Predictive Maintenance & Dispatch Reasoning)"]
        MuseSpark["Meta Muse Spark (Corridor Route Optimizer & Incident Triage)"]
        SAM["Meta SAM 3.1 (Yard & Depot Computer Vision Segmentation)"]
    end

    subgraph Infra ["Storage & Inference Partners"]
        Postgres[(PostgreSQL 14+ Relational Engine)]
        OSRM["Project-OSRM Continental Highway Engine"]
        Baseten["Baseten / Modal Inference Pipelines"]
    end

    TruckMarkers --> LiveMap
    RoutePanel --> LiveMap
    LiveMap --> Server
    Server --> Ingest
    Ingest --> Postgres
    Kinematics --> Ingest
    Server --> RouteCache
    RouteCache --> OSRM
    RouteCache --> Postgres
    Server --> MetaAI
    MetaAI --> Baseten
```

---

## 📦 What Was Built: Engineering Highlights

### 1. Source-Agnostic High-Throughput Telemetry Ingestion (Go)
- **Universal Contract**: Single ingestion path (`POST /api/v1/telemetry/positions`) accepting simulated telemetry today and real hardware telematics (CAN bus / OBD-II) with zero schema modifications.
- **Idempotency & Clean Deduplication**: Hardened composite index on `(vehicle_reg, recorded_at)` preventing duplicate writes.
- **Automated Retention**: 48-hour rolling purge on write keeping DB footprints lightweight and zero-maintenance.
- **Strict Gating**: `source=simulator` accepted only when `SIM_ENABLED=1`; unauthenticated device ingestion safely rejected until mutual-TLS device rollout.

### 2. Physics-Based Kinematics Simulation
- **Rigorous Limits**: Enforces acceleration caps ($\le 1.5\,\text{m/s}^2$), braking caps ($\le 2.5\,\text{m/s}^2$), and automated curve slowdowns (40% speed drop when turn angles exceed 25°).
- **Scheduled Dwell**: Realistic 35-minute layovers at delivery depots before next dispatch.
- **Multimodal Telemetry**: Emits dynamic fuel depletion ($1.1 - 1.5\,\text{km/L}$), odometer progression, reefer temperature maintenance ($-20^\circ\text{C} \pm 2^\circ\text{C}$), and active driver driving minutes.

### 3. Real Continental Interstate Routing
- **10 Continental Long-Haul Corridors**:
  1. Reno Hub $\rightarrow$ Salt Lake City Hub (I-80)
  2. Denver Intermodal Hub $\rightarrow$ Los Angeles Freight Hub (I-70 / I-15)
  3. Chicago Logistics Hub $\rightarrow$ Atlanta Distribution Hub (I-65)
  4. Dallas Gateway Hub $\rightarrow$ Houston Freight Depot (I-45)
  5. Seattle Terminal $\rightarrow$ Portland Logistics Hub (I-5)
  6. Kansas City Terminal $\rightarrow$ Dallas Gateway Hub (I-35)
  7. Chicago Logistics Hub $\rightarrow$ New York Central Hub (I-80 / I-90)
  8. Denver Intermodal Hub $\rightarrow$ Kansas City Terminal (I-70)
  9. Atlanta Distribution Hub $\rightarrow$ Dallas Gateway Hub (I-20)
  10. Los Angeles Freight Hub $\rightarrow$ Phoenix Transshipment Depot (I-10)
- **Intelligent Dual-Tier Geometry Resolution**: Queries local OSRM with automatic fallback to public OSRM. Pre-caches full precision-6 polylines directly in PostgreSQL `route_geometries`. Second dispatch is always an instantaneous $O(1)$ cache hit.

### 4. SiteDesign UI & Custom Semi-Truck SVG Markers
- **Polymorphic Marker**: Handcrafted top-down SVG semi-truck with cab, windshield, and corrugated trailer roof.
- **Heading Rotation**: Dynamically rotated $[0^\circ, 360^\circ)$ based on real road bearing.
- **Status Indicator Rings**:
  - `on_route`: Forest Primary (`#1b4332`)
  - `delayed`: Amber Signal (`#d97706`)
  - `alert` / `off_route` / `breakdown`: Destructive Red (`#ef4444`)
  - `offline` / `no_signal`: Muted Slate (`#9ca3af`)
- **Interactive States**: Pulse wave animation on selection, cyan snowflake badge on refrigerated reefers.
- **40px Dynamic Clustering**: Zoom-responsive spatial clustering grouping nearby trucks into clear counter badges.

### 5. Automated FMCSA Safety & Alert Engine
All computations run exclusively on the backend:
- `hos_near_limit`: Triggers when driver driving time $\ge 600$ minutes (under 60 min remaining of legal 11h limit).
- `late_arrival`: Triggers when estimated arrival delay exceeds 30 minutes.
- `speeding`: Triggers when speed exceeds the route maximum limit ($> 105\,\text{km/h}$).
- `low_fuel`: Triggers when tank level drops below 15%.
- `reefer_temp_out_of_range`: Triggers when temperature climbs above $-15^\circ\text{C}$ for cold-chain goods.

---

## 🔒 Enterprise Security & RBAC Matrix

TransitOps enforces role-based access control at the HTTP handler layer:

| Role | Telemetry & Vehicle Scope | Driver PII Visibility | Financial & Expense Data | Maintenance Work Orders |
|---|---|---|---|---|
| **Admin** | Full Fleet (Active + Idle) | Full Name, Phone, License | Full Access | Full Access |
| **Fleet Manager** | Full Fleet (Active + Idle) | Full Name, Phone, License | Read-Only | Create / Assign / Close |
| **Driver** | Assigned Truck & Trip Only | Self Only | Hidden | Report Defects Only |
| **Safety Officer** | All Active Vehicles | **Masked** (Compliance audit) | Hidden | Safety Inspections |
| **Financial Analyst** | All Active Vehicles | **Masked** | Full ROI & Ledger Access | Cost Records Only |

---

## 🧪 Comprehensive Verification Suite

### Backend Unit & Integration Tests
```bash
cd backend-go
# Run all telemetry, HOS, physics, and route cache tests
$env:CGO_ENABLED="0"; go test -v -count=1 -run "TestPhysicsLimits|TestHoursOfServiceRule|TestAlertThresholds|TestEnsureRouteCacheHit" ./internal/app
```
```
=== RUN   TestEnsureRouteCacheHit
--- PASS: TestEnsureRouteCacheHit (0.11s)
=== RUN   TestPhysicsLimits
--- PASS: TestPhysicsLimits (0.00s)
=== RUN   TestHoursOfServiceRule
--- PASS: TestHoursOfServiceRule (0.00s)
=== RUN   TestAlertThresholds
--- PASS: TestAlertThresholds (0.00s)
PASS
ok      github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app    0.197s
```

### Simulator Physics & Dwell Tests
```bash
cd backend-go
$env:CGO_ENABLED="0"; go test -v -count=1 ./cmd/simulator/...
```
```
=== RUN   TestStopAtDestinationThenDwell
--- PASS: TestStopAtDestinationThenDwell (0.00s)
=== RUN   TestNeverCompletesTrips
--- PASS: TestNeverCompletesTrips (0.00s)
PASS
ok      github.com/ASHMEET555/Transit-Trucks/backend-go/cmd/simulator    0.070s
```

### Frontend Typecheck, Lint & Unit Tests
```bash
cd frontend
npm test
npm run typecheck
npm run build
```
- **25 / 25 Unit Tests Passed**
- **0 TypeScript Errors**
- **22 / 22 Static Pages Built & Optimized**

---

## 👥 Hackathon Team & Acknowledgments

Built with dedication for the **Meta Global AI Developer Hackathon 2026**:
- **Ashmeet Singh** — System Architecture, Telematics Go Kernel, Kinematics & Next.js 15 UI
- **Team Collaborators & Mentors** — Meta AI Team, Baseten, Modal, and Fireworks.ai inference engineering support.

---

## 📄 License
This project is open-source under the [MIT License](LICENSE).
