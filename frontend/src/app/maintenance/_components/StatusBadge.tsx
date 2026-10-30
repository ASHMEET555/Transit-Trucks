import { DotPill } from "./chrome"

export function StatusBadge({ status }: { status: string }) {
  if (status === "Open") {
    return (
      <DotPill dot="bg-primary-foreground" pill="bg-primary text-primary-foreground">
        Open
      </DotPill>
    )
  }
  if (status === "Closed") {
    return (
      <DotPill dot="bg-chart-5" pill="bg-sage text-foreground">
        Closed
      </DotPill>
    )
  }
  return (
    <DotPill dot="bg-muted-foreground" pill="bg-muted text-muted-foreground">
      {status}
    </DotPill>
  )
}
