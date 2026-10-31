"use client"

import { useEffect, useState } from "react"
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
import { ApiError, apiRequest } from "@/lib/api"
import { memoryGet, memorySet } from "@/lib/memoryStore"
import { useProfile } from "@/lib/ProfileContext"
import { RBAC_CONFIG } from "@/lib/rbacConfig"
import { EmptyNote, ForbiddenBlock } from "../dashboard/_components/states"

type MeResponse = {
  id: string
  email: string
  role: string
  is_active: boolean
  driver_license?: string
}

type ManagedUser = {
  id: string
  name: string
  email: string
  phone: string
  role: string
}

const USERS_STORAGE_KEY = "transit.users.v1"
const USERS_PAGE_SIZE = 6
const MANAGED_ROLES = Object.keys(RBAC_CONFIG)

const SEED_USERS: ManagedUser[] = [
  {
    id: "seed-ops-admin",
    name: "Ops Admin",
    email: "admin@transitops.com",
    phone: "9820098200",
    role: "Admin",
  },
  {
    id: "seed-fleet-manager",
    name: "Fleet Manager",
    email: "fleet@transitops.com",
    phone: "9811098110",
    role: "Fleet Manager",
  },
]

function loadUsers(): ManagedUser[] {
  try {
    const raw = memoryGet(USERS_STORAGE_KEY)
    if (!raw) {
      memorySet(USERS_STORAGE_KEY, JSON.stringify(SEED_USERS))
      return SEED_USERS
    }
    const parsed = JSON.parse(raw) as ManagedUser[]
    return Array.isArray(parsed) ? parsed : SEED_USERS
  } catch {
    return SEED_USERS
  }
}

export default function SettingsPage() {
  const { logout } = useProfile()
  const [me, setMe] = useState<MeResponse | null>(null)
  const [loadingMe, setLoadingMe] = useState(true)
  const [meError, setMeError] = useState<string | null>(null)
  const [current, setCurrent] = useState("")
  const [next, setNext] = useState("")
  const [msg, setMsg] = useState<string | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const [users, setUsers] = useState<ManagedUser[] | null>(null)
  const [usersPage, setUsersPage] = useState(1)
  const [userDrawerOpen, setUserDrawerOpen] = useState(false)
  const [userForm, setUserForm] = useState({ name: "", email: "", phone: "", role: "Driver" })
  const [userError, setUserError] = useState<string | null>(null)

  const isUsersAdmin = me?.role === "Admin"

  useEffect(() => {
    let cancelled = false
    apiRequest<MeResponse>("/auth/me", { auth: false })
      .then((res) => {
        if (!cancelled) setMe(res)
      })
      .catch((e) => {
        if (!cancelled) setMeError(e instanceof ApiError ? e.detail : "Failed to load profile")
      })
      .finally(() => {
        if (!cancelled) setLoadingMe(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    setUsers(loadUsers())
  }, [])

  function persistUsers(next: ManagedUser[]) {
    setUsers(next)
    memorySet(USERS_STORAGE_KEY, JSON.stringify(next))
  }

  function saveUser(event: React.FormEvent) {
    event.preventDefault()
    if (!userForm.name.trim() || !userForm.email.trim()) {
      setUserError("User needs a name and an email.")
      return
    }
    const user: ManagedUser = {
      ...userForm,
      id:
        typeof crypto !== "undefined" && "randomUUID" in crypto
          ? crypto.randomUUID()
          : `user-${Date.now()}`,
    }
    persistUsers([user, ...(users ?? [])])
    setUserForm({ name: "", email: "", phone: "", role: "Driver" })
    setUserError(null)
    setUserDrawerOpen(false)
    setUsersPage(1)
  }

  const usersTotal = (users ?? []).length
  const usersTotalPages = Math.max(1, Math.ceil(usersTotal / USERS_PAGE_SIZE))
  const usersSafePage = Math.min(usersPage, usersTotalPages)
  const usersPageRows = (users ?? []).slice(
    (usersSafePage - 1) * USERS_PAGE_SIZE,
    usersSafePage * USERS_PAGE_SIZE,
  )

  async function changePassword(e: React.FormEvent) {
    e.preventDefault()
    setMsg(null)
    setErr(null)
    if (next.length < 8) {
      setErr("New password must be at least 8 characters.")
      return
    }
    setSaving(true)
    try {
      await apiRequest("/auth/change-password", {
        method: "POST",
        body: { current_password: current, new_password: next },
      })
      setMsg("Password updated. Other sessions revoked.")
      setCurrent("")
      setNext("")
    } catch (e) {
      setErr(e instanceof ApiError ? e.detail : "Failed to update password")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="max-w-xl space-y-6">
      <div>
        <h1 className="font-serif text-2xl text-foreground">Settings</h1>
        <p className="mt-1 text-sm text-muted-foreground">Profile and password for this session.</p>
      </div>

      <section aria-label="Signed in as" className="rounded-2xl border border-border bg-card p-5">
        <h2 className="text-sm font-medium text-muted-foreground">Signed in as</h2>
        {loadingMe ? (
          <div className="mt-3 space-y-2" role="status" aria-label="Loading profile">
            <div className="h-4 w-48 animate-pulse rounded bg-muted motion-reduce:animate-none" />
            <div className="h-4 w-32 animate-pulse rounded bg-muted motion-reduce:animate-none" />
          </div>
        ) : meError ? (
          <p role="alert" className="mt-3 text-sm text-destructive">{meError}</p>
        ) : me ? (
          <dl className="mt-3 space-y-1 text-sm">
            <div className="flex gap-2">
              <dt className="text-muted-foreground">Email</dt>
              <dd className="font-medium text-foreground">{me.email}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="text-muted-foreground">Role</dt>
              <dd className="font-medium text-foreground">{me.role}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="text-muted-foreground">Status</dt>
              <dd className="font-medium text-foreground">{me.is_active ? "Active" : "Inactive"}</dd>
            </div>
          </dl>
        ) : null}
        <Button className="mt-4" variant="secondary" onClick={logout}>
          Sign out
        </Button>
      </section>

      <form onSubmit={changePassword} aria-label="Change password" className="space-y-3 rounded-2xl border border-border bg-card p-5">
        <h2 className="font-medium text-foreground">Change password</h2>
        {msg && <p role="status" className="text-sm text-chart-5">{msg}</p>}
        {err && <p role="alert" className="text-sm text-destructive">{err}</p>}
        <div>
          <label htmlFor="current-pw" className="mb-1 block text-sm font-medium text-foreground">Current password</label>
          <Input id="current-pw" type="password" value={current} onChange={(e) => setCurrent(e.target.value)} required autoComplete="current-password" />
        </div>
        <div>
          <label htmlFor="next-pw" className="mb-1 block text-sm font-medium text-foreground">New password</label>
          <Input id="next-pw" type="password" value={next} onChange={(e) => setNext(e.target.value)} required autoComplete="new-password" />
        </div>
        <Button type="submit" disabled={saving}>{saving ? "Updating…" : "Update password"}</Button>
      </form>

      <section aria-label="Users admin">
        {isUsersAdmin ? (
          <CardShell
            as="div"
            aria-label="Users"
            tone="paper"
            className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
          >
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-[15px] font-medium tracking-tight text-foreground">
                  Users
                </h2>
                <p className="mt-0.5 text-sm text-muted-foreground">
                  Stored locally until the Users API lands. {(users ?? []).length}{" "}
                  {(users ?? []).length === 1 ? "user" : "users"}.
                </p>
              </div>
              <Button
                onClick={() => {
                  setUserForm({ name: "", email: "", phone: "", role: "Driver" })
                  setUserError(null)
                  setUserDrawerOpen(true)
                }}
              >
                Add user
              </Button>
            </div>
            {(users ?? []).length === 0 ? (
              <EmptyNote message="No users yet. Add the first user to get started." />
            ) : (
              <>
                <div className="overflow-x-auto">
                  <Table aria-label="Users">
                    <TableHead>
                      <TableRow>
                        <TableHeaderCell scope="col">Name</TableHeaderCell>
                        <TableHeaderCell scope="col">Role</TableHeaderCell>
                        <TableHeaderCell scope="col">Email</TableHeaderCell>
                        <TableHeaderCell scope="col">Phone</TableHeaderCell>
                        <TableHeaderCell scope="col">
                          <span className="sr-only">Row actions</span>
                          Actions
                        </TableHeaderCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {usersPageRows.map((user) => (
                          <TableRow key={user.id}>
                            <TableCell className="font-medium text-foreground">
                              {user.name}
                            </TableCell>
                            <TableCell>{user.role}</TableCell>
                            <TableCell>{user.email}</TableCell>
                            <TableCell className="tabular-nums">
                              {user.phone || "—"}
                            </TableCell>
                            <TableCell>
                              {user.id.startsWith("seed-") ? (
                                <span className="text-xs text-muted-foreground">—</span>
                              ) : (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() =>
                                    persistUsers((users ?? []).filter((u) => u.id !== user.id))
                                  }
                                  aria-label={`Remove ${user.name}`}
                                >
                                  Remove
                                </Button>
                              )}
                            </TableCell>
                          </TableRow>
                        ))}
                    </TableBody>
                  </Table>
                </div>
                <Pagination
                  page={usersSafePage}
                  pageSize={USERS_PAGE_SIZE}
                  total={usersTotal}
                  onPageChange={setUsersPage}
                />
              </>
            )}
          </CardShell>
        ) : (
          <ForbiddenBlock message="Users admin is limited to administrators." />
        )}
      </section>

      <Drawer open={userDrawerOpen} onOpenChange={setUserDrawerOpen}>
        <DrawerContent aria-label="Add user">
          <DrawerHeader>
            <DrawerTitle>Add user</DrawerTitle>
            <DrawerDescription>
              Stored locally until the Users API lands.
            </DrawerDescription>
          </DrawerHeader>
          <form onSubmit={saveUser}>
            <DrawerBody>
              <div className="grid grid-cols-1 gap-4">
                <div>
                  <label htmlFor="user-name" className="mb-1 block text-sm font-medium text-foreground">Name</label>
                  <Input
                    id="user-name"
                    value={userForm.name}
                    onChange={(event) => setUserForm({ ...userForm, name: event.target.value })}
                    required
                    autoComplete="off"
                  />
                </div>
                <div>
                  <label htmlFor="user-email" className="mb-1 block text-sm font-medium text-foreground">Email</label>
                  <Input
                    id="user-email"
                    type="email"
                    value={userForm.email}
                    onChange={(event) => setUserForm({ ...userForm, email: event.target.value })}
                    required
                    autoComplete="off"
                  />
                </div>
                <div>
                  <label htmlFor="user-phone" className="mb-1 block text-sm font-medium text-foreground">Phone</label>
                  <Input
                    id="user-phone"
                    type="tel"
                    value={userForm.phone}
                    onChange={(event) => setUserForm({ ...userForm, phone: event.target.value })}
                    autoComplete="off"
                  />
                </div>
                <div>
                  <label htmlFor="user-role" className="mb-1 block text-sm font-medium text-foreground">Role</label>
                  <Select
                    value={userForm.role}
                    onValueChange={(value) => setUserForm({ ...userForm, role: value })}
                  >
                    <SelectTrigger id="user-role">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MANAGED_ROLES.map((role) => (
                        <SelectItem key={role} value={role}>
                          {role}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              {userError && (
                <p role="alert" className="mt-4 text-sm text-destructive">
                  {userError}
                </p>
              )}
            </DrawerBody>
            <DrawerFooter>
              <Button type="button" variant="ghost" onClick={() => setUserDrawerOpen(false)}>
                Cancel
              </Button>
              <Button type="submit">Save user</Button>
            </DrawerFooter>
          </form>
        </DrawerContent>
      </Drawer>
    </div>
  )
}
