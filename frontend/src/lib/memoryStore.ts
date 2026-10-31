// In-memory session store. The backend hygiene gate forbids browser web
// storage APIs anywhere under frontend/src, so UI-only state (filters,
// demo seeds, display metadata) lives here instead.
// Values last for the tab's lifetime and never leave the client.
const store = new Map<string, string>()

export function memoryGet(key: string, fallback = ""): string {
  return store.get(key) ?? fallback
}

export function memorySet(key: string, value: string): void {
  store.set(key, value)
}
