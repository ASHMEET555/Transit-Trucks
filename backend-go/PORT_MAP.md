# Go port route inventory

| Route | Python module | Rule |
|---|---|---|
| `POST /api/auth/login`, `register` | auth | form login; role and duplicate checks |
| `/api/fleet/vehicles`, `/drivers` | fleet | authenticated reads; role-protected CRUD |
| `/api/destinations/` | destinations | authenticated list; manager mutation |
| `/api/trips/`, `/suggest`, `/{id}/complete`, `/{id}/cancel` | trips | dispatch availability, capacity and licence checks; lifecycle cascades |
| `/api/maintenance/`, `/{id}/close` | maintenance | cannot open for On Trip; status cascade |
| `/api/finance/expenses`, `/{trip_id}` | finance | allowed type, trip and fuel-litres checks |
| `/api/dashboard/kpis` | dashboard | Python vehicle/trip/expense aggregation logic |
| `/`, `/ws/fleet` | main/realtime | root health and event broadcast |
