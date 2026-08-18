"use client"

import { useProfile } from "@/lib/ProfileContext"
import { usePathname, useRouter } from "next/navigation"
import { useEffect } from "react"
import { SidebarProvider, SidebarTrigger } from "@/components/Sidebar"
import { AppSidebar } from "@/components/ui/navigation/AppSidebar"
import { Breadcrumbs } from "@/components/ui/navigation/Breadcrumbs"

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { isAuthenticated, isLoaded } = useProfile()
  const isPublicPage =
    pathname === "/" || pathname === "/login" || pathname === "/register"
  const isAuthPage = pathname === "/login" || pathname === "/register"

  useEffect(() => {
    if (!isLoaded) return
    if (!isPublicPage && !isAuthenticated) {
      router.replace("/login")
    }
    if (isAuthPage && isAuthenticated) {
      router.replace("/dashboard")
    }
  }, [isLoaded, isPublicPage, isAuthPage, isAuthenticated, router])

  if (!isLoaded) {
    return (
      <div className="flex min-h-screen items-center justify-center text-sm text-gray-500">
        Loading...
      </div>
    )
  }

  if (isPublicPage && !isAuthenticated) {
    return <>{children}</>
  }

  if (!isAuthenticated) {
    return null
  }

  return (
    <SidebarProvider defaultOpen={true}>
      <AppSidebar />
      <div className="w-full">
        <header className="app-main-header sticky top-0 z-10 flex h-16 shrink-0 items-center gap-2 px-4">
          <SidebarTrigger className="-ml-1" />
          <div className="mr-2 h-4 w-px bg-gray-700" />
          <Breadcrumbs />
        </header>
        <main className="app-main-content">{children}</main>
      </div>
    </SidebarProvider>
  )
}
