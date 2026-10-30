"use client"

import type { LucideIcon } from "lucide-react"
import { Lock, TriangleAlert } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/Button"
import { Skeleton } from "@/components/Skeleton"
import { CardShell } from "@/components/fleet/card-shell"

export function IconBadge({
  icon: Icon,
  className,
}: {
  icon: LucideIcon
  className?: string
}) {
  return (
    <span
      className={cn(
        "grid size-10 shrink-0 place-items-center rounded-full bg-current/8 ring-1 ring-current/10",
        className,
      )}
    >
      <Icon className="size-[18px]" aria-hidden="true" />
    </span>
  )
}

export function LoadingBlock({ label }: { label: string }) {
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={label}
      className="space-y-4"
    >
      <Skeleton className="h-5 w-48 rounded-full" />
      <Skeleton className="h-28 w-full rounded-[28px]" />
      <Skeleton className="h-28 w-full rounded-[28px]" />
      <span className="sr-only">{label}</span>
    </div>
  )
}

export function ErrorBlock({
  message,
  onRetry,
}: {
  message: string
  onRetry: () => void
}) {
  return (
    <CardShell
      tone="paper"
      className="items-center p-8 text-center motion-reduce:transition-none motion-reduce:hover:translate-y-0"
    >
      <IconBadge icon={TriangleAlert} />
      <p className="mt-4 text-sm font-medium text-foreground">
        Something went wrong
      </p>
      <p className="mt-1 text-sm text-muted-foreground">{message}</p>
      <Button variant="secondary" className="mt-4" onClick={onRetry}>
        Try again
      </Button>
    </CardShell>
  )
}

export function ForbiddenBlock({ message }: { message: string }) {
  return (
    <CardShell
      tone="paper"
      className="items-center p-8 text-center motion-reduce:transition-none motion-reduce:hover:translate-y-0"
    >
      <IconBadge icon={Lock} />
      <p className="mt-4 text-sm font-medium text-foreground">No access</p>
      <p className="mt-1 text-sm text-muted-foreground">{message}</p>
    </CardShell>
  )
}

export function EmptyNote({ message }: { message: string }) {
  return (
    <p className="py-8 text-center text-sm text-muted-foreground">{message}</p>
  )
}

export function SectionHeading({
  title,
  description,
  action,
  icon,
}: {
  title: string
  description?: string
  action?: React.ReactNode
  icon?: LucideIcon
}) {
  return (
    <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        {icon ? <IconBadge icon={icon} /> : null}
        <div>
          <h2 className="text-base font-semibold tracking-tight text-foreground">
            {title}
          </h2>
          {description ? (
            <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>
          ) : null}
        </div>
      </div>
      {action}
    </div>
  )
}
