"use client"

import { RiAddLine } from "@remixicon/react"
import { Button } from "@/components/Button"
import { CardShell } from "@/components/fleet/card-shell"
import { CardHead } from "./chrome"
import { StatusBadge } from "./StatusBadge"
import type { MaintenanceLog } from "./types"

interface OpenQueueProps {
  logs: MaintenanceLog[]
  canManage: boolean
  confirmId: number | null
  closingId: number | null
  onAskClose: (id: number) => void
  onCancelClose: () => void
  onConfirmClose: (id: number) => void
  onOpenOrder: () => void
}

function formatCost(cost: number): string {
  return `$${Number(cost).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function OpenQueue({
  logs,
  canManage,
  confirmId,
  closingId,
  onAskClose,
  onCancelClose,
  onConfirmClose,
  onOpenOrder,
}: OpenQueueProps) {
  if (logs.length === 0) {
    return (
      <div className="rounded-[28px] border border-dashed border-border bg-card p-8 text-center">
        <p className="text-sm font-medium text-foreground">No open work orders</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Every vehicle is out of the shop. Open a work order when one comes in.
        </p>
        {canManage && (
          <Button className="mt-4" onClick={onOpenOrder}>
            <RiAddLine className="mr-1.5 size-4" aria-hidden="true" />
            Open work order
          </Button>
        )}
      </div>
    )
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Open work orders">
      {logs.map((log) => {
        const confirming = confirmId === log.id
        const closing = closingId === log.id
        return (
          <CardShell
            as="li"
            key={log.id}
            tone="paper"
            className="rounded-3xl p-5"
          >
            <CardHead
              title={`Work order #${log.id}`}
              action={<StatusBadge status={log.status} />}
            />
            <p className="mt-3 font-serif text-lg font-medium tracking-tight">
              {log.vehicle_reg}
            </p>
            <p className="mt-1 text-sm opacity-70">{log.description}</p>
            <dl className="mt-3 space-y-1 text-sm">
              <div className="flex justify-between gap-2">
                <dt className="opacity-60">Estimated cost</dt>
                <dd className="font-mono tabular-nums">
                  {formatCost(log.cost)}
                </dd>
              </div>
              <div className="flex justify-between gap-2">
                <dt className="opacity-60">Opened</dt>
                <dd className="font-mono tabular-nums">
                  {log.start_date}
                </dd>
              </div>
            </dl>
            {canManage && (
              <div className="mt-4 border-t border-current/10 pt-3">
                {confirming ? (
                  <div role="group" aria-label={`Confirm closing work order ${log.id}`}>
                    <p className="text-sm">
                      Close work order #{log.id} and return {log.vehicle_reg} to
                      service?
                    </p>
                    <div className="mt-2 flex gap-2">
                      <Button
                        variant="destructive"
                        isLoading={closing}
                        loadingText="Closing"
                        onClick={() => onConfirmClose(log.id)}
                      >
                        Confirm close
                      </Button>
                      <Button
                        variant="secondary"
                        disabled={closing}
                        onClick={onCancelClose}
                      >
                        Keep open
                      </Button>
                    </div>
                  </div>
                ) : (
                  <Button onClick={() => onAskClose(log.id)}>
                    Close work order
                  </Button>
                )}
              </div>
            )}
          </CardShell>
        )
      })}
    </ul>
  )
}
