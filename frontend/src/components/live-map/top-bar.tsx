"use client"

import { Search } from "lucide-react"

type TopBarProps = {
  query: string
  onQueryChange: (q: string) => void
}

export function TopBar({ query, onQueryChange }: TopBarProps) {
  return (
    <div className="flex items-center gap-3">
      <form
        role="search"
        aria-label="Filter trips on the map"
        className="flex min-w-0 flex-1 items-center rounded-2xl bg-muted pl-4"
        onSubmit={(e) => e.preventDefault()}
      >
        <Search className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <label htmlFor="live-map-search" className="sr-only">
          Filter by vehicle, place, or driver license
        </label>
        <input
          id="live-map-search"
          type="search"
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          placeholder="Filter by vehicle, place, or driver license…"
          className="min-w-0 flex-1 bg-transparent px-3 py-2.5 text-sm text-foreground outline-none placeholder:text-muted-foreground"
        />
        <button
          type="submit"
          className="mr-1 hidden rounded-xl bg-card px-4 py-1.5 text-sm font-medium text-foreground shadow-sm sm:block"
        >
          Search
        </button>
      </form>
    </div>
  )
}
