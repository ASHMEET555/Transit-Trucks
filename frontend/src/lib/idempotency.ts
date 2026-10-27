export function generateUUID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID()
  }
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === "x" ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

const actionIdempotencyCache = new Map<string, string>()

export function getOrCreateActionIdempotencyKey(actionId?: string): string {
  if (actionId) {
    const existing = actionIdempotencyCache.get(actionId)
    if (existing) return existing
    const newKey = generateUUID()
    actionIdempotencyCache.set(actionId, newKey)
    return newKey
  }
  return generateUUID()
}

export function clearActionIdempotencyKey(actionId: string): void {
  actionIdempotencyCache.delete(actionId)
}
