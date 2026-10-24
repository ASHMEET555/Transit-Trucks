"use client"

import { useProfile } from "@/lib/ProfileContext"
import { usePathname, useRouter } from "next/navigation"
import { useEffect } from "react"
import { SidebarProvider, SidebarTrigger } from "@/components/Sidebar"
import { AppSidebar } from "@/components/ui/navigation/AppSidebar"

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { isAuthenticated, isLoaded } = useProfile()
  const isPublicPage = pathname === "/" || pathname === "/login"
  const isAuthPage = pathname === "/login"

  useEffect(() => {
    if (!isLoaded) return
    if (!isPublicPage && !isAuthenticated) {
      router.replace("/login")
    }
    if (isAuthPage && isAuthenticated) {
      router.replace("/dashboard")
    }
  }, [isAuthenticated, isLoaded, isPublicPage, isAuthPage, router])

  if (!isLoaded) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <div className="size-8 animate-spin rounded-full border-2 border-cyan-500 border-t-transparent" />
      </div>
    )
  }

  if (isPublicPage) {
    return <>{children}</>
  }

  if (!isAuthenticated) {
    return null
  }

  return (
    <SidebarProvider defaultOpen={true}>
      <AppSidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <header className="flex h-14 items-center gap-4 border-b border-gray-800 bg-gray-950 px-6">
          <SidebarTrigger className="-ml-1" />
        </header>
        <main className="flex-1 overflow-auto bg-gray-950 p-6">
          {children}
        </main>
      </div>
    </SidebarProvider>
  )
}
