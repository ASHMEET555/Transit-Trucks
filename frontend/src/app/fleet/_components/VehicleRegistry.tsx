"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Archive,
  CarFront,
  CircleParking,
  Wrench,
} from "lucide-react"
import { RiTruckLine } from "@remixicon/react"
import { Button } from "@/components/Button"
import {
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "@/components/Drawer"
import { Input } from "@/components/Input"
import { Pagination } from "@/components/Pagination"
import { CardShell } from "@/components/fleet/card-shell"
import { StatPills } from "@/components/live-map/stat-pills"
import { RealtimeIndicator } from "@/components/RealtimeIndicator"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import { Skeleton } from "@/components/Skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite, hasRouteAccess } from "@/lib/rbacConfig"
import { useFleetEvents } from "@/lib/realtime/useFleetEvents"
import { CardHead, DotPill } from "./chrome"

type ApiVehicle = {
  registration_number: string
  model: string
  type: string
  max_load_capacity: number
  odometer: number
  acquisition_cost: number
  status: string
  is_active?: boolean
  photo_url?: string | null
  insurance_copy_url?: string | null
  fitness_cert_url?: string | null
  rc_copy_url?: string | null
  puc_cert_url?: string | null
  owner_name?: string | null
  owner_address?: string | null
  owner_contact_number?: string | null
  rto_name?: string | null
  rc_expiry_date?: string | null
  fuel_type?: string | null
  insurance_policy_number?: string | null
  insurance_expiry_date?: string | null
  fitness_expiry_date?: string | null
  pollution_expiry_date?: string | null
}

type PaginatedVehicles = {
  items: ApiVehicle[]
  next_cursor: string | null
}

const STATUS_FILTER_KEY = "fleet-registry:status"
const TYPE_FILTER_KEY = "fleet-registry:type"
const INACTIVE_FILTER_KEY = "fleet-registry:includeInactive"
const SEARCH_KEY = "fleet-registry:search"

const ALL = "All"

function readStored(key: string, fallback: string): string {
  if (typeof window === "undefined") return fallback
  try {
    return window.localStorage.getItem(key) ?? fallback
  } catch {
    return fallback
  }
}

function readStoredBool(key: string, fallback: boolean): boolean {
  if (typeof window === "undefined") return fallback
  try {
    const raw = window.localStorage.getItem(key)
    return raw == null ? fallback : raw === "true"
  } catch {
    return fallback
  }
}

function store(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value)
  } catch {
    // storage unavailable, filters still work for the session
  }
}

async function fetchAllVehicles(
  includeInactive: boolean,
): Promise<ApiVehicle[]> {
  const all: ApiVehicle[] = []
  let cursor: string | null = null
  for (;;) {
    const params = new URLSearchParams({ limit: "100" })
    if (includeInactive) params.set("include_inactive", "true")
    if (cursor) params.set("cursor", cursor)
    const res = await apiRequest<ApiVehicle[] | PaginatedVehicles>(
      `/fleet/vehicles?${params.toString()}`,
    )
    all.push(...extractItems<ApiVehicle>(res))
    const next =
      res && typeof res === "object" && "next_cursor" in res
        ? (res as PaginatedVehicles).next_cursor
        : null
    if (!next) break
    cursor = next
  }
  return all
}

function statusPill(status: string): { dot: string; pill: string } {
  switch (status) {
    case "Available":
      return { dot: "bg-chart-5", pill: "bg-sage text-foreground" }
    case "On Trip":
      return { dot: "bg-amber-600", pill: "bg-sand text-foreground" }
    case "In Shop":
      return { dot: "bg-primary", pill: "bg-mist text-foreground" }
    case "Retired":
      return { dot: "bg-muted-foreground", pill: "bg-muted text-muted-foreground" }
    default:
      return { dot: "bg-muted-foreground", pill: "bg-muted text-muted-foreground" }
  }
}

const emptyForm = {
  registration_number: "",
  model: "",
  type: "Van",
  max_load_capacity: "",
  odometer: "",
  acquisition_cost: "",
  photo_url: "",
  insurance_copy_url: "",
  fitness_cert_url: "",
  rc_copy_url: "",
  puc_cert_url: "",
  owner_name: "",
  owner_address: "",
  owner_contact_number: "",
  rto_name: "",
  rc_expiry_date: "",
  fuel_type: "",
  insurance_policy_number: "",
  insurance_expiry_date: "",
  fitness_expiry_date: "",
  pollution_expiry_date: "",
}

const typeChoices = [
  "Van",
  "Box Truck",
  "Flatbed",
  "Semi",
  "Light Truck",
  "Medium Truck",
  "Heavy Truck",
  "Refrigerated",
]

const statusChoices = ["Available", "On Trip", "In Shop", "Retired"]

const PAGE_SIZE = 10

const fuelTypeChoices = ["Diesel", "Petrol", "CNG", "Electric"]

function toFormVehicle(v: ApiVehicle): typeof emptyForm {
  return {
    registration_number: v.registration_number ?? "",
    model: v.model ?? "",
    type: v.type ?? "Van",
    max_load_capacity: v.max_load_capacity == null ? "" : String(v.max_load_capacity),
    odometer: v.odometer == null ? "" : String(v.odometer),
    acquisition_cost: v.acquisition_cost == null ? "" : String(v.acquisition_cost),
    photo_url: v.photo_url ?? "",
    insurance_copy_url: v.insurance_copy_url ?? "",
    fitness_cert_url: v.fitness_cert_url ?? "",
    rc_copy_url: v.rc_copy_url ?? "",
    puc_cert_url: v.puc_cert_url ?? "",
    owner_name: v.owner_name ?? "",
    owner_address: v.owner_address ?? "",
    owner_contact_number: v.owner_contact_number ?? "",
    rto_name: v.rto_name ?? "",
    rc_expiry_date: v.rc_expiry_date ?? "",
    fuel_type: v.fuel_type ?? "",
    insurance_policy_number: v.insurance_policy_number ?? "",
    insurance_expiry_date: v.insurance_expiry_date ?? "",
    fitness_expiry_date: v.fitness_expiry_date ?? "",
    pollution_expiry_date: v.pollution_expiry_date ?? "",
  }
}

type DocExpiryState = "expired" | "expiring" | "valid"

function docExpiryStateOf(date?: string | null): DocExpiryState {
  if (!date) return "valid"
  const today = new Date().toISOString().slice(0, 10)
  if (date < today) return "expired"
  const horizon = new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10)
  if (date <= horizon) return "expiring"
  return "valid"
}

function ExpiryValue({ date }: { date?: string | null }) {
  if (!date) {
    return <span className="text-muted-foreground">Not recorded</span>
  }
  const state = docExpiryStateOf(date)
  if (state === "valid") {
    return <span className="font-mono tabular-nums">{date}</span>
  }
  const pill =
    state === "expired"
      ? { dot: "bg-destructive", pill: "bg-destructive/12 text-destructive", label: "Expired" }
      : { dot: "bg-amber-600", pill: "bg-sand text-foreground", label: "Expiring soon" }
  return (
    <span className="flex flex-wrap items-center gap-2">
      <span
        className={
          state === "expired"
            ? "font-mono font-semibold tabular-nums text-destructive"
            : "font-mono tabular-nums"
        }
      >
        {date}
      </span>
      <DotPill dot={pill.dot} pill={pill.pill}>
        {pill.label}
      </DotPill>
    </span>
  )
}

function OptionalText({ value }: { value?: string | null }) {
  if (!value) {
    return <span className="text-muted-foreground">Not recorded</span>
  }
  return <span>{value}</span>
}

function DocLink({ label, url }: { label: string; url?: string | null }) {
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="mt-1 font-medium text-foreground">
        {url ? (
          <a
            href={url}
            target="_blank"
            rel="noreferrer"
            className="underline underline-offset-2"
          >
            Open document
          </a>
        ) : (
          <span className="text-muted-foreground">Not recorded</span>
        )}
      </dd>
    </div>
  )
}

export default function VehicleRegistry() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role || ""
  const allowed = !isLoaded || hasRouteAccess(role, "fleet")
  const canManage = canWrite(role, "fleet")

  const [vehicles, setVehicles] = useState<ApiVehicle[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [liveEvent, setLiveEvent] = useState<string | null>(null)

  const [search, setSearch] = useState(() => readStored(SEARCH_KEY, ""))
  const [statusFilter, setStatusFilter] = useState(() =>
    readStored(STATUS_FILTER_KEY, ALL),
  )
  const [typeFilter, setTypeFilter] = useState(() =>
    readStored(TYPE_FILTER_KEY, ALL),
  )
  const [showInactive, setShowInactive] = useState(() =>
    readStoredBool(INACTIVE_FILTER_KEY, false),
  )

  const [showAddForm, setShowAddForm] = useState(false)
  const [editingReg, setEditingReg] = useState<string | null>(null)
  const [formData, setFormData] = useState(emptyForm)
  const [photoInputKey, setPhotoInputKey] = useState(0)
  const [saving, setSaving] = useState(false)
  const [page, setPage] = useState(1)

  const [selectedReg, setSelectedReg] = useState<string | null>(null)
  const [detail, setDetail] = useState<ApiVehicle | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [retireStep, setRetireStep] = useState<"idle" | "confirm" | "working">(
    "idle",
  )

  useEffect(() => store(SEARCH_KEY, search), [search])
  useEffect(() => store(STATUS_FILTER_KEY, statusFilter), [statusFilter])
  useEffect(() => store(TYPE_FILTER_KEY, typeFilter), [typeFilter])
  useEffect(
    () => store(INACTIVE_FILTER_KEY, String(showInactive)),
    [showInactive],
  )

  useEffect(() => {
    setPage(1)
  }, [search, statusFilter, typeFilter, showInactive])

  const load = useCallback(async () => {
    setError(null)
    setLoading(true)
    try {
      const rows = await fetchAllVehicles(showInactive)
      setVehicles(rows)
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.code
            ? `[${err.code}] ${err.detail}`
            : err.detail
          : "Failed to load vehicles",
      )
    } finally {
      setLoading(false)
    }
  }, [showInactive])

  useEffect(() => {
    if (!isLoaded || !hasRouteAccess(role, "fleet")) {
      setLoading(false)
      return
    }
    load()
  }, [isLoaded, role, load])

  const { status: realtimeStatus } = useFleetEvents({
    onEvent: (event) => {
      if (event.type.startsWith("vehicle")) {
        setLiveEvent(event.type)
        load()
      }
    },
    onReconnect: () => {
      load()
    },
  })

  const loadDetail = useCallback(async (reg: string) => {
    setDetail(null)
    setDetailError(null)
    setDetailLoading(true)
    setRetireStep("idle")
    try {
      const row = await apiRequest<ApiVehicle>(
        `/fleet/vehicles/${encodeURIComponent(reg)}`,
      )
      setDetail(row)
    } catch (err) {
      setDetailError(
        err instanceof ApiError ? err.detail : "Failed to load vehicle detail",
      )
    } finally {
      setDetailLoading(false)
    }
  }, [])

  function openDetail(reg: string) {
    setSelectedReg(reg)
    loadDetail(reg)
  }

  function closeDetail() {
    setSelectedReg(null)
    setDetail(null)
    setDetailError(null)
    setRetireStep("idle")
  }

  const typeOptions = useMemo(() => {
    const fromData = Array.from(new Set(vehicles.map((v) => v.type))).sort(
      (a, b) => a.localeCompare(b),
    )
    return Array.from(new Set([...typeChoices, ...fromData]))
  }, [vehicles])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return vehicles.filter((v) => {
      if (statusFilter !== ALL && v.status !== statusFilter) return false
      if (typeFilter !== ALL && v.type !== typeFilter) return false
      if (
        q &&
        !v.registration_number.toLowerCase().includes(q) &&
        !v.model.toLowerCase().includes(q)
      )
        return false
      return true
    })
  }, [vehicles, statusFilter, typeFilter, search])

  const paged = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return filtered.slice(start, start + PAGE_SIZE)
  }, [filtered, page])

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = {
      Available: 0,
      "On Trip": 0,
      "In Shop": 0,
      Retired: 0,
    }
    for (const v of vehicles) {
      if (v.status in counts) counts[v.status] += 1
    }
    return counts
  }, [vehicles])

  function clearFilters() {
    setSearch("")
    setStatusFilter(ALL)
    setTypeFilter(ALL)
    setPage(1)
  }

  function resetForm() {
    setShowAddForm(false)
    setEditingReg(null)
    setFormData(emptyForm)
    setPhotoInputKey((k) => k + 1)
  }

  function openEdit(vehicle: ApiVehicle) {
    setFormData(toFormVehicle(vehicle))
    setEditingReg(vehicle.registration_number)
    setPhotoInputKey((k) => k + 1)
    setShowAddForm(true)
    closeDetail()
  }

  function handlePhotoFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      setFormData((prev) => ({ ...prev, photo_url: String(reader.result ?? "") }))
    }
    reader.readAsDataURL(file)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!canManage || saving) return
    setSaving(true)
    try {
      setError(null)
      const body = {
        registration_number: formData.registration_number.trim(),
        model: formData.model.trim(),
        type: formData.type,
        max_load_capacity: Number(formData.max_load_capacity),
        odometer: Number(formData.odometer),
        acquisition_cost: Number(formData.acquisition_cost),
        photo_url: formData.photo_url.trim(),
        insurance_copy_url: formData.insurance_copy_url.trim(),
        fitness_cert_url: formData.fitness_cert_url.trim(),
        rc_copy_url: formData.rc_copy_url.trim(),
        puc_cert_url: formData.puc_cert_url.trim(),
        owner_name: formData.owner_name.trim(),
        owner_address: formData.owner_address.trim(),
        owner_contact_number: formData.owner_contact_number.trim(),
        rto_name: formData.rto_name.trim(),
        rc_expiry_date: formData.rc_expiry_date,
        fuel_type: formData.fuel_type,
        insurance_policy_number: formData.insurance_policy_number.trim(),
        insurance_expiry_date: formData.insurance_expiry_date,
        fitness_expiry_date: formData.fitness_expiry_date,
        pollution_expiry_date: formData.pollution_expiry_date,
      }
      if (editingReg) {
        await apiRequest(`/fleet/vehicles/${encodeURIComponent(editingReg)}`, {
          method: "PUT",
          body,
        })
      } else {
        await apiRequest("/fleet/vehicles", {
          method: "POST",
          body,
        })
      }
      resetForm()
      await load()
    } catch (err) {
      setError(
        err instanceof ApiError ? err.detail : "Failed to register vehicle",
      )
    } finally {
      setSaving(false)
    }
  }

  async function handleRetire() {
    if (!canManage || !selectedReg || retireStep !== "confirm") return
    setRetireStep("working")
    try {
      setError(null)
      await apiRequest(
        `/fleet/vehicles/${encodeURIComponent(selectedReg)}/retire`,
        { method: "POST" },
      )
      await load()
      await loadDetail(selectedReg)
    } catch (err) {
      setRetireStep("idle")
      setDetailError(
        err instanceof ApiError ? err.detail : "Failed to retire vehicle",
      )
    }
  }

  if (!allowed) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Vehicle registry</h1>
        <CardShell as="section" className="mt-6 items-center p-8 text-center">
          <h2 className="text-lg font-semibold">Access restricted</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Your role does not include fleet access. Ask an admin to update
            your role, then sign in again.
          </p>
          <div className="mt-4">
            <Button variant="secondary" onClick={() => window.location.reload()}>
              Reload page
            </Button>
          </div>
        </CardShell>
      </div>
    )
  }

  const drawerVehicle = detail
  const retireBlockedOnTrip = drawerVehicle?.status === "On Trip"
  const isRetired = drawerVehicle?.status === "Retired"

  return (
    <div className="space-y-4 p-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">Vehicle registry</h1>
            <RealtimeIndicator status={realtimeStatus} />
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Fleet vehicles, status and history
          </p>
        </div>
        {canManage && (
          <Button
            onClick={() => {
              if (showAddForm) {
                resetForm()
              } else {
                setEditingReg(null)
                setFormData(emptyForm)
                setShowAddForm(true)
              }
            }}
          >
            {showAddForm ? "Cancel" : "Add vehicle"}
          </Button>
        )}
      </div>

      <StatPills
        items={[
          { label: "Available", value: statusCounts.Available, icon: CarFront, tone: "sage" },
          { label: "On trip", value: statusCounts["On Trip"], icon: CircleParking, tone: "primary" },
          { label: "In shop", value: statusCounts["In Shop"], icon: Wrench, tone: "mist" },
          { label: "Retired", value: statusCounts.Retired, icon: Archive, tone: "signal" },
        ]}
      />

      {error && (
        <div
          role="alert"
          className="rounded-2xl border border-destructive/30 bg-destructive/12 p-4"
        >
          <p className="text-sm font-medium text-foreground">
            {error}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            Check your connection and fleet permissions, then try again.
          </p>
          <div className="mt-3">
            <Button variant="secondary" onClick={load}>
              Retry
            </Button>
          </div>
        </div>
      )}

      {liveEvent && (
        <div className="rounded-2xl border border-border bg-muted px-4 py-2 text-sm text-muted-foreground">
          Live update: <span className="font-medium text-foreground">{liveEvent}</span>
        </div>
      )}

      {showAddForm && canManage && (
        <CardShell as="section" aria-label={editingReg ? "Edit vehicle" : "Register vehicle"}>
          <CardHead title={editingReg ? `Edit vehicle ${editingReg}` : "Register vehicle"} />
          <form
            onSubmit={handleSubmit}
            className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
          >
            <div>
              <label
                htmlFor="fleet-reg"
                className="mb-2 block text-sm font-medium"
              >
                Plate number *
              </label>
              <Input
                id="fleet-reg"
                required
                disabled={editingReg !== null}
                value={formData.registration_number}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    registration_number: e.target.value,
                  })
                }
                placeholder="e.g. ABC-1234"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-model"
                className="mb-2 block text-sm font-medium"
              >
                Model *
              </label>
              <Input
                id="fleet-model"
                required
                value={formData.model}
                onChange={(e) =>
                  setFormData({ ...formData, model: e.target.value })
                }
                placeholder="e.g. City Bus 12"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-type"
                className="mb-2 block text-sm font-medium"
              >
                Type *
              </label>
              <Select
                value={formData.type}
                onValueChange={(value) =>
                  setFormData({ ...formData, type: value })
                }
              >
                <SelectTrigger id="fleet-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {typeChoices.map((t) => (
                    <SelectItem key={t} value={t}>
                      {t}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label
                htmlFor="fleet-capacity"
                className="mb-2 block text-sm font-medium"
              >
                Max load capacity (kg) *
              </label>
              <Input
                id="fleet-capacity"
                required
                type="number"
                min="0"
                value={formData.max_load_capacity}
                onChange={(e) =>
                  setFormData({ ...formData, max_load_capacity: e.target.value })
                }
                placeholder="e.g. 4500"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-odometer"
                className="mb-2 block text-sm font-medium"
              >
                Odometer (km) *
              </label>
              <Input
                id="fleet-odometer"
                required
                type="number"
                min="0"
                value={formData.odometer}
                onChange={(e) =>
                  setFormData({ ...formData, odometer: e.target.value })
                }
                placeholder="Current reading"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-cost"
                className="mb-2 block text-sm font-medium"
              >
                Acquisition cost *
              </label>
              <Input
                id="fleet-cost"
                required
                type="number"
                min="0"
                step="0.01"
                value={formData.acquisition_cost}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    acquisition_cost: e.target.value,
                  })
                }
                placeholder="Purchase cost"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-fuel-type"
                className="mb-2 block text-sm font-medium"
              >
                Fuel type
              </label>
              <Select
                value={formData.fuel_type}
                onValueChange={(value) =>
                  setFormData({ ...formData, fuel_type: value })
                }
              >
                <SelectTrigger id="fleet-fuel-type">
                  <SelectValue placeholder="Select fuel type" />
                </SelectTrigger>
                <SelectContent>
                  {fuelTypeChoices.map((f) => (
                    <SelectItem key={f} value={f}>
                      {f}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label
                htmlFor="fleet-owner-name"
                className="mb-2 block text-sm font-medium"
              >
                Owner name
              </label>
              <Input
                id="fleet-owner-name"
                value={formData.owner_name}
                onChange={(e) =>
                  setFormData({ ...formData, owner_name: e.target.value })
                }
                placeholder="Registered owner"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-owner-contact"
                className="mb-2 block text-sm font-medium"
              >
                Owner contact number
              </label>
              <Input
                id="fleet-owner-contact"
                value={formData.owner_contact_number}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    owner_contact_number: e.target.value,
                  })
                }
                placeholder="e.g. +911234567890"
              />
            </div>
            <div className="md:col-span-2 lg:col-span-3">
              <label
                htmlFor="fleet-owner-address"
                className="mb-2 block text-sm font-medium"
              >
                Owner address
              </label>
              <Input
                id="fleet-owner-address"
                value={formData.owner_address}
                onChange={(e) =>
                  setFormData({ ...formData, owner_address: e.target.value })
                }
                placeholder="Owner postal address"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-rto"
                className="mb-2 block text-sm font-medium"
              >
                RTO name
              </label>
              <Input
                id="fleet-rto"
                value={formData.rto_name}
                onChange={(e) =>
                  setFormData({ ...formData, rto_name: e.target.value })
                }
                placeholder="Registering authority"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-policy-number"
                className="mb-2 block text-sm font-medium"
              >
                Insurance policy number
              </label>
              <Input
                id="fleet-policy-number"
                value={formData.insurance_policy_number}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    insurance_policy_number: e.target.value,
                  })
                }
                placeholder="Policy number"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-rc-expiry"
                className="mb-2 block text-sm font-medium"
              >
                RC expiry date
              </label>
              <Input
                id="fleet-rc-expiry"
                type="date"
                value={formData.rc_expiry_date}
                onChange={(e) =>
                  setFormData({ ...formData, rc_expiry_date: e.target.value })
                }
              />
            </div>
            <div>
              <label
                htmlFor="fleet-insurance-expiry"
                className="mb-2 block text-sm font-medium"
              >
                Insurance expiry date
              </label>
              <Input
                id="fleet-insurance-expiry"
                type="date"
                value={formData.insurance_expiry_date}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    insurance_expiry_date: e.target.value,
                  })
                }
              />
            </div>
            <div>
              <label
                htmlFor="fleet-fitness-expiry"
                className="mb-2 block text-sm font-medium"
              >
                Fitness expiry date
              </label>
              <Input
                id="fleet-fitness-expiry"
                type="date"
                value={formData.fitness_expiry_date}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    fitness_expiry_date: e.target.value,
                  })
                }
              />
            </div>
            <div>
              <label
                htmlFor="fleet-pollution-expiry"
                className="mb-2 block text-sm font-medium"
              >
                Pollution expiry date
              </label>
              <Input
                id="fleet-pollution-expiry"
                type="date"
                value={formData.pollution_expiry_date}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    pollution_expiry_date: e.target.value,
                  })
                }
              />
            </div>
            <div className="md:col-span-2 lg:col-span-3">
              <span
                id="fleet-photo-label"
                className="mb-2 block text-sm font-medium"
              >
                Photo
              </span>
              <div className="flex flex-col gap-2">
                <input
                  key={photoInputKey}
                  type="file"
                  accept="image/*"
                  onChange={handlePhotoFile}
                  aria-labelledby="fleet-photo-label"
                  className="text-sm text-foreground file:mr-3 file:rounded-full file:border file:border-border file:bg-muted file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                />
                <Input
                  id="fleet-photo-url"
                  value={formData.photo_url}
                  onChange={(e) =>
                    setFormData({ ...formData, photo_url: e.target.value })
                  }
                  placeholder="Or paste a photo URL"
                  aria-label="Photo URL"
                />
                {formData.photo_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- user-supplied URL or data-URL preview, not optimizable
                  <img
                    src={formData.photo_url}
                    alt="Vehicle photo preview"
                    className="h-28 w-full rounded-2xl object-cover"
                  />
                ) : null}
              </div>
            </div>
            <div>
              <label
                htmlFor="fleet-insurance-copy"
                className="mb-2 block text-sm font-medium"
              >
                Insurance copy URL
              </label>
              <Input
                id="fleet-insurance-copy"
                type="url"
                value={formData.insurance_copy_url}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    insurance_copy_url: e.target.value,
                  })
                }
                placeholder="https://…"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-fitness-cert"
                className="mb-2 block text-sm font-medium"
              >
                Fitness certificate URL
              </label>
              <Input
                id="fleet-fitness-cert"
                type="url"
                value={formData.fitness_cert_url}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    fitness_cert_url: e.target.value,
                  })
                }
                placeholder="https://…"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-rc-copy"
                className="mb-2 block text-sm font-medium"
              >
                RC copy URL
              </label>
              <Input
                id="fleet-rc-copy"
                type="url"
                value={formData.rc_copy_url}
                onChange={(e) =>
                  setFormData({ ...formData, rc_copy_url: e.target.value })
                }
                placeholder="https://…"
              />
            </div>
            <div>
              <label
                htmlFor="fleet-puc-cert"
                className="mb-2 block text-sm font-medium"
              >
                PUC certificate URL
              </label>
              <Input
                id="fleet-puc-cert"
                type="url"
                value={formData.puc_cert_url}
                onChange={(e) =>
                  setFormData({ ...formData, puc_cert_url: e.target.value })
                }
                placeholder="https://…"
              />
            </div>
            <div className="flex items-end gap-2 md:col-span-2 lg:col-span-3">
              <Button type="submit" isLoading={saving} loadingText="Saving">
                {editingReg ? "Save changes" : "Register vehicle"}
              </Button>
              <Button
                type="button"
                variant="secondary"
                onClick={resetForm}
              >
                Cancel
              </Button>
            </div>
          </form>
        </CardShell>
      )}

      <div className="flex flex-wrap items-end gap-4 rounded-2xl border border-border bg-muted p-3 sm:p-4">
        <div className="min-w-[200px] flex-1">
          <label
            htmlFor="fleet-search"
            className="mb-2 block text-sm font-medium"
          >
            Search
          </label>
          <Input
            id="fleet-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Plate or model"
          />
        </div>
        <div className="min-w-[160px] flex-1">
          <label
            htmlFor="fleet-status-filter"
            className="mb-2 block text-sm font-medium"
          >
            Status
          </label>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger id="fleet-status-filter">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>All statuses</SelectItem>
              {statusChoices.map((s) => (
                <SelectItem key={s} value={s}>
                  {s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="min-w-[160px] flex-1">
          <label
            htmlFor="fleet-type-filter"
            className="mb-2 block text-sm font-medium"
          >
            Type
          </label>
          <Select value={typeFilter} onValueChange={setTypeFilter}>
            <SelectTrigger id="fleet-type-filter">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>All types</SelectItem>
              {typeOptions.map((t) => (
                <SelectItem key={t} value={t}>
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2 pb-2">
          <input
            id="fleet-inactive"
            type="checkbox"
            checked={showInactive}
            onChange={(e) => setShowInactive(e.target.checked)}
            className="size-4 rounded border-border focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          />
          <label htmlFor="fleet-inactive" className="text-sm font-medium">
            Include inactive
          </label>
        </div>
        <div className="pb-1">
          <Button variant="secondary" onClick={clearFilters}>
            Clear filters
          </Button>
        </div>
      </div>

      <CardShell as="section" aria-label="Vehicle registry">
        <CardHead
          title="Vehicles"
          meta={
            loading
              ? "Loading…"
              : `${filtered.length} of ${vehicles.length} vehicles`
          }
        />
        <div className="mt-4 overflow-x-auto">
          <Table>
            <TableHead>
              <TableRow>
                <TableHeaderCell>Plate</TableHeaderCell>
                <TableHeaderCell>Model</TableHeaderCell>
                <TableHeaderCell>Type</TableHeaderCell>
                <TableHeaderCell>Capacity (kg)</TableHeaderCell>
                <TableHeaderCell>Odometer (km)</TableHeaderCell>
                <TableHeaderCell>Cost</TableHeaderCell>
                <TableHeaderCell>Status</TableHeaderCell>
                <TableHeaderCell>
                  <span className="sr-only">Row actions</span>
                </TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell colSpan={8}>
                      <Skeleton className="h-5 w-full motion-reduce:animate-none" />
                    </TableCell>
                  </TableRow>
                ))
              ) : filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center">
                    <div className="flex flex-col items-center gap-3 py-8">
                      <RiTruckLine
                        className="size-8 text-muted-foreground"
                        aria-hidden="true"
                      />
                      <div>
                        <p className="font-medium text-foreground">
                          {vehicles.length === 0
                            ? "No vehicles registered yet"
                            : "No vehicles match these filters"}
                        </p>
                        <p className="mt-1 text-sm text-muted-foreground">
                          {vehicles.length === 0
                            ? "Register the first vehicle to start the fleet record."
                            : "Try a different search or clear the filters."}
                        </p>
                      </div>
                      {vehicles.length === 0 ? (
                        canManage && (
                          <Button
                            onClick={() => {
                              setEditingReg(null)
                              setFormData(emptyForm)
                              setShowAddForm(true)
                            }}
                          >
                            Add vehicle
                          </Button>
                        )
                      ) : (
                        <Button variant="secondary" onClick={clearFilters}>
                          Clear filters
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                paged.map((v) => {
                  const pill = statusPill(v.status)
                  return (
                    <TableRow key={v.registration_number}>
                      <TableCell className="font-serif font-medium text-foreground">
                        {v.registration_number}
                      </TableCell>
                      <TableCell>{v.model}</TableCell>
                      <TableCell>{v.type}</TableCell>
                      <TableCell className="tabular-nums">
                        {Number(v.max_load_capacity).toLocaleString()}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        {Number(v.odometer).toLocaleString()}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        ${Number(v.acquisition_cost).toLocaleString()}
                      </TableCell>
                      <TableCell>
                        <DotPill dot={pill.dot} pill={pill.pill}>
                          {v.status}
                        </DotPill>
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="secondary"
                          onClick={() => openDetail(v.registration_number)}
                          aria-label={`View details for ${v.registration_number}`}
                        >
                          Details
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>
        <Pagination
          page={page}
          pageSize={PAGE_SIZE}
          total={filtered.length}
          onPageChange={setPage}
          id="fleet-registry-pagination"
        />
      </CardShell>

      <Drawer
        open={selectedReg !== null}
        onOpenChange={(open) => {
          if (!open) closeDetail()
        }}
      >
        <DrawerContent aria-label="Vehicle detail">
          <DrawerHeader>
            <DrawerTitle className="font-serif">
              {selectedReg ?? "Vehicle detail"}
            </DrawerTitle>
          </DrawerHeader>
          <DrawerDescription>
            Registration record, current status and retirement.
          </DrawerDescription>
          <DrawerBody>
            {detailLoading && (
              <div className="flex flex-col gap-3">
                <Skeleton className="h-5 w-2/3 motion-reduce:animate-none" />
                <Skeleton className="h-5 w-full motion-reduce:animate-none" />
                <Skeleton className="h-5 w-1/2 motion-reduce:animate-none" />
              </div>
            )}
            {detailError && (
              <div
                role="alert"
                className="rounded-2xl border border-destructive/30 bg-destructive/12 p-4 text-sm text-foreground"
              >
                <p>{detailError}</p>
                <div className="mt-3">
                  <Button
                    variant="secondary"
                    onClick={() => selectedReg && loadDetail(selectedReg)}
                  >
                    Retry
                  </Button>
                </div>
              </div>
            )}
            {drawerVehicle && (
              <div className="flex flex-col gap-5">
                <div>
                  {(() => {
                    const pill = statusPill(drawerVehicle.status)
                    return (
                      <DotPill dot={pill.dot} pill={pill.pill}>
                        {drawerVehicle.status}
                      </DotPill>
                    )
                  })()}
                </div>
                <dl className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <dt className="text-muted-foreground">Model</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      {drawerVehicle.model}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Type</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      {drawerVehicle.type}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Max load capacity</dt>
                    <dd className="mt-1 font-medium tabular-nums text-foreground">
                      {Number(drawerVehicle.max_load_capacity).toLocaleString()}{" "}
                      kg
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Odometer</dt>
                    <dd className="mt-1 font-medium tabular-nums text-foreground">
                      {Number(drawerVehicle.odometer).toLocaleString()} km
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Acquisition cost</dt>
                    <dd className="mt-1 font-medium tabular-nums text-foreground">
                      ${Number(drawerVehicle.acquisition_cost).toLocaleString()}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Record</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      {drawerVehicle.is_active === false
                        ? "Inactive"
                        : "Active"}
                    </dd>
                  </div>
                </dl>

                {drawerVehicle.photo_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- user-supplied URL or data-URL, not optimizable
                  <img
                    src={drawerVehicle.photo_url}
                    alt={`Photo of vehicle ${drawerVehicle.registration_number}`}
                    className="h-44 w-full rounded-2xl object-cover"
                  />
                ) : null}
                <dl className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <dt className="text-muted-foreground">Fuel type</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <OptionalText value={drawerVehicle.fuel_type} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">RTO name</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <OptionalText value={drawerVehicle.rto_name} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Owner name</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <OptionalText value={drawerVehicle.owner_name} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Owner contact</dt>
                    <dd className="mt-1 font-medium tabular-nums text-foreground">
                      <OptionalText value={drawerVehicle.owner_contact_number} />
                    </dd>
                  </div>
                  <div className="col-span-2">
                    <dt className="text-muted-foreground">Owner address</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <OptionalText value={drawerVehicle.owner_address} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Insurance policy</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <OptionalText
                        value={drawerVehicle.insurance_policy_number}
                      />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">RC expiry</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <ExpiryValue date={drawerVehicle.rc_expiry_date} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Insurance expiry</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <ExpiryValue date={drawerVehicle.insurance_expiry_date} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Fitness expiry</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <ExpiryValue date={drawerVehicle.fitness_expiry_date} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">Pollution expiry</dt>
                    <dd className="mt-1 font-medium text-foreground">
                      <ExpiryValue date={drawerVehicle.pollution_expiry_date} />
                    </dd>
                  </div>
                  <DocLink
                    label="Insurance copy"
                    url={drawerVehicle.insurance_copy_url}
                  />
                  <DocLink
                    label="Fitness certificate"
                    url={drawerVehicle.fitness_cert_url}
                  />
                  <DocLink label="RC copy" url={drawerVehicle.rc_copy_url} />
                  <DocLink
                    label="PUC certificate"
                    url={drawerVehicle.puc_cert_url}
                  />
                </dl>

                <div className="rounded-2xl border border-border bg-muted p-4">
                  <h3 className="font-semibold">Retirement</h3>
                  {isRetired ? (
                    <p className="mt-2 text-sm text-muted-foreground">
                      This vehicle is retired. Retired vehicles stay in history
                      and remain visible with the inactive filter.
                    </p>
                  ) : retireBlockedOnTrip ? (
                    <p className="mt-2 text-sm text-muted-foreground">
                      This vehicle is on a trip and cannot be retired until the
                      trip ends.
                    </p>
                  ) : !canManage ? (
                    <p className="mt-2 text-sm text-muted-foreground">
                      Only fleet managers and admins can retire vehicles.
                    </p>
                  ) : retireStep === "confirm" || retireStep === "working" ? (
                    <div>
                      <p className="mt-2 text-sm text-muted-foreground">
                        Retire {drawerVehicle.registration_number}? It leaves
                        active service but stays in history.
                      </p>
                      <div className="mt-3 flex gap-2">
                        <Button
                          variant="destructive"
                          onClick={handleRetire}
                          isLoading={retireStep === "working"}
                          loadingText="Retiring"
                        >
                          Confirm retirement
                        </Button>
                        <Button
                          variant="secondary"
                          onClick={() => setRetireStep("idle")}
                          disabled={retireStep === "working"}
                        >
                          Keep vehicle
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div>
                      <p className="mt-2 text-sm text-muted-foreground">
                        Retirement is the only way to end service. Retired
                        vehicles stay in history.
                      </p>
                      <div className="mt-3">
                        <Button
                          variant="destructive"
                          onClick={() => setRetireStep("confirm")}
                        >
                          Retire vehicle
                        </Button>
                      </div>
                    </div>
                  )}
                </div>

                <div className="flex justify-end gap-2">
                  {canManage && !isRetired && (
                    <Button onClick={() => openEdit(drawerVehicle)}>
                      Edit vehicle
                    </Button>
                  )}
                  <Button variant="secondary" onClick={closeDetail}>
                    Close
                  </Button>
                </div>
              </div>
            )}
          </DrawerBody>
        </DrawerContent>
      </Drawer>

      <p className="text-sm text-muted-foreground">
        Filters are saved on this device.
      </p>
    </div>
  )
}
