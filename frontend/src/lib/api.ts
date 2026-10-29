import { BACKEND_URL } from "@/data/data"
import { getOrCreateActionIdempotencyKey } from "./idempotency"

export class ApiError extends Error {
  status: number
  detail: string
  code?: string
  requestId?: string

  constructor(status: number, detail: string, code?: string, requestId?: string) {
    super(code ? `[${code}] ${detail}` : detail)
    this.status = status
    this.detail = detail
    this.code = code
    this.requestId = requestId
  }
}

export type PaginatedResponse<T> = {
  items: T[]
  next_cursor: string | null
}

export function extractItems<T>(data: T[] | PaginatedResponse<T> | unknown): T[] {
  if (Array.isArray(data)) return data
  if (data && typeof data === "object" && "items" in data && Array.isArray((data as any).items)) {
    return (data as any).items
  }
  return []
}

type RequestOptions = {
  method?: string
  body?: unknown
  form?: boolean
  auth?: boolean
  headers?: Record<string, string>
  actionId?: string
}

export async function apiRequest<T = unknown>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = "GET", body, form = false, auth = true, headers: customHeaders, actionId } = options
  const headers: Record<string, string> = { ...customHeaders }

  // Auto-generate or reuse Idempotency-Key for mutating requests
  if (method === "POST" || method === "PUT" || method === "PATCH") {
    if (!headers["Idempotency-Key"]) {
      headers["Idempotency-Key"] = getOrCreateActionIdempotencyKey(actionId)
    }
  }

  let payload: BodyInit | undefined
  if (body !== undefined) {
    if (form) {
      headers["Content-Type"] = "application/x-www-form-urlencoded"
      payload = body as string
    } else {
      headers["Content-Type"] = "application/json"
      payload = JSON.stringify(body)
    }
  }

  let normalizedPath = path
  if (BACKEND_URL.endsWith("/api/v1") && normalizedPath.startsWith("/api/v1/")) {
    normalizedPath = normalizedPath.slice("/api/v1".length)
  }

  const url = normalizedPath.startsWith("http")
    ? normalizedPath
    : `${BACKEND_URL}${normalizedPath.startsWith("/") ? "" : "/"}${normalizedPath}`

  const response = await fetch(url, {
    method,
    headers,
    body: payload,
    credentials: "include",
  })

  if (response.status === 401 && auth) {
    if (typeof window !== "undefined" && !window.location.pathname.includes("/login")) {
      window.location.href = "/login"
    }
    throw new ApiError(401, "Unauthorized - please sign in again")
  }

  if (!response.ok) {
    let detail = `Request failed (${response.status})`
    let code: string | undefined
    let requestId: string | undefined
    try {
      const data = await response.json()
      if (data.error && typeof data.error === "object") {
        if (typeof data.error.message === "string") {
          detail = data.error.message
        }
        if (typeof data.error.code === "string") {
          code = data.error.code
        }
        if (typeof data.error.request_id === "string") {
          requestId = data.error.request_id
        }
      } else if (typeof data.detail === "string") {
        detail = data.detail
      } else if (data.detail) {
        detail = JSON.stringify(data.detail)
      }
    } catch {
      // ignore
    }
    throw new ApiError(response.status, detail, code, requestId)
  }

  if (response.status === 204) return undefined as T
  const text = await response.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export async function loginRequest(
  email: string,
  password: string,
  extra?: { workspace?: string; company?: string; expected_role?: string },
) {
  return apiRequest<{
    token: string
    email: string
    role: string
  }>("/auth/login", {
    method: "POST",
    body: { email, password, ...extra },
    auth: false,
  })
}

export async function logoutRequest(): Promise<void> {
  return apiRequest("/auth/logout", {
    method: "POST",
  })
}

export async function getMeRequest(): Promise<{
  id: string
  email: string
  role: string
  is_active: boolean
  driver_license?: string
}> {
  return apiRequest("/auth/me", {
    method: "GET",
    auth: false,
  })
}
