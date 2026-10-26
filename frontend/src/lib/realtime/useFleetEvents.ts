"use client"

import { useEffect, useRef, useState, useCallback } from "react"
import { apiRequest } from "@/lib/api"
import { BACKEND_URL } from "@/data/data"

export type RealtimeStatus = "connecting" | "live" | "reconnecting" | "offline"

export interface FleetEvent {
  type: string
  ids: Record<string, any>
  occurred_at: string
}

export interface UseFleetEventsOptions {
  onEvent?: (event: FleetEvent) => void
  onReconnect?: () => void
  enabled?: boolean
}

export function useFleetEvents(options: UseFleetEventsOptions = {}) {
  const { onEvent, onReconnect, enabled = true } = options
  const [status, setStatus] = useState<RealtimeStatus>("connecting")
  const [lastEvent, setLastEvent] = useState<FleetEvent | null>(null)

  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  const onReconnectRef = useRef(onReconnect)
  onReconnectRef.current = onReconnect

  const socketRef = useRef<WebSocket | null>(null)
  const reconnectAttemptRef = useRef(0)
  const reconnectTimerRef = useRef<NodeJS.Timeout | null>(null)
  const isUnmountedRef = useRef(false)
  const hasConnectedOnceRef = useRef(false)

  const cleanup = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (socketRef.current) {
      socketRef.current.onopen = null
      socketRef.current.onmessage = null
      socketRef.current.onerror = null
      socketRef.current.onclose = null
      socketRef.current.close()
      socketRef.current = null
    }
  }, [])

  const scheduleReconnectRef = useRef<() => void>(() => {})

  const connect = useCallback(async () => {
    if (isUnmountedRef.current || !enabled) return

    cleanup()
    setStatus(hasConnectedOnceRef.current ? "reconnecting" : "connecting")

    try {
      // 1. Fetch single-use ticket
      const { ticket } = await apiRequest<{ ticket: string; expires_in: number }>(
        "/api/v1/realtime/ticket",
        { method: "POST" }
      )

      if (isUnmountedRef.current) return

      // 2. Build WebSocket URL
      const baseWsUrl = BACKEND_URL.replace(/^http/, "ws")
      const wsUrl = `${baseWsUrl}/ws?ticket=${encodeURIComponent(ticket)}`

      const ws = new WebSocket(wsUrl)
      socketRef.current = ws

      ws.onopen = () => {
        if (isUnmountedRef.current) {
          ws.close()
          return
        }
        setStatus("live")
        reconnectAttemptRef.current = 0
        if (hasConnectedOnceRef.current) {
          onReconnectRef.current?.()
        }
        hasConnectedOnceRef.current = true
      }

      ws.onmessage = (messageEvent) => {
        try {
          const raw = JSON.parse(messageEvent.data)
          // Handle both standard FleetEvent and legacy broadcast payloads
          const event: FleetEvent = {
            type: raw.type || raw.event || "unknown",
            ids: raw.ids || raw.payload || {},
            occurred_at: raw.occurred_at || new Date().toISOString(),
          }
          setLastEvent(event)
          onEventRef.current?.(event)
        } catch {
          // ignore malformed payloads
        }
      }

      ws.onclose = (event) => {
        if (isUnmountedRef.current) return

        // 4429: Too many connections from user
        if (event.code === 4429) {
          setStatus("offline")
          return
        }

        setStatus("reconnecting")
        scheduleReconnectRef.current()
      }

      ws.onerror = () => {
        // ws.onclose will fire following error
      }
    } catch {
      if (isUnmountedRef.current) return
      setStatus("reconnecting")
      scheduleReconnectRef.current()
    }
  }, [enabled, cleanup])

  const scheduleReconnect = useCallback(() => {
    if (reconnectTimerRef.current || isUnmountedRef.current || !enabled) return

    reconnectAttemptRef.current += 1
    // Exponential backoff: min(30s, 1s * 1.5^attempt + jitter)
    const baseDelay = Math.min(30000, Math.floor(Math.pow(1.5, reconnectAttemptRef.current) * 1000))
    const jitter = Math.floor(Math.random() * 500)
    const delay = baseDelay + jitter

    reconnectTimerRef.current = setTimeout(() => {
      reconnectTimerRef.current = null
      connect()
    }, delay)
  }, [enabled, connect])

  scheduleReconnectRef.current = scheduleReconnect

  useEffect(() => {
    isUnmountedRef.current = false
    if (enabled) {
      connect()
    } else {
      cleanup()
      setStatus("offline")
    }

    return () => {
      isUnmountedRef.current = true
      cleanup()
    }
  }, [enabled, connect, cleanup])

  return {
    status,
    lastEvent,
    reconnect: connect,
  }
}
