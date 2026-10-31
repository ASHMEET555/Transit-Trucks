# Live-map verification log

Commands were each run once from a clean state unless noted. Host: Windows,
Go 1.26.3 with 32-bit-only MinGW gcc (no `-race` support — see below).

## Backend

- `go vet ./...` → clean, no output.
- `CGO_ENABLED=0 go test -count=1 ./...` (seeded dev DB, `SEED_PASSWORD=truck@555`)
  → every package `ok` except the known-failing list (empty: all pass,
  including `TestHygiene_NoLocalStorageInFrontendAuth` after the
  memory-store refactor, `TestCompulsory_MigrationUpDownUp`,
  `TestOpenAPITypesDrift` after `gen`, and all new live/route/sim tests).
- `go test -race -count=1 ./...` → NOT RUNNABLE on this host. Exact output:
  `# runtime/cgo` / `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`
  / `FAIL ... [build failed]` for every package with tests. Environmental:
  the only C compiler present is 32-bit MinGW.org GCC 6.3. Non-race run
  above substitutes.
- Migrations: `000006` up/down/up tested on a scratch DB; `000007`
  up/down/up via `cmd/migrate` CLI on both scratch and dev DBs
  (dev left at version 7, dirty=false). Note: `000007` had to be made
  idempotent (conditional renames) because `TestCompulsory_MigrationUpDownUp`
  does `force 5` + `up` on an already-migrated schema.

## Frontend

- `npm run typecheck` → clean. `npm run lint` → no warnings/errors.
- `npm test` (node --test) → 25 pass, 0 fail (incl. 7 new live-map tests:
  polyline decode, dropdown filter, progress==API value, E6 labels,
  ETA/age, clustering, forbidden-wording grep).
- `npm run build` → success.

## E2E (Playwright, chromium, seeded dev DB)

- `e2e/live-map.spec.ts` (4 tests) → 4 passed:
  - marker moves between 5 s snapshots (bounding-box delta > 2 px)
  - each rendered route decodes to > 2 points; dropdown changes the card
  - overview screenshots at 1440px and 390px (in this dir)
  - axe: zero serious/critical violations on the overview at both widths
  - Operational note: Playwright `reuseExistingServer` keeps the backend
    alive across runs; a stale pre-change server on :8000 (a `go run` exe)
    caused 404s and flakes — kill the PID listening on :8000 before
    re-running.
- `e2e/realtime.spec.ts` (M31) → FAILED in this environment, proven
  PRE-EXISTING and not a regression: the identical failure (driver never
  sees the dispatched trip) reproduces against a true baseline backend
  (pre-change commit `51c80f6`, current frontend). Baseline server log
  shows `RATE_LIMIT_EXCEEDED` on `/api/v1/realtime/ticket`, so the driver
  never gets a websocket; with a 25 s window the trip still never appears.
  Same Windows-env websocket/rate-limit flakiness already visible in
  `phase5_*` backend tests (`read tcp ... i/o timeout`). SLO thresholds
  were not changed to accommodate it. Fixture note: the dev DB was missing
  `VH-1005` (M31 assumption); re-inserted as Available/Van for the runs.

## OSRM route count / cache behaviour (observed)

- Dispatch Chicago→Denver with OSRM down: 1 `route_geometries` row,
  `provider=approximate`, 2-point straight polyline (UI draws dashed).
- `TestDispatchCachesRoutePair`: 2 dispatches of the same pair → exactly
  1 OSRM call; second is a cache hit.
- Live smoke (dev server + simulator, SIM_TIME_SCALE=60): simulator tracked
  with seed offset, posted ~23 positions; board showed `status:fresh` with
  backend-computed `progress_m/route_length_m/progress_pct`.
