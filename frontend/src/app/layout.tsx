import { AppShell } from "@/components/AppShell"
import { ProfileProvider } from "@/lib/ProfileContext"
import { ThemeProvider } from "next-themes"
import { Inter, Newsreader } from "next/font/google"
import "./globals.css"
import { siteConfig } from "./siteConfig"

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
})

const newsreader = Newsreader({
  subsets: ["latin"],
  variable: "--font-newsreader",
  style: ["normal", "italic"],
  display: "swap",
})

export const metadata = {
  title: "TransitOps Telematics & Fleet Intelligence",
  description: "Enterprise fleet intelligence, real-time dispatch, and telemetry.",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="h-full" suppressHydrationWarning>
      <body
        className={`${inter.variable} ${newsreader.variable} h-full bg-canvas font-sans text-foreground antialiased`}
      >
        <ThemeProvider
          defaultTheme="light"
          disableTransitionOnChange
          attribute="class"
        >
          <ProfileProvider>
            <AppShell>{children}</AppShell>
          </ProfileProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
