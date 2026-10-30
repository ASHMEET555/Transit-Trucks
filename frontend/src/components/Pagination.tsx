"use client"

import { Button } from "@/components/Button"

interface PaginationProps {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
  id?: string
}

export function Pagination({ page, pageSize, total, onPageChange, id }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const safePage = Math.min(Math.max(1, page), totalPages)
  if (totalPages <= 1) return null

  const from = (safePage - 1) * pageSize + 1
  const to = Math.min(safePage * pageSize, total)

  const numbers: (number | "…")[] = []
  for (let p = 1; p <= totalPages; p++) {
    if (p === 1 || p === totalPages || Math.abs(p - safePage) <= 1) {
      numbers.push(p)
    } else if (numbers[numbers.length - 1] !== "…") {
      numbers.push("…")
    }
  }

  return (
    <nav
      id={id}
      aria-label="Pagination"
      className="flex flex-wrap items-center justify-between gap-3 pt-4"
    >
      <p className="text-xs text-muted-foreground" role="status">
        Showing {from} to {to} of {total} results
      </p>
      <div className="flex items-center gap-1">
        <Button
          variant="secondary"
          size="sm"
          disabled={safePage <= 1}
          onClick={() => onPageChange(safePage - 1)}
          aria-label="Previous page"
        >
          ‹
        </Button>
        {numbers.map((n, i) =>
          n === "…" ? (
            <span key={`gap-${i}`} className="px-1 text-xs text-muted-foreground" aria-hidden="true">
              …
            </span>
          ) : (
            <Button
              key={n}
              variant={n === safePage ? "default" : "ghost"}
              size="sm"
              onClick={() => onPageChange(n)}
              aria-label={`Page ${n}`}
              aria-current={n === safePage ? "page" : undefined}
            >
              {n}
            </Button>
          ),
        )}
        <Button
          variant="secondary"
          size="sm"
          disabled={safePage >= totalPages}
          onClick={() => onPageChange(safePage + 1)}
          aria-label="Next page"
        >
          ›
        </Button>
      </div>
    </nav>
  )
}
