"use client"

import { Building2, Plus } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
import { Pagination } from "@/components/Pagination"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/Select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import {
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/Drawer"
import { CardShell } from "@/components/fleet/card-shell"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite } from "@/lib/rbacConfig"
import {
  EmptyNote,
  ForbiddenBlock,
  LoadingBlock,
  SectionHeading,
} from "../dashboard/_components/states"

const STORAGE_KEY = "transit.clients.v1"
const PAGE_SIZE = 8

const CLIENT_TYPES = [
  "Customer",
  "Consignee",
  "Consignor",
  "Consignor & Consignee",
] as const

type ClientType = (typeof CLIENT_TYPES)[number]

interface Ledger {
  name: string
  code: string
  group: string
  tallyAccount: string
  openingBalance: string
  openingBalanceDate: string
  creditDays: string
  creditLimit: string
  branches: string
  bankAccounts: string
}

interface Client {
  id: string
  name: string
  code: string
  type: ClientType
  address: string
  city: string
  state: string
  pin: string
  email: string
  mobile: string
  gst: string
  panEntity: string
  pan: string
  branches: string
  ledger: string
  tds: string
  clientTo: string
  businessVerticals: string
  pointBasedCN: boolean
  defaultBranch: boolean
  customFields: string
  ledgers: Ledger[]
}

const EMPTY_LEDGER: Ledger = {
  name: "",
  code: "",
  group: "",
  tallyAccount: "",
  openingBalance: "",
  openingBalanceDate: "",
  creditDays: "",
  creditLimit: "",
  branches: "",
  bankAccounts: "",
}

const EMPTY_FORM: Omit<Client, "id" | "ledgers"> = {
  name: "",
  code: "",
  type: "Customer",
  address: "",
  city: "",
  state: "",
  pin: "",
  email: "",
  mobile: "",
  gst: "",
  panEntity: "",
  pan: "",
  branches: "",
  ledger: "",
  tds: "",
  clientTo: "",
  businessVerticals: "",
  pointBasedCN: false,
  defaultBranch: false,
  customFields: "",
}

const SEED_CLIENTS: Client[] = [
  {
    id: "seed-acme",
    name: "Acme Traders",
    code: "CL-1001",
    type: "Customer",
    address: "14 Market Road",
    city: "Mumbai",
    state: "Maharashtra",
    pin: "400001",
    email: "billing@acmetraders.in",
    mobile: "9820012345",
    gst: "27ABCDE1234F1Z5",
    panEntity: "Private Limited",
    pan: "ABCDE1234F",
    branches: "Mumbai, Pune",
    ledger: "Acme Receivable",
    tds: "194C",
    clientTo: "Transit Operations",
    businessVerticals: "Retail, Distribution",
    pointBasedCN: false,
    defaultBranch: true,
    customFields: "",
    ledgers: [],
  },
  {
    id: "seed-sharma",
    name: "Sharma Logistics",
    code: "CL-1002",
    type: "Consignor & Consignee",
    address: "8 Transport Nagar",
    city: "Delhi",
    state: "Delhi",
    pin: "110042",
    email: "ops@sharmalogistics.in",
    mobile: "9811087654",
    gst: "07ABCDE5678G1Z2",
    panEntity: "Partnership",
    pan: "ABCDE5678G",
    branches: "Delhi",
    ledger: "Sharma Receivable",
    tds: "194C",
    clientTo: "Transit Operations",
    businessVerticals: "FMCG",
    pointBasedCN: true,
    defaultBranch: false,
    customFields: "",
    ledgers: [],
  },
]

function loadClients(): Client[] {
  if (typeof window === "undefined") return SEED_CLIENTS
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(SEED_CLIENTS))
      return SEED_CLIENTS
    }
    const parsed = JSON.parse(raw) as Client[]
    return Array.isArray(parsed) ? parsed : SEED_CLIENTS
  } catch {
    return SEED_CLIENTS
  }
}

function Field({
  label,
  children,
  htmlFor,
}: {
  label: string
  children: React.ReactNode
  htmlFor?: string
}) {
  return (
    <div>
      <label
        htmlFor={htmlFor}
        className="mb-1 block text-sm font-medium text-foreground"
      >
        {label}
      </label>
      {children}
    </div>
  )
}

export default function ClientsPage() {
  const { currentProfile, isLoaded } = useProfile()
  const role = currentProfile?.role ?? ""
  const pageAllowed = role === "Admin" || role === "Fleet Manager"
  const writer = canWrite(role, "trips") && pageAllowed

  const [clients, setClients] = useState<Client[] | null>(null)
  const [search, setSearch] = useState("")
  const [typeFilter, setTypeFilter] = useState("All")
  const [page, setPage] = useState(1)

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [form, setForm] = useState(EMPTY_FORM)
  const [ledgers, setLedgers] = useState<Ledger[]>([])
  const [ledgerOpen, setLedgerOpen] = useState(false)
  const [ledgerForm, setLedgerForm] = useState<Ledger>(EMPTY_LEDGER)
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    setClients(loadClients())
  }, [])

  useEffect(() => {
    setPage(1)
  }, [search, typeFilter])

  function persist(next: Client[]) {
    setClients(next)
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
    } catch {
      // storage full or unavailable; keep in-memory copy
    }
  }

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase()
    return (clients ?? []).filter((client) => {
      if (typeFilter !== "All" && client.type !== typeFilter) return false
      if (!query) return true
      return (
        client.name.toLowerCase().includes(query) ||
        client.code.toLowerCase().includes(query) ||
        client.city.toLowerCase().includes(query)
      )
    })
  }, [clients, search, typeFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const safePage = Math.min(page, totalPages)
  const pageRows = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)

  function set<K extends keyof typeof form>(key: K, value: (typeof form)[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function setLedger<K extends keyof Ledger>(key: K, value: Ledger[K]) {
    setLedgerForm((prev) => ({ ...prev, [key]: value }))
  }

  function openDrawer() {
    setForm(EMPTY_FORM)
    setLedgers([])
    setLedgerForm(EMPTY_LEDGER)
    setFormError(null)
    setDrawerOpen(true)
  }

  function saveLedger() {
    if (!ledgerForm.name.trim() || !ledgerForm.code.trim()) {
      setFormError("Ledger needs a name and a code.")
      return
    }
    setLedgers((prev) => [...prev, ledgerForm])
    setForm((prev) => ({ ...prev, ledger: ledgerForm.name }))
    setLedgerForm(EMPTY_LEDGER)
    setLedgerOpen(false)
    setFormError(null)
  }

  function saveClient(event: React.FormEvent) {
    event.preventDefault()
    if (!form.name.trim() || !form.code.trim()) {
      setFormError("Client needs a name and a code.")
      return
    }
    const client: Client = {
      ...form,
      id:
        typeof crypto !== "undefined" && "randomUUID" in crypto
          ? crypto.randomUUID()
          : `client-${Date.now()}`,
      ledgers,
    }
    persist([client, ...(clients ?? [])])
    setDrawerOpen(false)
  }

  function removeClient(id: string) {
    persist((clients ?? []).filter((client) => client.id !== id))
  }

  if (!isLoaded || clients === null) {
    return (
      <div className="space-y-6 p-6 lg:p-8" aria-busy="true">
        <LoadingBlock label="Loading clients" />
      </div>
    )
  }

  if (!pageAllowed) {
    return (
      <div className="p-6 lg:p-8">
        <ForbiddenBlock message="Clients are limited to Admin and Fleet Manager roles." />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-sm text-muted-foreground">Customer master</p>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">Clients</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Stored locally until the Clients API lands.
          </p>
        </div>
        {writer && (
          <Button onClick={openDrawer}>
            <Plus className="size-4" aria-hidden="true" />
            Add client
          </Button>
        )}
      </div>

      <CardShell
        as="section"
        aria-label="Clients"
        tone="paper"
        className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
      >
        <SectionHeading
          icon={Building2}
          title="Clients"
          description={`${filtered.length} ${filtered.length === 1 ? "client" : "clients"}`}
        />
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_220px]">
          <Input
            type="search"
            aria-label="Search clients"
            placeholder="Search by name, code or city"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <Select value={typeFilter} onValueChange={setTypeFilter}>
            <SelectTrigger aria-label="Filter by type">
              <SelectValue placeholder="All types" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All types</SelectItem>
              {CLIENT_TYPES.map((type) => (
                <SelectItem key={type} value={type}>
                  {type}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="mt-4 overflow-x-auto">
          <Table aria-label="Clients">
            <TableHead>
              <TableRow>
                <TableHeaderCell scope="col">Name</TableHeaderCell>
                <TableHeaderCell scope="col">Code</TableHeaderCell>
                <TableHeaderCell scope="col">Type</TableHeaderCell>
                <TableHeaderCell scope="col">Address</TableHeaderCell>
                <TableHeaderCell scope="col">
                  <span className="sr-only">Row actions</span>
                  Actions
                </TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {pageRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5}>
                    {writer ? (
                      <div className="flex flex-col items-center gap-2 py-10 text-center">
                        <p className="text-sm font-medium text-foreground">
                          No clients found
                        </p>
                        <p className="max-w-sm text-sm text-muted-foreground">
                          Try a different search, or add the first client.
                        </p>
                        <Button variant="secondary" onClick={openDrawer}>
                          Add client
                        </Button>
                      </div>
                    ) : (
                      <EmptyNote message="No clients match the current filters." />
                    )}
                  </TableCell>
                </TableRow>
              ) : (
                pageRows.map((client) => (
                  <TableRow key={client.id}>
                    <TableCell className="font-medium text-foreground">
                      {client.name}
                    </TableCell>
                    <TableCell className="font-mono tabular-nums">
                      {client.code}
                    </TableCell>
                    <TableCell>{client.type}</TableCell>
                    <TableCell>
                      {[client.address, client.city, client.state, client.pin]
                        .filter(Boolean)
                        .join(", ") || "—"}
                    </TableCell>
                    <TableCell>
                      {writer && !client.id.startsWith("seed-") ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => removeClient(client.id)}
                          aria-label={`Remove ${client.name}`}
                        >
                          Remove
                        </Button>
                      ) : (
                        <span className="text-xs text-muted-foreground">—</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <Pagination
          page={safePage}
          pageSize={PAGE_SIZE}
          total={filtered.length}
          onPageChange={setPage}
        />
      </CardShell>

      <Drawer open={drawerOpen} onOpenChange={setDrawerOpen}>
        <DrawerContent aria-label="Add client">
          <DrawerHeader>
            <DrawerTitle>Add client</DrawerTitle>
            <DrawerDescription>
              Stored locally until the Clients API lands.
            </DrawerDescription>
          </DrawerHeader>
          <form onSubmit={saveClient}>
            <DrawerBody>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field label="Name" htmlFor="client-name">
                  <Input
                    id="client-name"
                    value={form.name}
                    onChange={(event) => set("name", event.target.value)}
                    required
                    autoComplete="off"
                  />
                </Field>
                <Field label="Code" htmlFor="client-code">
                  <Input
                    id="client-code"
                    value={form.code}
                    onChange={(event) => set("code", event.target.value)}
                    required
                    autoComplete="off"
                  />
                </Field>
                <Field label="Type" htmlFor="client-type">
                  <Select
                    value={form.type}
                    onValueChange={(value) => set("type", value as ClientType)}
                  >
                    <SelectTrigger id="client-type">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {CLIENT_TYPES.map((type) => (
                        <SelectItem key={type} value={type}>
                          {type}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
                <Field label="Email" htmlFor="client-email">
                  <Input
                    id="client-email"
                    type="email"
                    value={form.email}
                    onChange={(event) => set("email", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <div className="sm:col-span-2">
                  <Field label="Address" htmlFor="client-address">
                    <Input
                      id="client-address"
                      value={form.address}
                      onChange={(event) => set("address", event.target.value)}
                      autoComplete="off"
                    />
                  </Field>
                </div>
                <Field label="City" htmlFor="client-city">
                  <Input
                    id="client-city"
                    value={form.city}
                    onChange={(event) => set("city", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="State" htmlFor="client-state">
                  <Input
                    id="client-state"
                    value={form.state}
                    onChange={(event) => set("state", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="Pin" htmlFor="client-pin">
                  <Input
                    id="client-pin"
                    value={form.pin}
                    onChange={(event) => set("pin", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="Mobile" htmlFor="client-mobile">
                  <Input
                    id="client-mobile"
                    type="tel"
                    value={form.mobile}
                    onChange={(event) => set("mobile", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="GST number" htmlFor="client-gst">
                  <Input
                    id="client-gst"
                    value={form.gst}
                    onChange={(event) => set("gst", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="PAN" htmlFor="client-pan">
                  <Input
                    id="client-pan"
                    value={form.pan}
                    onChange={(event) => set("pan", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="PAN entity" htmlFor="client-pan-entity">
                  <Input
                    id="client-pan-entity"
                    value={form.panEntity}
                    onChange={(event) => set("panEntity", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="Branches" htmlFor="client-branches">
                  <Input
                    id="client-branches"
                    value={form.branches}
                    onChange={(event) => set("branches", event.target.value)}
                    placeholder="Comma separated"
                    autoComplete="off"
                  />
                </Field>
                <Field label="Ledger" htmlFor="client-ledger">
                  <div className="flex gap-2">
                    <Input
                      id="client-ledger"
                      value={form.ledger}
                      onChange={(event) => set("ledger", event.target.value)}
                      autoComplete="off"
                    />
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setLedgerOpen(true)}
                    >
                      New ledger
                    </Button>
                  </div>
                </Field>
                <Field label="TDS master" htmlFor="client-tds">
                  <Input
                    id="client-tds"
                    value={form.tds}
                    onChange={(event) => set("tds", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="Client to" htmlFor="client-to">
                  <Input
                    id="client-to"
                    value={form.clientTo}
                    onChange={(event) => set("clientTo", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <Field label="Business verticals" htmlFor="client-verticals">
                  <Input
                    id="client-verticals"
                    value={form.businessVerticals}
                    onChange={(event) => set("businessVerticals", event.target.value)}
                    placeholder="Comma separated"
                    autoComplete="off"
                  />
                </Field>
                <Field label="Custom fields" htmlFor="client-custom">
                  <Input
                    id="client-custom"
                    value={form.customFields}
                    onChange={(event) => set("customFields", event.target.value)}
                    autoComplete="off"
                  />
                </Field>
                <div className="flex flex-wrap gap-4 sm:col-span-2">
                  <label className="flex items-center gap-2 text-sm text-foreground">
                    <input
                      type="checkbox"
                      checked={form.pointBasedCN}
                      onChange={(event) => set("pointBasedCN", event.target.checked)}
                      className="size-4 accent-primary"
                    />
                    Point based CN
                  </label>
                  <label className="flex items-center gap-2 text-sm text-foreground">
                    <input
                      type="checkbox"
                      checked={form.defaultBranch}
                      onChange={(event) => set("defaultBranch", event.target.checked)}
                      className="size-4 accent-primary"
                    />
                    Default branch
                  </label>
                </div>
                {ledgers.length > 0 && (
                  <p className="text-sm text-muted-foreground sm:col-span-2" role="status">
                    {ledgers.length} {ledgers.length === 1 ? "ledger" : "ledgers"} attached.
                  </p>
                )}
              </div>
              {formError && (
                <p role="alert" className="mt-4 text-sm text-destructive">
                  {formError}
                </p>
              )}
            </DrawerBody>
            <DrawerFooter>
              <Button type="button" variant="ghost" onClick={() => setDrawerOpen(false)}>
                Cancel
              </Button>
              <Button type="submit">Save client</Button>
            </DrawerFooter>
          </form>
        </DrawerContent>
      </Drawer>

      <Drawer open={ledgerOpen} onOpenChange={setLedgerOpen}>
        <DrawerContent aria-label="Create new ledger">
          <DrawerHeader>
            <DrawerTitle>Create new ledger</DrawerTitle>
            <DrawerDescription>
              Attached to the client being added.
            </DrawerDescription>
          </DrawerHeader>
          <DrawerBody>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Ledger name" htmlFor="ledger-name">
                <Input
                  id="ledger-name"
                  value={ledgerForm.name}
                  onChange={(event) => setLedger("name", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Ledger code" htmlFor="ledger-code">
                <Input
                  id="ledger-code"
                  value={ledgerForm.code}
                  onChange={(event) => setLedger("code", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Under group" htmlFor="ledger-group">
                <Input
                  id="ledger-group"
                  value={ledgerForm.group}
                  onChange={(event) => setLedger("group", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Tally account" htmlFor="ledger-tally">
                <Input
                  id="ledger-tally"
                  value={ledgerForm.tallyAccount}
                  onChange={(event) => setLedger("tallyAccount", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Opening balance" htmlFor="ledger-opening">
                <Input
                  id="ledger-opening"
                  inputMode="decimal"
                  value={ledgerForm.openingBalance}
                  onChange={(event) => setLedger("openingBalance", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Opening balance date" htmlFor="ledger-opening-date">
                <Input
                  id="ledger-opening-date"
                  type="date"
                  value={ledgerForm.openingBalanceDate}
                  onChange={(event) => setLedger("openingBalanceDate", event.target.value)}
                />
              </Field>
              <Field label="Credit days" htmlFor="ledger-days">
                <Input
                  id="ledger-days"
                  inputMode="numeric"
                  value={ledgerForm.creditDays}
                  onChange={(event) => setLedger("creditDays", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Credit limit" htmlFor="ledger-limit">
                <Input
                  id="ledger-limit"
                  inputMode="decimal"
                  value={ledgerForm.creditLimit}
                  onChange={(event) => setLedger("creditLimit", event.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Branches" htmlFor="ledger-branches">
                <Input
                  id="ledger-branches"
                  value={ledgerForm.branches}
                  onChange={(event) => setLedger("branches", event.target.value)}
                  placeholder="Comma separated"
                  autoComplete="off"
                />
              </Field>
              <Field label="Bank accounts" htmlFor="ledger-banks">
                <Input
                  id="ledger-banks"
                  value={ledgerForm.bankAccounts}
                  onChange={(event) => setLedger("bankAccounts", event.target.value)}
                  placeholder="One per line"
                  autoComplete="off"
                />
              </Field>
            </div>
          </DrawerBody>
          <DrawerFooter>
            <Button type="button" variant="ghost" onClick={() => setLedgerOpen(false)}>
              Cancel
            </Button>
            <Button type="button" onClick={saveLedger}>
              Save ledger
            </Button>
          </DrawerFooter>
        </DrawerContent>
      </Drawer>
    </div>
  )
}
