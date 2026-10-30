"use client"

import { useState } from "react"
import { Button } from "@/components/Button"
import { cn } from "@/lib/utils"

export interface DateRange {
  from: string | null
  to: string | null
}

interface DateRangePickerProps {
  value: DateRange
  onChange: (range: DateRange) => void
  id?: string
  label?: string
}

function toISO(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`
}

function parseISO(s: string | null): Date | null {
  if (!s) return null
  const [y, m, d] = s.split("-").map(Number)
  if (!y || !m || !d) return null
  return new Date(y, m - 1, d)
}

function fmtShort(s: string | null): string {
  if (!s) return "Start date"
  const d = parseISO(s)
  return d ? d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" }) : s
}

const WEEKDAYS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"]

export function DateRangePicker({ value, onChange, id, label = "Date range" }: DateRangePickerProps) {
  const [open, setOpen] = useState(false)
  const [cursor, setCursor] = useState(() => parseISO(value.from) ?? new Date())
  const from = parseISO(value.from)
  const to = parseISO(value.to)

  const year = cursor.getFullYear()
  const month = cursor.getMonth()
  const firstDow = new Date(year, month, 1).getDay()
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const cells: (Date | null)[] = []
  for (let i = 0; i < firstDow; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(new Date(year, month, d))

  function pick(day: Date) {
    const iso = toISO(day)
    if (!value.from || (value.from && value.to)) {
      onChange({ from: iso, to: null })
    } else if (iso < value.from) {
      onChange({ from: iso, to: value.from })
    } else {
      onChange({ from: value.from, to: iso })
    }
  }

  function inRange(day: Date): boolean {
    if (!from || !to) return false
    return day > from && day < to
  }
  function isEndpoint(day: Date): boolean {
    const iso = toISO(day)
    return iso === value.from || iso === value.to
  }

  const monthLabel = cursor.toLocaleDateString(undefined, { month: "long", year: "numeric" })

  return (
    <div className="relative" id={id}>
      <span className="sr-only">{label}</span>
      <Button
        type="button"
        variant="secondary"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="dialog"
      >
        {value.from ? fmtShort(value.from) : "Start date"}
        <span aria-hidden="true" className="text-muted-foreground"> → </span>
        {value.to ? fmtShort(value.to) : "End date"}
      </Button>
      {open && (
        <div
          role="dialog"
          aria-label="Choose start and end dates"
          className="absolute z-[700] mt-2 w-72 rounded-2xl border border-border bg-card p-4 shadow-xl"
        >
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-foreground">{monthLabel}</p>
            <div className="flex gap-1">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-label="Previous month"
                onClick={() => setCursor(new Date(year, month - 1, 1))}
              >
                ↑
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-label="Next month"
                onClick={() => setCursor(new Date(year, month + 1, 1))}
              >
                ↓
              </Button>
            </div>
          </div>
          <div className="mt-3 grid grid-cols-7 gap-1" role="grid" aria-label={monthLabel}>
            {WEEKDAYS.map((w) => (
              <span key={w} className="py-1 text-center text-[11px] font-medium text-muted-foreground">
                {w}
              </span>
            ))}
            {cells.map((day, i) => {
              if (day === null) return <span key={`b-${i}`} />
              const endpoint = isEndpoint(day)
              const mid = inRange(day)
              return (
                <button
                  key={toISO(day)}
                  type="button"
                  role="gridcell"
                  aria-selected={endpoint || mid}
                  aria-label={`${day.toLocaleDateString(undefined, { month: "long", day: "numeric", year: "numeric" })}${endpoint ? ", selected endpoint" : mid ? ", in selected range" : ""}`}
                  onClick={() => pick(day)}
                  className={cn(
                    "grid size-9 place-items-center rounded-full text-sm tabular-nums transition-colors focus-visible:outline-2 focus-visible:outline-ring",
                    endpoint
                      ? "bg-primary font-semibold text-primary-foreground"
                      : mid
                        ? "rounded-none bg-primary/15 text-foreground"
                        : "text-foreground hover:bg-muted",
                  )}
                >
                  {day.getDate()}
                </button>
              )
            })}
          </div>
          <div className="mt-3 flex items-center justify-between border-t border-border pt-3">
            <Button type="button" variant="ghost" size="sm" onClick={() => onChange({ from: null, to: null })}>
              Clear
            </Button>
            <Button type="button" size="sm" onClick={() => setOpen(false)}>
              Done
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
