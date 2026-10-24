import Link from "next/link"
import {
  ArrowRight,
  BarChart3,
  CheckCircle2,
  ClipboardList,
  Fuel,
  MapPin,
  Radio,
  Shield,
  Truck,
  Users,
  Wrench,
} from "lucide-react"
import { Logo } from "../../public/Logo"

const painPoints = [
  {
    title: "No single source of truth",
    description:
      "Vehicle status, driver assignments, and trip data live in spreadsheets and phone calls — not one system.",
  },
  {
    title: "Blind spots on the road",
    description:
      "Dispatchers can't see where trucks are or whether loads will arrive on time until it's too late.",
  },
  {
    title: "Costs you can't explain",
    description:
      "Fuel, maintenance, and revenue are tracked separately, so fleet ROI stays a guess.",
  },
]

const features = [
  {
    icon: MapPin,
    title: "Live fleet map",
    description:
      "Track dispatched trucks on real road routes. See origin, destination, and active trips at a glance.",
  },
  {
    icon: ClipboardList,
    title: "Trip dispatch & planning",
    description:
      "Create trips, assign vehicles and drivers, and get smart suggestions based on capacity and safety scores.",
  },
  {
    icon: Truck,
    title: "Fleet & driver management",
    description:
      "Manage registrations, load capacity, license expiry, and on-duty status from one place.",
  },
  {
    icon: Wrench,
    title: "Maintenance workflows",
    description:
      "Open and close work orders, track vehicles in the shop, and keep availability accurate.",
  },
  {
    icon: Fuel,
    title: "Fuel & expense tracking",
    description:
      "Log operational costs per trip and vehicle so finance teams see the full cost picture.",
  },
  {
    icon: BarChart3,
    title: "Executive analytics",
    description:
      "ROI, fuel efficiency, driver safety, odometer trends, and route capacity — filterable dashboards.",
  },
  {
    icon: Radio,
    title: "Real-time updates",
    description:
      "Instant refresh when trips dispatch, complete, or maintenance events happen — no manual reload.",
  },
  {
    icon: Shield,
    title: "Role-based access",
    description:
      "Fleet managers, drivers, safety officers, and finance each get the right tools — nothing more.",
  },
]

const steps = [
  {
    step: "01",
    title: "Create your account",
    description:
      "Sign up as a fleet manager, driver, safety officer, or financial analyst. Each role sees tailored features.",
  },
  {
    step: "02",
    title: "Run your operations",
    description:
      "Add vehicles, assign drivers, dispatch trips, and log maintenance and expenses from the command center.",
  },
  {
    step: "03",
    title: "Track and improve",
    description:
      "Use the live map and analytics to cut empty miles, improve utilization, and protect margins.",
  },
]

const audiences = [
  {
    icon: Truck,
    role: "Trucking & logistics companies",
    points: [
      "Centralize fleet, trips, and drivers",
      "Live visibility on dispatched loads",
      "Reduce manual dispatch overhead",
    ],
  },
  {
    icon: Users,
    role: "Fleet managers & dispatchers",
    points: [
      "Smart vehicle & driver suggestions",
      "Filter trips by route and asset",
      "Real-time status across the fleet",
    ],
  },
  {
    icon: BarChart3,
    role: "Finance & operations leaders",
    points: [
      "ROI and fuel efficiency reporting",
      "Revenue vs operational cost view",
      "Data-backed fleet decisions",
    ],
  },
]

export default function LandingPage() {
  return (
    <div className="landing-page">
      <header className="landing-nav">
        <div className="landing-nav-inner">
          <Link href="/" className="landing-brand">
            <span className="landing-brand-icon">
              <Logo className="size-6 text-cyan-400" />
            </span>
            TransitOps
          </Link>
          <nav className="landing-nav-links">
            <a href="#features">Features</a>
            <a href="#for-trucking">For trucking</a>
            <a href="#how-it-works">How it works</a>
            <Link href="/login" className="landing-login-link">
              Log in
            </Link>
            <Link href="/login" className="landing-cta-sm">
              Launch platform
              <ArrowRight className="size-4" />
            </Link>
          </nav>
        </div>
      </header>

      <section className="landing-hero landing-hero-marketing">
        <div className="landing-hero-grid landing-hero-grid-single">
          <div className="landing-hero-copy landing-hero-centered">
            <p className="landing-eyebrow">Built for trucking & freight operators</p>
            <h1>
              Run your fleet with
              <span className="landing-gradient-text"> one command center</span>
            </h1>
            <p className="landing-lead landing-lead-centered">
              TransitOps helps trucking companies dispatch loads, track vehicles
              on real roads, manage maintenance, and understand costs — so your
              team spends less time chasing updates and more time moving freight.
            </p>
            <div className="landing-hero-actions landing-hero-actions-centered">
              <Link href="/login" className="landing-cta">
                Launch platform
                <ArrowRight className="size-4" />
              </Link>
              <Link href="/login" className="landing-cta-ghost">
                Log in to dashboard
              </Link>
            </div>
          </div>

          <div className="landing-product-preview" aria-hidden="true">
            <div className="preview-window">
              <div className="preview-titlebar">
                <span />
                <span />
                <span />
                <p>TransitOps · Fleet command center</p>
              </div>
              <div className="preview-body">
                <div className="preview-kpis">
                  <div className="preview-kpi">
                    <span>Active trips</span>
                    <strong>24</strong>
                  </div>
                  <div className="preview-kpi">
                    <span>Utilization</span>
                    <strong>87%</strong>
                  </div>
                  <div className="preview-kpi">
                    <span>On-time</span>
                    <strong>94%</strong>
                  </div>
                </div>
                <div className="preview-rows">
                  <div className="preview-row">
                    <span className="preview-dot cyan" />
                    TX-401 · Houston → Dallas · In transit
                  </div>
                  <div className="preview-row">
                    <span className="preview-dot blue" />
                    CA-118 · LA → San Diego · Dispatched
                  </div>
                  <div className="preview-row">
                    <span className="preview-dot violet" />
                    IL-290 · Chicago → Indianapolis · Loading
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="landing-section landing-problems">
        <div className="landing-section-header">
          <p className="landing-eyebrow">The problem</p>
          <h2>Running a trucking company shouldn&apos;t feel like chaos</h2>
          <p>
            Most fleets juggle disconnected tools. TransitOps replaces the
            patchwork with one platform your whole team can trust.
          </p>
        </div>
        <div className="landing-problems-grid">
          {painPoints.map((item) => (
            <article key={item.title} className="landing-problem-card">
              <h3>{item.title}</h3>
              <p>{item.description}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="landing-section" id="features">
        <div className="landing-section-header">
          <p className="landing-eyebrow">Platform features</p>
          <h2>Everything your fleet team needs</h2>
          <p>
            From first dispatch to monthly reporting — one product, no bolt-ons.
          </p>
        </div>
        <div className="landing-feature-grid">
          {features.map((feature) => (
            <article key={feature.title} className="landing-feature-card">
              <span className="landing-feature-icon">
                <feature.icon className="size-5" />
              </span>
              <h3>{feature.title}</h3>
              <p>{feature.description}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="landing-section landing-audience" id="for-trucking">
        <div className="landing-section-header">
          <p className="landing-eyebrow">Who it&apos;s for</p>
          <h2>Designed for modern freight operations</h2>
          <p>
            Whether you run 20 trucks or 200, TransitOps scales with your
            workflows and your team structure.
          </p>
        </div>
        <div className="landing-audience-grid">
          {audiences.map((group) => (
            <article key={group.role} className="landing-audience-card">
              <span className="landing-feature-icon">
                <group.icon className="size-5" />
              </span>
              <h3>{group.role}</h3>
              <ul>
                {group.points.map((point) => (
                  <li key={point}>
                    <CheckCircle2 className="size-4 shrink-0 text-cyan-400" />
                    {point}
                  </li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </section>

      <section className="landing-section" id="how-it-works">
        <div className="landing-section-header">
          <p className="landing-eyebrow">How it works</p>
          <h2>Up and running in minutes</h2>
          <p>
            Create an account, sign in, and access features based on your role —
            no separate apps, no complicated setup.
          </p>
        </div>
        <div className="landing-steps">
          {steps.map((item) => (
            <article key={item.step} className="landing-step-card">
              <span className="landing-step-num">{item.step}</span>
              <h3>{item.title}</h3>
              <p>{item.description}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="landing-cta-band">
        <div>
          <h2>Ready to modernize your fleet operations?</h2>
          <p>
            Deploy this landing page publicly. When customers log in or register,
            they enter the full TransitOps platform with role-based access.
          </p>
        </div>
        <div className="landing-cta-band-actions">
          <Link href="/login" className="landing-cta">
            Access dashboard
            <ArrowRight className="size-4" />
          </Link>
          <Link href="/login" className="landing-cta-ghost">
            Log in
          </Link>
        </div>
      </section>

      <footer className="landing-footer">
        <div className="landing-footer-brand">
          <Logo className="size-5 text-cyan-400" />
          <span>TransitOps</span>
        </div>
        <span>© 2026 TransitOps · Fleet management for trucking companies</span>
        <div className="landing-footer-links">
          <Link href="/login">Log in</Link>
          <Link href="/login">Register</Link>
        </div>
      </footer>
    </div>
  )
}
