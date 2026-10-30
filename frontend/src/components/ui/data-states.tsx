import React from "react"
import { AlertCircle, AlertTriangle, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"

export function TableSkeleton({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="w-full space-y-3 animate-pulse" data-testid="table-skeleton">
      <div className="h-10 bg-neutral-200/60 rounded-md w-full" />
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex gap-4">
          {Array.from({ length: cols }).map((_, j) => (
            <div key={j} className="h-8 bg-neutral-100 rounded-md flex-1" />
          ))}
        </div>
      ))}
    </div>
  )
}

export function EmptyState({
  title,
  description,
  actionLabel,
  onAction,
}: {
  title: string
  description?: string
  actionLabel?: string
  onAction?: () => void
}) {
  return (
    <div className="flex flex-col items-center justify-center p-8 text-center rounded-xl border border-dashed border-neutral-300/80 bg-neutral-50/50">
      <div className="size-10 rounded-full bg-neutral-200/60 flex items-center justify-center mb-3 text-neutral-500">
        <AlertCircle className="size-5" />
      </div>
      <h3 className="text-sm font-semibold text-neutral-800">{title}</h3>
      {description && <p className="text-xs text-neutral-500 max-w-sm mt-1 mb-4">{description}</p>}
      {actionLabel && onAction && (
        <Button size="sm" onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </div>
  )
}

export function ErrorState({
  message,
  onRetry,
}: {
  message: string
  onRetry?: () => void
}) {
  return (
    <div className="flex flex-col items-center justify-center p-6 text-center rounded-xl border border-destructive/20 bg-destructive/5 text-destructive">
      <div className="size-9 rounded-full bg-destructive/10 flex items-center justify-center mb-2">
        <AlertTriangle className="size-4" />
      </div>
      <p className="text-xs font-medium max-w-md mb-3">{message}</p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry} className="gap-1.5 border-destructive/30 hover:bg-destructive/10">
          <RefreshCw className="size-3.5" /> Retry
        </Button>
      )}
    </div>
  )
}

export function ForbiddenState({
  message = "You do not have permission to view or manage these operational records.",
}: {
  message?: string
}) {
  return (
    <div className="flex flex-col items-center justify-center p-10 text-center rounded-xl border border-neutral-200 bg-white">
      <div className="size-11 rounded-full bg-amber-500/10 text-amber-600 flex items-center justify-center mb-3">
        <AlertCircle className="size-5" />
      </div>
      <h3 className="text-sm font-semibold text-neutral-800">Access Restricted</h3>
      <p className="text-xs text-neutral-500 max-w-sm mt-1">{message}</p>
    </div>
  )
}
