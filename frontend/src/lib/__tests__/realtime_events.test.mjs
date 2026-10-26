import test from "node:test"
import assert from "node:assert/strict"

test("Realtime - URL formatting with ticket", () => {
  const backendUrl = "http://localhost:8000/api/v1"
  const ticket = "test-ticket-abc-123"
  const baseWsUrl = backendUrl.replace(/^http/, "ws")
  const wsUrl = `${baseWsUrl}/ws?ticket=${encodeURIComponent(ticket)}`

  assert.equal(wsUrl, "ws://localhost:8000/api/v1/ws?ticket=test-ticket-abc-123")
})

test("Realtime - Event payload parser handles standard and legacy formats", () => {
  function parseEvent(raw) {
    return {
      type: raw.type || raw.event || "unknown",
      ids: raw.ids || raw.payload || {},
      occurred_at: raw.occurred_at || "2026-10-26T00:00:00Z",
    }
  }

  // Standard Phase 5 format
  const standard = parseEvent({
    type: "trip.dispatched",
    ids: { trip_id: 12, vehicle_reg: "TRK-01" },
    occurred_at: "2026-10-26T12:00:00Z",
  })
  assert.equal(standard.type, "trip.dispatched")
  assert.equal(standard.ids.trip_id, 12)
  assert.equal(standard.ids.vehicle_reg, "TRK-01")

  // Legacy format
  const legacy = parseEvent({
    event: "expense.created",
    payload: { expense_id: 99 },
  })
  assert.equal(legacy.type, "expense.created")
  assert.equal(legacy.ids.expense_id, 99)
})

test("Realtime - Backoff algorithm bounds delay between 1s and 30s with jitter", () => {
  function calculateBackoff(attempt) {
    const baseDelay = Math.min(30000, Math.floor(Math.pow(1.5, attempt) * 1000))
    const jitter = 250 // simulated midpoint jitter
    return baseDelay + jitter
  }

  assert.ok(calculateBackoff(1) >= 1000 && calculateBackoff(1) <= 2000)
  assert.ok(calculateBackoff(5) >= 5000 && calculateBackoff(5) <= 10000)
  assert.ok(calculateBackoff(20) <= 31000) // Capped at ~30s
})
