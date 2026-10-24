"use client"

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react"
import { getMeRequest, loginRequest, logoutRequest } from "@/lib/api"

export interface Profile {
  id: string
  name: string
  email: string
  initials: string
  role: string
}

interface ProfileContextType {
  currentProfile: Profile | null
  token: string | null
  isAuthenticated: boolean
  isLoaded: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
}

const ProfileContext = createContext<ProfileContextType | undefined>(undefined)

function profileFromUser(user: { id?: string; email: string; role: string }): Profile {
  const local = user.email.split("@")[0] || "User"
  return {
    id: user.id || user.email,
    name: local,
    email: user.email,
    initials: (local.length >= 2 ? local.slice(0, 2) : local).toUpperCase(),
    role: user.role,
  }
}

export function ProfileProvider({ children }: { children: React.ReactNode }) {
  const [currentProfile, setCurrentProfile] = useState<Profile | null>(null)
  const [isLoaded, setIsLoaded] = useState(false)

  useEffect(() => {
    getMeRequest()
      .then((user) => {
        setCurrentProfile(profileFromUser(user))
      })
      .catch(() => {
        setCurrentProfile(null)
      })
      .finally(() => {
        setIsLoaded(true)
      })
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    await loginRequest(email.trim(), password)
    const user = await getMeRequest()
    setCurrentProfile(profileFromUser(user))
  }, [])

  const logout = useCallback(async () => {
    try {
      await logoutRequest()
    } catch {
      // ignore
    }
    setCurrentProfile(null)
    if (typeof window !== "undefined") {
      window.location.href = "/login"
    }
  }, [])

  const value = useMemo(
    () => ({
      currentProfile,
      token: null,
      isAuthenticated: Boolean(currentProfile),
      isLoaded,
      login,
      logout,
    }),
    [currentProfile, isLoaded, login, logout],
  )

  return (
    <ProfileContext.Provider value={value}>{children}</ProfileContext.Provider>
  )
}

export function useProfile() {
  const context = useContext(ProfileContext)
  if (!context) {
    throw new Error("useProfile must be used within a ProfileProvider")
  }
  return context
}
