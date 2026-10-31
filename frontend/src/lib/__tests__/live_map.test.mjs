import test from "node:test"
import assert from "node:assert/strict"
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

import {
  clusterMarkers,
  decodePolyline6,
  etaLabel,
  filterTrucks,
  formatAge,
  progressWidthPct,
  statusPillLabel,
} from "../live/liveUtils.mjs"

const here = path.dirname(fileURLToPath(import.meta.url))
const srcDir = path.resolve(here, "../..")

test("Live - polyline6 decodes the backend route fixture", () => {
  const poly = "gd`{nAn}ncfDfd~Onrbt@~flW~`jnA~flW~hwfB~flW~fayB~flW~dkkC~reK~lxcD~hbE~xqwC_qkAvpprF"
  const pts = decodePolyline6(poly)
  assert.equal(pts.length, 9)
  assert.ok(Math.abs(pts[0][0] - 41.8781) < 1e-4)
  assert.ok(Math.abs(pts[0][1] - -87.6298) < 1e-4)
  assert.ok(Math.abs(pts[8][0] - 39.7392) < 1e-4)
})

test("Live - dropdown lists only trucks on an active trip and filters", () => {
  const vehicles = [
    { vehicle_reg: "VH-1", trip_id: 1, origin: { name: "Chicago Logistics Hub" }, destination: { name: "Denver Intermodal Hub" } },
    { vehicle_reg: "VH-2", trip_id: 2, origin: { name: "Chicago Logistics Hub" }, destination: { name: "Miami Port Hub" } },
  ]
  assert.equal(filterTrucks(vehicles, "").length, 2)
  assert.deepEqual(filterTrucks(vehicles, "vh-2").map((v) => v.vehicle_reg), ["VH-2"])
  assert.deepEqual(filterTrucks(vehicles, "miami").map((v) => v.vehicle_reg), ["VH-2"])
  assert.deepEqual(filterTrucks(vehicles, "  DENVER  ").map((v) => v.vehicle_reg), ["VH-1"])
  assert.equal(filterTrucks(vehicles, "zzz").length, 0)
})

test("Live - progress bar equals API progress_pct", () => {
  assert.equal(progressWidthPct(1.2822979091032896), 1.2822979091032896)
  assert.equal(progressWidthPct(0), 0)
  assert.equal(progressWidthPct(100), 100)
  assert.equal(progressWidthPct(140), 100)
  assert.equal(progressWidthPct(-3), 0)
  assert.equal(progressWidthPct(null), 0)
})

test("Live - source and freshness map to the exact pill labels", () => {
  const pill = (source, ageSeconds, hasPosition = true, paused = false) =>
    statusPillLabel({ source, ageSeconds, hasPosition, paused })
  assert.equal(pill("simulator", 4), "Simulated · updated 4 s ago")
  assert.equal(pill("simulator", 360), "Simulated · last update 6 min ago")
  assert.equal(pill("device", 4), "Live · updated 4 s ago")
  assert.equal(pill("device", 360), "Stale · last update 6 min ago")
  assert.equal(pill("device", 2000), "Stale · last update 33 min ago")
  assert.equal(pill("simulator", null, false), "No position yet")
  assert.equal(pill("device", 4, true, true), "Paused, not updating")
  assert.equal(pill("device", null, false, true), "Paused, not updating")
})

test("Live - age and ETA formatting", () => {
  assert.equal(formatAge(4), "4 s")
  assert.equal(formatAge(360), "6 min")
  assert.equal(etaLabel(10000, 50), "est. 12 min")
  assert.equal(etaLabel(2000, 60), "est. 2 min")
  assert.equal(etaLabel(100000, 50), "est. 2 h")
  assert.equal(etaLabel(500, 0), null)
  assert.equal(etaLabel(null, 50), null)
})

test("Live - markers cluster within 40px", () => {
  const items = [
    { vehicle_reg: "A", latitude: 0, longitude: 0 },
    { vehicle_reg: "B", latitude: 0, longitude: 0.0001 },
    { vehicle_reg: "C", latitude: 5, longitude: 5 },
  ]
  const project = (lat, lon) => ({ x: lon * 1000, y: lat * 1000 })
  const clusters = clusterMarkers(items, project, 40)
  assert.equal(clusters.length, 2)
  const big = clusters.find((c) => c.members.length === 2)
  assert.ok(big)
})

test("Live - forbidden wording is gone from the frontend", () => {
  const banned = ["Estimated position", "Route Studio", "custom route", "estimated_progress"]
  const hits = []
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (entry.name === "__tests__") continue
      const p = path.join(dir, entry.name)
      if (entry.isDirectory()) walk(p)
      else if (/\.(tsx?|mjs|css)$/.test(entry.name)) {
        const text = fs.readFileSync(p, "utf8")
        for (const b of banned) {
          if (text.includes(b)) hits.push(`${path.relative(srcDir, p)}: ${b}`)
        }
      }
    }
  }
  walk(srcDir)
  assert.deepEqual(hits, [])
})
