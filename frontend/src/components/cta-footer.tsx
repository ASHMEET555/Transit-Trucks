import Image from "next/image"
import Link from "next/link"
import { ArrowUpRight } from "lucide-react"
import { Logo } from "@/components/site-header"

export function CtaFooter() {
  return (
    <>
      <section id="demo" aria-labelledby="demo-title" className="relative overflow-hidden rounded-[40px] bg-primary text-primary-foreground">
        <Image
          src="/images/depot.png"
          alt=""
          fill
          sizes="100vw"
          className="object-cover opacity-30 mix-blend-luminosity"
        />
        <div className="relative grid gap-10 p-8 sm:p-12 lg:grid-cols-[1.3fr_1fr] lg:items-end lg:p-16">
          <h2 id="demo-title" className="text-4xl font-semibold leading-[1.02] tracking-[-0.03em] text-balance sm:text-6xl">
            Every vehicle, <span className="font-serif font-normal italic">accounted for.</span>
          </h2>
          <div>
            <p className="max-w-sm leading-relaxed text-primary-foreground/80">
              Enterprise fleet operations, verified telemetry, and strict role security out of the box.
            </p>
            <Link
              href="/login"
              className="mt-6 inline-flex items-center gap-2 rounded-full bg-sand px-6 py-3.5 text-sm font-medium text-foreground transition-transform hover:-translate-y-0.5"
            >
              Sign in to TransitOps <ArrowUpRight className="size-4" aria-hidden="true" />
            </Link>
          </div>
        </div>
      </section>
      <footer className="flex flex-col items-start justify-between gap-4 py-10 text-sm text-muted-foreground sm:flex-row sm:items-center">
        <Logo />
        <p>© 2026 TransitOps. Built for the people who keep things moving.</p>
      </footer>
    </>
  )
}
