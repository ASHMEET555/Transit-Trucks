"use client"

import { Button } from "@/components/Button"
import { cx, focusRing } from "@/lib/utils"
import { ChevronsUpDown } from "lucide-react"
import { useProfile } from "@/lib/ProfileContext"
import { DropdownUserProfile } from "./DropdownUserProfile"

export function UserProfile() {
  const { currentProfile } = useProfile()
  if (!currentProfile) return null

  return (
    <DropdownUserProfile>
      <Button
        aria-label="User settings"
        variant="ghost"
        className={cx(
          "group flex w-full items-center justify-between rounded-md px-1 py-2 text-sm font-medium text-foreground hover:bg-muted data-[state=open]:bg-muted",
          focusRing,
        )}
      >
        <span className="flex items-center gap-3">
          <span
            className="flex size-8 shrink-0 items-center justify-center rounded-full border border-border bg-card text-xs font-semibold text-muted-foreground"
            aria-hidden="true"
          >
            {currentProfile.initials}
          </span>
          <span className="truncate text-left text-sm font-semibold text-foreground">
            {currentProfile.role}
          </span>
        </span>
        <ChevronsUpDown
          className="size-4 shrink-0 text-muted-foreground group-hover:text-muted-foreground group-hover:dark:text-muted-foreground"
          aria-hidden="true"
        />
      </Button>
    </DropdownUserProfile>
  )
}
