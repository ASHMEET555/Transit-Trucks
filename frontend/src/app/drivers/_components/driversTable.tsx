"use client"

import { Button } from "@/components/Button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/Table"
import { DotPill } from "./chrome"
import { expiryStateOf, plusDaysKey, safetyBand, todayKey, type DriverRow } from "./driverTypes"

function statusPill(status: string): { dot: string; pill: string } {
  switch (status) {
    case "Available":
      return { dot: "bg-chart-5", pill: "bg-sage text-foreground" }
    case "On Trip":
      return { dot: "bg-amber-600", pill: "bg-sand text-foreground" }
    case "Off Duty":
      return { dot: "bg-primary", pill: "bg-mist text-foreground" }
    case "Suspended":
      return { dot: "bg-destructive", pill: "bg-destructive/12 text-destructive" }
    default:
      return { dot: "bg-muted-foreground", pill: "bg-muted text-muted-foreground" }
  }
}

function expiryPill(expiry: "expired" | "expiring"): { dot: string; pill: string; label: string } {
  return expiry === "expired"
    ? { dot: "bg-destructive", pill: "bg-destructive/12 text-destructive", label: "Expired" }
    : { dot: "bg-amber-600", pill: "bg-sand text-foreground", label: "Expiring soon" }
}

interface DriversTableProps {
  drivers: DriverRow[]
  canEdit: boolean
  canDelete: boolean
  deletingLicense: string | null
  onEdit: (driver: DriverRow) => void
  onDelete: (license: string) => void
}

export function DriversTable({
  drivers,
  canEdit,
  canDelete,
  deletingLicense,
  onEdit,
  onDelete,
}: DriversTableProps) {
  const today = todayKey()
  const horizon = plusDaysKey(30)
  const showActions = canEdit || canDelete

  return (
    <Table>
      <TableHead>
        <TableRow>
          <TableHeaderCell>Licence number</TableHeaderCell>
          <TableHeaderCell>Name</TableHeaderCell>
          <TableHeaderCell>Category</TableHeaderCell>
          <TableHeaderCell>Expiry date</TableHeaderCell>
          <TableHeaderCell>Contact</TableHeaderCell>
          <TableHeaderCell>Safety score</TableHeaderCell>
          <TableHeaderCell>Status</TableHeaderCell>
          {showActions && <TableHeaderCell>Actions</TableHeaderCell>}
        </TableRow>
      </TableHead>
      <TableBody>
        {drivers.map((driver) => {
          const expiry = expiryStateOf(driver.license_expiry_date, today, horizon)
          const status = statusPill(driver.status)
          return (
            <TableRow key={driver.license_number}>
              <TableCell className="font-serif font-medium text-foreground">
                {driver.license_number}
              </TableCell>
              <TableCell className="font-serif">{driver.name}</TableCell>
              <TableCell>{driver.license_category}</TableCell>
              <TableCell
                className={
                  expiry === "expired"
                    ? "font-semibold text-destructive"
                    : expiry === "expiring"
                      ? "font-medium text-foreground"
                      : undefined
                }
              >
                <span className="flex flex-wrap items-center gap-2">
                  <span>{driver.license_expiry_date || "Not recorded"}</span>
                  {expiry !== "valid" && (() => {
                    const pill = expiryPill(expiry)
                    return (
                      <DotPill dot={pill.dot} pill={pill.pill}>
                        {pill.label}
                      </DotPill>
                    )
                  })()}
                </span>
              </TableCell>
              <TableCell className="font-mono tabular-nums">{driver.contact_number}</TableCell>
              <TableCell className="tabular-nums">
                {Number(driver.safety_score).toFixed(1)} · {safetyBand(Number(driver.safety_score))}
              </TableCell>
              <TableCell>
                <DotPill dot={status.dot} pill={status.pill}>{driver.status}</DotPill>
              </TableCell>
              {showActions && (
                <TableCell>
                  <span className="flex gap-2">
                    {canEdit && (
                      <Button variant="secondary" onClick={() => onEdit(driver)}>
                        Edit
                      </Button>
                    )}
                    {canDelete && (
                      <Button
                        variant="destructive"
                        isLoading={deletingLicense === driver.license_number}
                        loadingText="Deleting"
                        onClick={() => onDelete(driver.license_number)}
                      >
                        Delete
                      </Button>
                    )}
                  </span>
                </TableCell>
              )}
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
