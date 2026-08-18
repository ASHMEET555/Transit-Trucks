import { AppShell } from "@/components/AppShell"
import { ProfileProvider } from "@/lib/ProfileContext"
import { ThemeProvider } from "next-themes"
import localFont from "next/font/local"
import "./globals.css"
import { siteConfig } from "./siteConfig"

const geistSans = localFont({
  src: "./fonts/GeistVF.woff",
  variable: "--font-geist-sans",
  weight: "100 900",
})
const geistMono = localFont({
  src: "./fonts/GeistMonoVF.woff",
  variable: "--font-geist-mono",
  weight: "100 900",
})

export const metadata = {
  title: siteConfig.name,
  description: siteConfig.description,
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="h-full" suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} h-full bg-surface-0 antialiased text-gray-50`}
      >
        <ThemeProvider
          defaultTheme="system"
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
