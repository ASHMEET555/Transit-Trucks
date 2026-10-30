# Feature tracking — frontend → backend sync

Every frontend addition that needs a backend field, endpoint, or migration.
Single source for the backend build. Update with each change.

## Status: frontend-built, backend-pending

### B2.1 Destination coordinates (migration 000006)
- `destinations.latitude` DOUBLE NULL [-90,90], `longitude` DOUBLE NULL [-180,180].
- Seed hub coords. Null-coord destinations listed, never mapped.
- Needed by: live map markers, add-route flow, daily report route lines.

### B2.2 GET /api/v1/dashboard/utilization?days=7
- Per-day dispatched/completed counts. Role-scoped like /dashboard/kpis.
- Needed by: dashboard utilization tiles/bars (currently derived client-side).

### B2.3 GET /api/v1/trips/live
- Active trips + origin/dest coords, dispatched_at, planned_distance,
  `estimated_progress` = elapsed×assumed_speed/planned_distance, clamped 0–1.
- Role scope: Driver own trip only; Safety/Finance no driver names/licenses.
- Document formula + assumed speed in docs/LIVE_MAP.md.
- Needed by: dashboard map panel, live-map markers, add-route preview.

### B2.4 Vehicle extended fields (migration)
- `photo_url`, `insurance_copy_url`, `fitness_cert_url`, `rc_copy_url`, `puc_cert_url`
  (file refs; storage decision open), `owner_name`, `owner_address`,
  `owner_contact_number`, `rto_name`, `rc_expiry_date`, `fuel_type` (enum),
  `insurance_policy_number`, `insurance_expiry_date`, `fitness_expiry_date`,
  `pollution_expiry_date`.
- Needed by: vehicle register/edit form + detail drawer.
- Frontend (step2 wiring): register/edit form sends the full body incl. new fields
  (empty strings when unset; `fuel_type` empty unless chosen); edit via
  PUT /fleet/vehicles/{reg} with the plate locked. Photo: file input → local
  data-URL preview written into `photo_url`, plus URL field fallback.
  Detail drawer shows photo, doc links, owner/RTO/fuel/policy fields, and expiry
  dates with expired / expiring-soon (30d) highlight. See B2.10 for upload decision.

### B2.5 Clients (new resource)
- CRUD `/api/v1/clients`: name, code (unique), type enum
  (Customer|Consignee|Consignor|Consignor & Consignee), address, city, state,
  pin_code, email, mobile, gst_number, pan_entity, pan, branches, ledger_id,
  tds_master, client_to, business_verticals[], point_based_cn bool,
  default_branch bool, custom_fields JSONB.
- Ledger modal: ledger name/code, under_group, tally_account, opening_balance,
  opening_balance_date, credit_days, credit_limit, branches, bank_accounts[].
- Needed by: Clients page, add-client form, trip create (client link, optional).

### B2.6 Users admin (Admin only)
- GET /api/v1/users (paginated), POST /api/v1/users (create: name, email,
  phone, role, tags), PATCH role/active, soft-delete.
- Needed by: Users admin section.

### B2.7 Daily trip report endpoint
- GET /api/v1/reports/daily?company&date/group/vehicle: summary
  (runnings, km, alarms, avg speed), fuel summary (used, mileage, filling/theft
  alarms), history rows (vehicle, start/end, locations, distance, duration,
  mileage, fuel, alarms).
- Needed by: Reports page (currently static layout + real trip data where mapped).

### B2.8 Notifications enrichment
- Payload already `{days_remaining, expiry_date}` — frontend formats it.
- Wanted: alert-type registry (license_expiring, maintenance_due, trip_event)
  with title/body template keys so no client-side string building.
- Needed by: notifications page human-readable rows.

### B2.9 Add-route flow
- POST /api/v1/trips/ already accepts source/destination names; wanted:
  name→coordinate resolution (destinations lookup, 404 with "unknown stop"),
  optional boarding_points[] passthrough stored on trip.
- Needed by: dashboard "add route" floating window map preview.

### B2.10 Vehicle photo/docs upload
- Decision open: multipart upload vs URL-only fields. Frontend sends URLs today.
- Frontend today: URL-only to backend; photo file input converts to a data URL
  client-side for preview (no multipart yet).
- Needed by: vehicle form attachments.
