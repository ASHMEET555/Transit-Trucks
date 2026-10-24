"use client"

import { Button } from "@/components/Button"
import { Input } from "@/components/Input"
import { ApiError } from "@/lib/api"
import { useProfile } from "@/lib/ProfileContext"
import { Logo } from "../../../public/Logo"
import Link from "next/link"
import { FormEvent, useState } from "react"
import { useTheme } from "next-themes"
import { ArrowLeft, Moon, Sun, Monitor, Truck } from "lucide-react"

export default function LoginPage() {
  const { login } = useProfile()
  const { theme, setTheme } = useTheme()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(email, password)
      window.location.href = "/dashboard"
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : "Login failed")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-visual">
        <div className="auth-visual-content">
          <Link href="/" className="auth-back-link">
            <ArrowLeft className="size-4" />
            Back to home
          </Link>
          <div className="auth-visual-icon">
            <Truck className="size-10 text-cyan-400" />
          </div>
          <h2>Fleet command center</h2>
          <p>
            Sign in to dispatch trips, track trucks on the live map, manage
            maintenance, and view analytics — based on your role.
          </p>
          <ul className="auth-visual-list">
            <li>Live map for dispatched trips</li>
            <li>Smart vehicle & driver suggestions</li>
            <li>Role-based dashboards</li>
          </ul>
        </div>
      </div>

      <div className="auth-form-panel">
        <div className="auth-card">
          <div className="mb-6 flex justify-end">
            <button
              onClick={() => {
                if (theme === "light") setTheme("dark")
                else if (theme === "dark") setTheme("system")
                else setTheme("light")
              }}
              className="rounded-lg p-2 text-gray-400 hover:bg-white/5"
              title="Toggle theme"
              type="button"
            >
              {theme === "light" && <Moon className="size-5" />}
              {theme === "dark" && <Sun className="size-5" />}
              {theme === "system" && <Monitor className="size-5" />}
            </button>
          </div>

          <div className="auth-brand">
            <span className="auth-brand-icon">
              <Logo className="size-7 text-cyan-400" />
            </span>
            <div>
              <h1>Welcome back</h1>
              <p className="auth-subtitle">Log in to TransitOps</p>
            </div>
          </div>

          <form onSubmit={onSubmit} className="space-y-4">
            <div>
              <label className="mb-1.5 block text-sm text-gray-300">
                Email
              </label>
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="name@transitops.com"
                required
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm text-gray-300">
                Password
              </label>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                required
              />
            </div>
            {error && (
              <p className="rounded-md bg-red-500/10 px-3 py-2 text-sm text-red-400">
                {error}
              </p>
            )}
            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "Signing in..." : "Sign in"}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-400">
            Need an account? Contact your system Administrator.
          </p>
        </div>
      </div>
    </div>
  )
}
