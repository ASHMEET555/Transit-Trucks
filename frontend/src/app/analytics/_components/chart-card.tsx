"use client"

import type { LucideIcon } from "lucide-react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
import { IconBadge } from "../../dashboard/_components/states"

export function ChartCard({
  icon,
  title,
  description,
  filters,
  onDownload,
  downloadLabel,
  children,
}: {
  icon: LucideIcon
  title: string
  description: string
  filters?: React.ReactNode
  onDownload?: () => void
  downloadLabel?: string
  children: React.ReactNode
}) {
  return (
    <CardShell
      as="section"
      aria-label={title}
      tone="paper"
      className="motion-reduce:transition-none motion-reduce:hover:translate-y-0"
    >
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex items-center gap-3">
          <IconBadge icon={icon} />
          <div>
            <h2 className="text-[15px] font-medium tracking-tight text-foreground">
              {title}
            </h2>
            <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {filters}
          {onDownload && (
            <Button variant="secondary" onClick={onDownload}>
              {downloadLabel ?? "Download CSV"}
            </Button>
          )}
        </div>
      </div>
      {children}
    </CardShell>
  )
}
