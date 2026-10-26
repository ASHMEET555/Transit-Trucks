import { BACKEND_URL } from "@/data/data"

export class ApiError extends Error {
  status: number
  detail: string
  constructor(status: number, detail: string) {
    super(detail)
    this.status = status
    this.detail = detail
  }
}

type RequestOptions = {
  method?: string
  body?: unknown
  form?: boolean
  auth?: boolean
}

export async function apiRequest<T = unknown>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = "GET", body, form = false, auth = true } = options
  const headers: Record<string, string> = {}

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

  const url = path.startsWith("http")
    ? path
    : `${BACKEND_URL}${path.startsWith("/") ? "" : "/"}${path}`

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
    throw new ApiError(401, "Unauthorized — please sign in again")
  }

  if (!response.ok) {
    let detail = `Request failed (${response.status})`
    try {
      const data = await response.json()
      if (data.error && typeof data.error.message === "string") {
        detail = data.error.message
      } else if (typeof data.detail === "string") {
        detail = data.detail
      } else if (data.detail) {
        detail = JSON.stringify(data.detail)
      }
    } catch {
      // ignore
    }
    throw new ApiError(response.status, detail)
  }

  if (response.status === 204) return undefined as T
  const text = await response.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export async function loginRequest(email: string, password: string) {
  return apiRequest<{
    token: string
    email: string
    role: string
  }>("/auth/login", {
    method: "POST",
    body: { email, password },
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
  })
}
