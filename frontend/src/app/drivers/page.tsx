"use client"

import React, { useState, useEffect } from "react"
import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
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
import { ApiError, apiRequest, extractItems } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { canWrite } from "@/lib/rbacConfig"

type DriverRow = {
  license_number: string
  name: string
  license_category: string
  license_expiry_date: string
  contact_number: string
  safety_score: number
  status: string
}

const emptyForm = {
  license_number: "",
  name: "",
  license_category: "Light Transport",
  license_expiry_date: "",
  contact_number: "",
  safety_score: 100,
  status: "Available",
}

export default function DriversPage() {
  const { currentProfile } = useProfile()
  const [drivers, setDrivers] = useState<DriverRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  const [statusFilter, setStatusFilter] = useState<string>("All")
  const [sortSafetyScore, setSortSafetyScore] = useState<"asc" | "desc" | "none">(
    "none",
  )
  const [formData, setFormData] = useState(emptyForm)

  const canAddDriver = canWrite(currentProfile?.role || "", "drivers")
  const canDeleteDriver =
    currentProfile?.role === "Fleet Manager" || currentProfile?.role === "Admin"

  useEffect(() => {
    fetchDrivers()
  }, [])

  const fetchDrivers = async () => {
    try {
      setError(null)
      const res = await apiRequest<any>("/fleet/drivers")
      setDrivers(extractItems<DriverRow>(res))
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to fetch drivers")
    } finally {
      setLoading(false)
    }
  }

  const handleAddDriver = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!canAddDriver) return
    try {
      setError(null)
      await apiRequest("/fleet/drivers", { method: "POST", body: formData })
      await fetchDrivers()
      setShowAddForm(false)
      setFormData(emptyForm)
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to add driver")
    }
  }

  const handleDeleteDriver = async (licenseNum: string) => {
    if (!canDeleteDriver) return
    if (!confirm("Are you sure you want to delete this driver?")) return
    try {
      setError(null)
      await apiRequest(`/fleet/drivers/${encodeURIComponent(licenseNum)}`, {
        method: "DELETE",
      })
      await fetchDrivers()
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Failed to delete driver")
    }
  }

  const filteredAndSortedDrivers = drivers
    .filter((driver) => {
      if (statusFilter !== "All" && driver.status !== statusFilter) return false
      return true
    })
    .sort((a, b) => {
      if (sortSafetyScore === "asc") return a.safety_score - b.safety_score
      if (sortSafetyScore === "desc") return b.safety_score - a.safety_score
      return 0
    })

  const getStatusColor = (status: string) => {
    switch (status) {
      case "Available":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
      case "On Trip":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
      case "Off Duty":
        return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
      case "Suspended":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200"
    }
  }

  const getSafetyScoreColor = (score: number) => {
    if (score >= 90) return "text-green-600 dark:text-green-400"
    if (score >= 70) return "text-yellow-600 dark:text-yellow-400"
    if (score >= 50) return "text-orange-600 dark:text-orange-400"
    return "text-red-600 dark:text-red-400"
  }

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border/60 pb-5">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">Drivers & Compliance</h1>
          <p className="text-xs text-muted-foreground mt-1">
            Commercial license verification, driver shift availability, duty status, and compliance safety records.
          </p>
        </div>
        {canAddDriver && (
          <Button
            onClick={() => setShowAddForm(!showAddForm)}
            className="bg-primary text-primary-foreground hover:bg-primary/90"
          >
            {showAddForm ? "Close Form" : "Add New Driver"}
          </Button>
        )}
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-950">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              <span className="mr-2 text-red-600 dark:text-red-400">⚠</span>
              <p className="text-sm font-medium text-red-900 dark:text-red-100">
                {error}
              </p>
            </div>
            <button
              onClick={() => setError(null)}
              className="text-sm font-medium text-red-700 hover:text-red-900 dark:text-red-300 dark:hover:text-red-100"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      <div className="mb-6 flex flex-wrap gap-4 rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-950">
        <div className="min-w-[200px] flex-1">
          <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
            Status
          </label>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="All">All Statuses</SelectItem>
              <SelectItem value="Available">Available</SelectItem>
              <SelectItem value="On Trip">On Trip</SelectItem>
              <SelectItem value="Off Duty">Off Duty</SelectItem>
              <SelectItem value="Suspended">Suspended</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="min-w-[200px] flex-1">
          <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
            Sort by Safety Score
          </label>
          <Select
            value={sortSafetyScore}
            onValueChange={(value) =>
              setSortSafetyScore(value as "asc" | "desc" | "none")
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">No sorting</SelectItem>
              <SelectItem value="asc">Low to High</SelectItem>
              <SelectItem value="desc">High to Low</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {showAddForm && canAddDriver && (
        <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-950">
          <h2 className="mb-4 text-lg font-semibold">Add New Driver</h2>
          <form
            onSubmit={handleAddDriver}
            className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
          >
            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                License Number *
              </label>
              <Input
                required
                value={formData.license_number}
                onChange={(e) =>
                  setFormData({ ...formData, license_number: e.target.value })
                }
                placeholder="e.g., DL123456789"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Name *
              </label>
              <Input
                required
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="e.g., John Doe"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                License Category *
              </label>
              <Select
                value={formData.license_category}
                onValueChange={(value) =>
                  setFormData({ ...formData, license_category: value })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Light Transport">Light Transport</SelectItem>
                  <SelectItem value="Heavy Transport">Heavy Transport</SelectItem>
                  <SelectItem value="Commercial">Commercial</SelectItem>
                  <SelectItem value="Hazardous Materials">
                    Hazardous Materials
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                License Expiry Date *
              </label>
              <Input
                required
                type="date"
                value={formData.license_expiry_date}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    license_expiry_date: e.target.value,
                  })
                }
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Contact Number *
              </label>
              <Input
                required
                value={formData.contact_number}
                onChange={(e) =>
                  setFormData({ ...formData, contact_number: e.target.value })
                }
                placeholder="e.g., +1234567890"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Safety Score *
              </label>
              <Input
                required
                type="number"
                min="0"
                max="100"
                step="0.1"
                value={formData.safety_score || ""}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    safety_score: parseFloat(e.target.value) || 0,
                  })
                }
                placeholder="0-100"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-gray-50">
                Status *
              </label>
              <Select
                value={formData.status}
                onValueChange={(value) =>
                  setFormData({ ...formData, status: value })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Available">Available</SelectItem>
                  <SelectItem value="On Trip">On Trip</SelectItem>
                  <SelectItem value="Off Duty">Off Duty</SelectItem>
                  <SelectItem value="Suspended">Suspended</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-end md:col-span-2 lg:col-span-3">
              <div className="flex gap-2">
                <Button type="submit">Add Driver</Button>
                <Button
                  variant="secondary"
                  type="button"
                  onClick={() => setShowAddForm(false)}
                >
                  Cancel
                </Button>
              </div>
            </div>
          </form>
        </div>
      )}

      <div className="rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
        <Table>
          <TableHead>
            <TableRow>
              <TableHeaderCell>License Number</TableHeaderCell>
              <TableHeaderCell>Name</TableHeaderCell>
              <TableHeaderCell>License Category</TableHeaderCell>
              <TableHeaderCell>Expiry Date</TableHeaderCell>
              <TableHeaderCell>Contact Number</TableHeaderCell>
              <TableHeaderCell>Safety Score</TableHeaderCell>
              <TableHeaderCell>Status</TableHeaderCell>
              {canDeleteDriver && <TableHeaderCell>Actions</TableHeaderCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center">
                  Loading drivers...
                </TableCell>
              </TableRow>
            ) : filteredAndSortedDrivers.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center">
                  No drivers found
                </TableCell>
              </TableRow>
            ) : (
              filteredAndSortedDrivers.map((driver) => (
                <TableRow key={driver.license_number}>
                  <TableCell className="font-medium">
                    {driver.license_number}
                  </TableCell>
                  <TableCell>{driver.name}</TableCell>
                  <TableCell>{driver.license_category}</TableCell>
                  <TableCell>{driver.license_expiry_date}</TableCell>
                  <TableCell>{driver.contact_number}</TableCell>
                  <TableCell
                    className={`font-medium ${getSafetyScoreColor(driver.safety_score)}`}
                  >
                    {Number(driver.safety_score).toFixed(1)}
                  </TableCell>
                  <TableCell>
                    <span
                      className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${getStatusColor(driver.status)}`}
                    >
                      {driver.status}
                    </span>
                  </TableCell>
                  {canDeleteDriver && (
                    <TableCell>
                      <Button
                        variant="destructive"
                        onClick={() =>
                          handleDeleteDriver(driver.license_number)
                        }
                      >
                        Delete
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
