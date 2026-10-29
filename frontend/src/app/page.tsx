import { SiteHeader } from "@/components/site-header"
import { Hero } from "@/components/hero"
import { CoverDashboard } from "@/components/cover-dashboard"
import { CardShowcase } from "@/components/card-showcase"
import { CtaFooter } from "@/components/cta-footer"

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-canvas px-4 text-foreground selection:bg-primary selection:text-primary-foreground sm:px-6 lg:px-10">
      <div className="mx-auto max-w-[1440px]">
        <SiteHeader />
        <main>
          <Hero />
          <CoverDashboard />
          <CardShowcase />
          <CtaFooter />
        </main>
      </div>
    </div>
  )
}
