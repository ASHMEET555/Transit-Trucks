import type { ComponentType, ReactNode } from "react"
import { cn } from "@/lib/utils"

export function CardHead({
  title,
  meta,
  action,
}: {
  title: string
  meta?: ReactNode
  action?: ReactNode
}) {
  return (
    <header className="flex flex-wrap items-center justify-between gap-3">
      <h3 className="text-[15px] font-medium tracking-tight">{title}</h3>
      {action ??
        (meta != null ? (
          <p className="text-xs tabular-nums text-muted-foreground">{meta}</p>
        ) : null)}
    </header>
  )
}

export function DotPill({
  dot,
  pill,
  children,
}: {
  dot: string
  pill: string
  children: ReactNode
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium",
        pill,
      )}
    >
      <span className={cn("size-1.5 rounded-full", dot)} aria-hidden="true" />
      {children}
    </span>
  )
}

export function SectionIcon({
  icon: Icon,
  className,
}: {
  icon: ComponentType<{ className?: string }>
  className?: string
}) {
  return (
    <span
      className={cn(
        "grid size-9 shrink-0 place-items-center rounded-full bg-muted text-foreground ring-1 ring-foreground/10",
        className,
      )}
    >
      <Icon className="size-4" aria-hidden="true" />
    </span>
  )
}
