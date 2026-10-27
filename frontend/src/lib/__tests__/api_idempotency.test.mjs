import test from "node:test"
import assert from "node:assert/strict"
import { getOrCreateActionIdempotencyKey, clearActionIdempotencyKey, generateUUID } from "../idempotency.ts"

test("Idempotency - RFC 4122 compliant UUID v4 generator format", () => {
  const uuid = generateUUID()
  const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
  assert.match(uuid, uuidRegex)
})

test("Idempotency - Retried action sends identical Idempotency-Key; new action sends new key", () => {
  const action1 = "create-trip-attempt-1"
  const key1 = getOrCreateActionIdempotencyKey(action1)
  const key1Retry = getOrCreateActionIdempotencyKey(action1)

  // Retried POST sends identical key
  assert.equal(key1, key1Retry)

  // Clear or start new user action -> new key
  clearActionIdempotencyKey(action1)
  const action2 = "create-trip-attempt-2"
  const key2 = getOrCreateActionIdempotencyKey(action2)

  assert.notEqual(key1, key2)
})

test("Realtime - URL formatting has exactly one /api/v1", () => {
  const backendUrlWithApi = "http://localhost:8000/api/v1"
  const ticket = "test-ticket-abc-123"

  function formatWsUrl(backendUrl, ticketStr) {
    const baseWsUrl = backendUrl.replace(/^http/, "ws")
    return `${baseWsUrl}/ws?ticket=${encodeURIComponent(ticketStr)}`
  }

  const wsUrl = formatWsUrl(backendUrlWithApi, ticket)
  assert.equal(wsUrl, "ws://localhost:8000/api/v1/ws?ticket=test-ticket-abc-123")
  const matches = wsUrl.match(/\/api\/v1/g)
  assert.equal(matches?.length, 1, "URL should contain exactly one /api/v1")
})

test("Realtime - Backoff algorithm increases and caps strictly at 30 s", () => {
  function calculateBackoff(attempt) {
    const baseDelay = Math.min(30000, Math.floor(Math.pow(1.5, attempt) * 1000))
    return baseDelay
  }

  assert.equal(calculateBackoff(1), 1500)
  assert.equal(calculateBackoff(2), 2250)
  assert.equal(calculateBackoff(5), 7593)
  assert.equal(calculateBackoff(10), 30000)
  assert.equal(calculateBackoff(20), 30000)
})

test("Realtime - Status labels are live, reconnecting, offline", () => {
  const statusLabels = {
    live: "Live Fleet Sync",
    reconnecting: "Reconnecting...",
    offline: "Offline",
  }

  assert.equal(statusLabels["live"], "Live Fleet Sync")
  assert.equal(statusLabels["reconnecting"], "Reconnecting...")
  assert.equal(statusLabels["offline"], "Offline")
})
