"use client"

import { useState } from "react"
import { useProfile } from "@/lib/ProfileContext"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/Input"
import { apiRequest, ApiError } from "@/lib/api"
import { User, KeyRound, Shield, LogOut } from "lucide-react"

export default function SettingsPage() {
  const { currentProfile, logout } = useProfile()
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)

    if (newPassword !== confirmPassword) {
      setError("New password and confirmation do not match.")
      return
    }

    if (newPassword.length < 8) {
      setError("Password must be at least 8 characters long.")
      return
    }

    setLoading(true)
    try {
      await apiRequest("/api/v1/auth/change-password", {
        method: "POST",
        body: {
          old_password: oldPassword,
          new_password: newPassword,
        },
      })
      setSuccess("Password updated successfully.")
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err.detail)
      } else {
        setError("Failed to change password. Please check your current password.")
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="p-6 md:p-8 space-y-8 max-w-4xl mx-auto">
      <div className="border-b border-border/60 pb-5">
        <h1 className="text-2xl font-bold tracking-tight text-foreground font-serif">Account & Security</h1>
        <p className="text-xs text-muted-foreground mt-1">
          Profile verification, session credentials, and password rotation.
        </p>
      </div>

      {/* Profile Card */}
      <div className="rounded-xl border border-border/80 bg-card p-6 shadow-xs space-y-4">
        <div className="flex items-center gap-3 pb-4 border-b border-border/60">
          <div className="size-10 rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold">
            <User className="size-5" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">Operator Profile</h2>
            <p className="text-xs text-muted-foreground">Authenticated enterprise session</p>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
          <div>
            <label className="text-muted-foreground block mb-1">Email Address</label>
            <div className="font-mono text-foreground font-medium p-2.5 rounded-lg border border-border bg-neutral-50/50">
              {currentProfile?.email || "—"}
            </div>
          </div>
          <div>
            <label className="text-muted-foreground block mb-1">Access Role</label>
            <div className="font-semibold text-primary p-2.5 rounded-lg border border-border bg-neutral-50/50 flex items-center gap-1.5">
              <Shield className="size-3.5" />
              <span>{currentProfile?.role || "—"}</span>
            </div>
          </div>
        </div>

        <div className="pt-2 flex justify-end">
          <Button variant="outline" size="sm" onClick={logout} className="gap-2 text-xs text-destructive hover:bg-destructive/10 border-destructive/30">
            <LogOut className="size-3.5" /> Sign Out
          </Button>
        </div>
      </div>

      {/* Change Password */}
      <div className="rounded-xl border border-border/80 bg-card p-6 shadow-xs space-y-4">
        <div className="flex items-center gap-3 pb-4 border-b border-border/60">
          <div className="size-10 rounded-full bg-signal/10 text-signal flex items-center justify-center font-bold">
            <KeyRound className="size-5" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">Change Password</h2>
            <p className="text-xs text-muted-foreground">Rotate your account password</p>
          </div>
        </div>

        {error && (
          <div className="rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-xs font-semibold text-destructive">
            {error}
          </div>
        )}

        {success && (
          <div className="rounded-xl border border-primary/20 bg-primary/10 p-3 text-xs font-semibold text-primary">
            {success}
          </div>
        )}

        <form onSubmit={handleChangePassword} className="space-y-4 max-w-md">
          <div>
            <label className="block text-xs font-medium text-foreground mb-1.5">Current Password</label>
            <Input
              type="password"
              required
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              placeholder="••••••••••"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-foreground mb-1.5">New Password</label>
            <Input
              type="password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="Minimum 8 characters"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-foreground mb-1.5">Confirm New Password</label>
            <Input
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Re-enter new password"
            />
          </div>

          <Button type="submit" disabled={loading} size="sm" className="bg-primary text-primary-foreground hover:bg-primary/90">
            {loading ? "Updating Password..." : "Update Password"}
          </Button>
        </form>
      </div>
    </div>
  )
}
