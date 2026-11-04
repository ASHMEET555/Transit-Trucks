"use client"

import L from "leaflet"
import React from "react"
import { cn } from "@/lib/utils"

export type TruckStatusType =
  | "on_route"
  | "at_stop"
  | "delayed"
  | "off_route"
  | "breakdown"
  | "no_signal"

export interface TruckMarkerProps {
  vehicleReg: string
  headingDeg: number
  status: TruckStatusType | string
  alerts?: string[]
  selected?: boolean
  isReefer?: boolean
  source?: string | null
  speedKmh?: number
  fuelPct?: number | null
  reeferTempC?: number | null
  eta?: string
  driverName?: string | null
}

export function getStatusTheme(status: string, alerts?: string[]) {
  if (alerts && alerts.length > 0) {
    return {
      border: "#ef4444", // destructive
      bg: "#fef2f2",
      text: "#dc2626",
      label: "Alert",
      ringClass: "border-destructive ring-destructive/20",
    }
  }
  switch (status) {
    case "delayed":
      return {
        border: "#d97706", // signal
        bg: "#fffbeb",
        text: "#b45309",
        label: "Delayed",
        ringClass: "border-signal ring-signal/20",
      }
    case "off_route":
    case "breakdown":
      return {
        border: "#ef4444", // destructive
        bg: "#fef2f2",
        text: "#dc2626",
        label: status === "breakdown" ? "Breakdown" : "Off route",
        ringClass: "border-destructive ring-destructive/20",
      }
    case "no_signal":
    case "offline":
      return {
        border: "#9ca3af", // muted
        bg: "#f3f4f6",
        text: "#6b7280",
        label: "Offline",
        ringClass: "border-muted ring-muted/20",
      }
    case "at_stop":
      return {
        border: "#0284c7",
        bg: "#f0f9ff",
        text: "#0369a1",
        label: "At stop",
        ringClass: "border-sky-500 ring-sky-500/20",
      }
    case "on_route":
    default:
      return {
        border: "#1b4332", // primary
        bg: "#f0fdf4",
        text: "#15803d",
        label: "On route",
        ringClass: "border-primary ring-primary/20",
      }
  }
}

/**
 * Top-down SVG of semi-truck: Cab (front) with hood + windshield, trailer (back) with cargo lines.
 * Pointing North (0 deg).
 */
export function SemiTruckSvg({
  className,
  isReefer,
}: {
  className?: string
  isReefer?: boolean
}) {
  return (
    <svg
      viewBox="0 0 24 44"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={cn("size-full shrink-0", className)}
      aria-hidden="true"
    >
      {/* Trailer (Rear) */}
      <rect
        x="4"
        y="14"
        width="16"
        height="26"
        rx="2"
        fill="currentColor"
        fillOpacity="0.88"
        stroke="#111827"
        strokeWidth="1.2"
      />
      {/* Trailer corrugated roof lines */}
      <line x1="7" y1="18" x2="17" y2="18" stroke="#ffffff" strokeWidth="0.8" strokeOpacity="0.4" />
      <line x1="7" y1="23" x2="17" y2="23" stroke="#ffffff" strokeWidth="0.8" strokeOpacity="0.4" />
      <line x1="7" y1="28" x2="17" y2="28" stroke="#ffffff" strokeWidth="0.8" strokeOpacity="0.4" />
      <line x1="7" y1="33" x2="17" y2="33" stroke="#ffffff" strokeWidth="0.8" strokeOpacity="0.4" />

      {/* Kingpin hitch */}
      <rect x="10" y="11" width="4" height="4" rx="1" fill="#111827" />

      {/* Cab (Front) */}
      <path
        d="M6 12V6C6 4.34315 7.34315 3 9 3H15C16.6569 3 18 4.34315 18 6V12H6Z"
        fill="currentColor"
        stroke="#111827"
        strokeWidth="1.2"
      />
      {/* Windshield */}
      <path
        d="M8 6.5C8 5.67157 8.67157 5 9.5 5H14.5C15.3284 5 16 5.67157 16 6.5V8.5H8V6.5Z"
        fill="#0f172a"
      />
      {/* Side mirrors */}
      <rect x="4.5" y="7" width="1.5" height="3" rx="0.5" fill="#111827" />
      <rect x="18" y="7" width="1.5" height="3" rx="0.5" fill="#111827" />
    </svg>
  )
}

/**
 * Creates Leaflet DivIcon with rotating semi-truck SVG, status ring, pulse if selected,
 * and reefer snowflake badge.
 */
export function createTruckDivIcon(props: TruckMarkerProps): L.DivIcon {
  const {
    vehicleReg,
    headingDeg,
    status,
    alerts = [],
    selected = false,
    isReefer = false,
  } = props

  const theme = getStatusTheme(status, alerts)
  const size = selected ? 40 : 32
  const anchor = size / 2

  const pulseRingHtml = selected
    ? `<span style="position:absolute; inset:-6px; border-radius:9999px; border:2px solid ${theme.border}; opacity:0.6; animation:ping 1.5s cubic-bezier(0,0,0.2,1) infinite; pointer-events:none;"></span>`
    : ""

  const reeferBadgeHtml = isReefer
    ? `<span style="position:absolute; top:-2px; right:-2px; z-index:10; background:#38bdf8; color:#ffffff; font-size:9px; width:13px; height:13px; border-radius:9999px; display:grid; place-items:center; border:1px solid #ffffff; box-shadow:0 1px 2px rgba(0,0,0,0.15);" title="Refrigerated">&#10052;</span>`
    : ""

  const iconHtml = `
    <div style="position:relative; width:${size}px; height:${size}px; display:grid; place-items:center;">
      ${pulseRingHtml}
      <div style="
        position:relative;
        width:${size}px;
        height:${size}px;
        border-radius:9999px;
        background:#ffffff;
        border:2.5px solid ${theme.border};
        box-shadow:0 3px 10px rgba(0,0,0,0.15);
        display:grid;
        place-items:center;
        color:${theme.border};
        transform: rotate(${Math.round(headingDeg)}deg);
        transition: transform 0.4s ease;
      ">
        <div style="width:${Math.round(size * 0.65)}px; height:${Math.round(size * 0.85)}px;">
          <svg viewBox="0 0 24 44" fill="none" style="width:100%; height:100%;" aria-hidden="true">
            <rect x="4" y="14" width="16" height="26" rx="2" fill="currentColor" fill-opacity="0.88" stroke="#111827" stroke-width="1.2"/>
            <line x1="7" y1="18" x2="17" y2="18" stroke="#ffffff" stroke-width="0.8" stroke-opacity="0.5"/>
            <line x1="7" y1="23" x2="17" y2="23" stroke="#ffffff" stroke-width="0.8" stroke-opacity="0.5"/>
            <line x1="7" y1="28" x2="17" y2="28" stroke="#ffffff" stroke-width="0.8" stroke-opacity="0.5"/>
            <line x1="7" y1="33" x2="17" y2="33" stroke="#ffffff" stroke-width="0.8" stroke-opacity="0.5"/>
            <rect x="10" y="11" width="4" height="4" rx="1" fill="#111827"/>
            <path d="M6 12V6C6 4.34315 7.34315 3 9 3H15C16.6569 3 18 4.34315 18 6V12H6Z" fill="currentColor" stroke="#111827" stroke-width="1.2"/>
            <path d="M8 6.5C8 5.67157 8.67157 5 9.5 5H14.5C15.3284 5 16 5.67157 16 6.5V8.5H8V6.5Z" fill="#0f172a"/>
            <rect x="4.5" y="7" width="1.5" height="3" rx="0.5" fill="#111827"/>
            <rect x="18" y="7" width="1.5" height="3" rx="0.5" fill="#111827"/>
          </svg>
        </div>
      </div>
      ${reeferBadgeHtml}
    </div>
  `

  return L.divIcon({
    className: "truck-marker-icon-container",
    html: iconHtml,
    iconSize: [size, size],
    iconAnchor: [anchor, anchor],
  })
}
