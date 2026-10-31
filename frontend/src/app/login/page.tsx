"use client"

import Image from "next/image"
import { useRouter } from "next/navigation"
import { FormEvent, useEffect, useState } from "react"
import { ApiError, getMeRequest } from "@/lib/api"
import { memorySet } from "@/lib/memoryStore"
import { useProfile } from "@/lib/ProfileContext"
import { COMPANIES, WORKSPACES } from "@/config/workspaces"

const ROLES = [
  "Admin",
  "Fleet Manager",
  "Driver",
  "Safety Officer",
  "Financial Analyst",
] as const

type RoleType = (typeof ROLES)[number]

const SLIDES = [
  {
    image: "/images/hero-truck.png",
    alt: "TransitOps heavy transport vehicle on interstate highway",
    caption: "Real-time dispatch and fleet telemetry built for resilient freight operations.",
  },
  {
    image: "/images/rig.png",
    alt: "TransitOps highway freight rig in transit",
    caption: "Precision tracking, automated maintenance scheduling, and verified KPI integrity.",
  },
  {
    image: "/images/van.png",
    alt: "TransitOps electric logistics van at depot",
    caption: "Streamlined driver workflows and end-to-end operational visibility.",
  },
]

export default function LoginPage() {
  const router = useRouter()
  const { login } = useProfile()
  const [workspace, setWorkspace] = useState(WORKSPACES[0].id)
  const [company, setCompany] = useState(COMPANIES[0].id)
  const [expectedRole, setExpectedRole] = useState<RoleType>("Fleet Manager")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [currentSlide, setCurrentSlide] = useState(0)

  // Slideshow interval
  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentSlide((prev) => (prev + 1) % SLIDES.length)
    }, 5000)
    return () => clearInterval(timer)
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      await login(email.trim(), password, {
        workspace,
        company,
        expected_role: expectedRole,
      })

      // Always redirect according to the verified role from /api/v1/auth/me
      const me = await getMeRequest()
      // Session display metadata only (not an access boundary)
      memorySet("transitops.workspace", workspace)
      memorySet("transitops.company", company)
      if (me.role === "Driver") {
        router.push("/trips")
      } else {
        router.push("/dashboard")
      }
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          setError("Incorrect email or password")
        } else if (err.status === 403 && err.code === "ROLE_MISMATCH") {
          setError(`This account is not a ${expectedRole} account.`)
        } else if (err.status === 429) {
          setError(err.detail || "Account temporarily locked. Please check Retry-After and try again.")
        } else {
          setError(err.detail || "Authentication failed. Please verify credentials.")
        }
      } else {
        setError("Network or authentication error. Please try again.")
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted p-0 font-sans text-foreground antialiased md:p-6 lg:p-10 selection:bg-foreground selection:text-background">
      <main className="relative flex h-screen max-h-[960px] w-full max-w-[1240px] flex-col overflow-hidden border border-border bg-card shadow-2xl md:h-[840px] md:flex-row md:rounded-2xl">
        {/* Left Auth Form Pane */}
        <section
          data-purpose="auth-pane"
          className="telematics-grid-bg relative z-10 flex h-full w-full flex-col justify-between overflow-y-auto p-6 sm:p-10 md:w-1/2 lg:p-12"
        >
          {/* Subtle decorative watermark */}
          <svg className="bg-watermark-glyph text-foreground" fill="currentColor" viewBox="0 0 100 100" aria-hidden="true">
            <rect height="90" rx="3" width="6" x="47" y="5" />
            <rect height="6" rx="3" width="90" x="5" y="47" />
            <rect height="90" rx="3" transform="rotate(45 50 50)" width="6" x="47" y="5" />
            <rect height="90" rx="3" transform="rotate(-45 50 50)" width="6" x="47" y="5" />
          </svg>

          <div />

          <div className="mx-auto w-full max-w-[380px] py-4">
            <div className="mb-6 text-center">
              <div
                role="img" aria-label="TransitOps Logo"
                className="inline-flex size-11 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-md mb-4"
              >
                <svg className="size-6" fill="none" stroke="currentColor" strokeWidth="2.2" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M4 7l8 10 8-10" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </div>
              <h1 id="form-heading" className="text-[22px] font-bold tracking-tight text-foreground sm:text-[24px]">
                Sign in to TransitOps
              </h1>
              <p id="form-subheading" className="mx-auto mt-1 max-w-[300px] text-xs leading-relaxed text-muted-foreground sm:text-[13px]">
                Enterprise dispatch, realtime operations, and compliance telemetry.
              </p>
            </div>

            {error && (
              <div
                role="alert"
                className="mb-4 rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-xs font-semibold text-destructive animate-in fade-in"
              >
                {error}
              </div>
            )}

            <form aria-labelledby="form-heading" onSubmit={handleSubmit} className="space-y-4">
              {/* 1. Workspace Select */}
              <div className="space-y-1.5 text-left">
                <label htmlFor="workspace-select" className="block text-xs font-semibold tracking-wide text-foreground">
                  Workspace
                </label>
                <select
                  id="workspace-select"
                  name="workspace"
                  value={workspace}
                  onChange={(e) => setWorkspace(e.target.value)}
                  className="w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm text-foreground transition-all focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                >
                  {WORKSPACES.map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* 2. Company Select */}
              <div className="space-y-1.5 text-left">
                <label htmlFor="company-select" className="block text-xs font-semibold tracking-wide text-foreground">
                  Company
                </label>
                <select
                  id="company-select"
                  name="company"
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                  className="w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm text-foreground transition-all focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                >
                  {COMPANIES.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* 3. Role Segmented Buttons */}
              <div className="space-y-1.5 text-left">
                <span id="role-group-label" className="block text-xs font-semibold tracking-wide text-foreground">
                  Expected Role
                </span>
                <div
                  role="group"
                  aria-labelledby="role-group-label"
                  className="grid grid-cols-2 gap-1.5 sm:grid-cols-3"
                >
                  {ROLES.map((r) => {
                    const isSelected = expectedRole === r
                    return (
                      <button
                        key={r}
                        type="button"
                        aria-pressed={isSelected}
                        onClick={() => setExpectedRole(r)}
                        className={`rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-all ${
                          isSelected
                            ? "border-primary bg-primary text-primary-foreground shadow-sm"
                            : "border-border bg-muted text-muted-foreground hover:bg-muted/70"
                        }`}
                      >
                        {r}
                      </button>
                    )
                  })}
                </div>
              </div>

              {/* 4. Email */}
              <div className="space-y-1.5 text-left">
                <label htmlFor="fleet-email" className="block text-xs font-semibold tracking-wide text-foreground">
                  Email
                </label>
                <input
                  id="fleet-email"
                  name="email"
                  type="email"
                  required
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="name@transitops.com"
                  className="w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>

              {/* 5. Password */}
              <div className="space-y-1.5 text-left">
                <div className="flex items-center justify-between">
                  <label htmlFor="fleet-password" className="block text-xs font-semibold tracking-wide text-foreground">
                    Password
                  </label>
                  <span className="text-[11px] text-muted-foreground">
                    Contact your administrator
                  </span>
                </div>
                <input
                  id="fleet-password"
                  name="password"
                  type="password"
                  required
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••••"
                  className="w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>

              {/* 6. Submit Button */}
              <div className="pt-2">
                <button
                  id="btn-submit"
                  type="submit"
                  disabled={loading}
                  className="w-full cursor-pointer rounded-xl bg-primary py-3 px-4 text-[13px] font-semibold text-primary-foreground shadow-md transition-all hover:bg-primary/90 hover:shadow-lg active:scale-[0.99] disabled:opacity-50"
                >
                  {loading ? "Signing in…" : "Sign in"}
                </button>
              </div>
            </form>
          </div>

          {/* Footer note: D4 */}
          <footer className="pt-4 pb-2 text-center" data-purpose="auth-pane-footer">
            <p className="text-xs text-muted-foreground">
              Accounts are created by your administrator.
            </p>
          </footer>
        </section>

        {/* Right Slideshow Pane */}
        <section
          data-purpose="transit-showcase"
          className="relative hidden h-full w-1/2 flex-col justify-end overflow-hidden bg-foreground p-8 md:flex"
        >
          <div className="absolute inset-0 z-0 size-full overflow-hidden" id="slides-wrapper">
            {SLIDES.map((slide, index) => (
              <article
                key={slide.image}
                data-index={index}
                className={`carousel-slide absolute inset-0 size-full transition-opacity duration-700 ease-in-out ${
                  index === currentSlide ? "opacity-100" : "pointer-events-none opacity-0"
                }`}
              >
                <Image
                  src={slide.image}
                  alt={slide.alt}
                  fill
                  priority={index === 0}
                  className="object-cover object-center brightness-[0.75] contrast-[1.05]"
                  sizes="(min-width: 768px) 50vw, 100vw"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-black/85 via-black/20 to-transparent" />
              </article>
            ))}
          </div>

          {/* Slide Caption Box */}
          <div className="relative z-10 max-w-md">
            <div className="rounded-2xl border border-background/10 bg-foreground/40 p-6 backdrop-blur-md">
              <span className="inline-flex items-center gap-1.5 rounded-full bg-signal px-2.5 py-0.5 text-[11px] font-semibold text-signal-foreground">
                TransitOps Fleet
              </span>
              <p className="mt-3 text-sm font-medium leading-relaxed text-background/90">
                {SLIDES[currentSlide].caption}
              </p>
              <div className="mt-4 flex gap-1.5">
                {SLIDES.map((_, i) => (
                  <button
                    key={i}
                    type="button"
                    aria-label={`Go to slide ${i + 1}`}
                    onClick={() => setCurrentSlide(i)}
                    className={`h-1.5 rounded-full transition-all ${
                      i === currentSlide ? "w-6 bg-signal" : "w-1.5 bg-background/30"
                    }`}
                  />
                ))}
              </div>
            </div>
          </div>
        </section>
      </main>
    </div>
  )
}
