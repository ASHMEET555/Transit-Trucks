# TransitOps — Step 2 Data Plan & Wiring Matrix

## Web-Research Summary (Exa / Industry UX Standards)
Research on modern fleet and dispatch systems (Samsara, Motive, Flexport dispatch) highlights:
1. **Calm UI Under High Density**: A single operational centerpiece (the active dispatch board / live map); all supporting information is quiet, scannable, and tabular.
2. **Attention-First Layout**: Top-level 4 KPI strip with micro-indicators; exception queue surfaced as jump-links directly to actionable entries (license expiring, open repairs, unassigned drafts).
3. **Map ⇄ Table State Alignment**: Live map shows active operations with estimated progress disclosure ("Estimated position, no GPS"). No fake telemetry or fictitious stops.
4. **Restrained Color Palettes**: Status badges use semantic shapes + color (never color alone); actions gated strictly by permissions and validations.

---

## Data Wiring Matrix

| Element | Shows | Data Source | Action | Phase 6 Ref |
|---|---|---|---|---|
| **KPI Strip** (Active trips, Available vehicles, In shop, Fleet Utilization, Op Cost, Revenue) | Operational fleet numbers | `GET /api/v1/dashboard/kpis` (or `/api/v1/dashboard/me` for Driver) | Wire | §6.1 |
| **7-Day Utilization Chart** | Dispatched vs Completed counts per day | `GET /api/v1/dashboard/utilization?days=7` | Wire | §6.2 |
| **Exceptions Queue** | License expiries (≤30d), Open repairs, Draft trips | `GET /api/v1/fleet/drivers?license_expiring_within_days=30`, `GET /api/v1/maintenance/`, `GET /api/v1/trips/?status=Draft` | Wire | §6.1 |
| **CSV Export** | Export operations & financial metrics | `GET /api/v1/dashboard/export.csv` | Wire | §6.1 |
| **Trips Board & Table** | Kanban columns (Draft, Dispatched, Completed, Cancelled) & dense table | `GET /api/v1/trips/`, `POST /api/v1/trips`, `POST /api/v1/trips/{id}/dispatch`, `POST /api/v1/trips/{id}/complete`, `POST /api/v1/trips/{id}/cancel` | Wire | §6.3 |
| **Trip Suggestion Modal** | Pair vehicle + driver with payload match | `POST /api/v1/trips/suggest` | Wire | §6.3 |
| **Trip Audit History** | Status transition timeline | `GET /api/v1/audit?entity=trips&entity_id={id}` | Wire | §6.3 |
| **Vehicles Registry** | Vehicle roster, specs, status, retirement | `GET /api/v1/fleet/vehicles`, `POST /api/v1/fleet/vehicles`, `PATCH /api/v1/fleet/vehicles/{reg}`, `POST /api/v1/fleet/vehicles/{reg}/retire`, `DELETE /api/v1/fleet/vehicles/{reg}` | Wire | §6.4 |
| **Drivers Registry & Compliance** | Drivers list, license status, status transitions | `GET /api/v1/fleet/drivers`, `POST /api/v1/fleet/drivers`, `PATCH /api/v1/fleet/drivers/{lic}`, `POST /api/v1/fleet/drivers/{lic}/set-available`, `POST /api/v1/fleet/drivers/{lic}/set-off-duty`, `POST /api/v1/fleet/drivers/{lic}/suspend`, `POST /api/v1/fleet/drivers/{lic}/reinstate` | Wire | §6.4 |
| **Finance / Expenses Ledger** | Fuel logs, maintenance expenses, other operational costs | `GET /api/v1/expenses`, `POST /api/v1/expenses`, `GET /api/v1/finance/summary` | Wire | §6.5 |
| **Maintenance Work Orders** | Defect reports, open work orders, closing logs | `GET /api/v1/maintenance/`, `POST /api/v1/maintenance/`, `POST /api/v1/maintenance/{id}/close` | Wire | §6.4 |
| **Live Map Dispatched Markers** | Straight line origin→destination with estimated progress | `GET /api/v1/trips/live` | Wire | §6.2 |
| **Live Map Stops / Depots** | Known destination hubs | Seeded 8 hubs with coordinates | Wire | §6.2 |
| **Notifications Drawer** | License expiry alerts & system notices | `GET /api/v1/notifications`, `POST /api/v1/notifications/{id}/read` | Wire | §6.6 |
| **Settings & Security** | User profile, password rotation, theme switch | `GET /api/v1/auth/me`, `POST /api/v1/auth/change-password` | Wire | §6.6 |
| **Audit Logs Viewer** | Tamper-evident operational audit logs | `GET /api/v1/audit` | Wire | §6.6 |
| **Simulated bus routes & stops** | City bus routes and passengers | `src/lib/data/transit-data.ts` | Removed | §6.2 |
| **Hero vanity metrics (1.2M, 98.6%)** | Invented figures | None | Removed | §0 |
| **Speed / Telematics sensors / Harsh braking** | Live sensor telemetry | NOT TRACKED | Labelled / Removed | §0 |
| **Fuel tank levels (%) & GPS breadcrumbs** | Live IoT telemetry | NOT TRACKED | Labelled / Removed | §0 |

---

## Totals
- **Elements Wired**: 16
- **Elements Labelled ("Not tracked")**: 2
- **Elements Removed**: 4
