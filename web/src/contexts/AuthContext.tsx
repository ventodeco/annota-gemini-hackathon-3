import { createContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import type { User } from '@/lib/types'
import { getUserProfile } from '@/lib/api'
import { getAuthToken, clearAuthToken } from '@/lib/api'

export interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  login: () => void
  logout: () => void
  refreshUser: () => Promise<void>
}

// eslint-disable-next-line react-refresh/only-export-components
export const AuthContext = createContext<AuthContextType | undefined>(undefined)

async function loadUserProfile(): Promise<User | null> {
  if (!getAuthToken()) return null
  try {
    return await getUserProfile()
  } catch {
    clearAuthToken()
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const refreshUser = useCallback(async () => {
    const userData = await loadUserProfile()
    setUser(userData)
    setIsLoading(false)
  }, [])

  useEffect(() => {
    let cancelled = false
    void loadUserProfile().then((userData) => {
      if (cancelled) return
      setUser(userData)
      setIsLoading(false)
    })
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(() => {
    // Login is handled by opening Google OAuth popup
    // The popup will close and trigger a refresh
  }, [])

  const logout = useCallback(() => {
    clearAuthToken()
    setUser(null)
  }, [])

  const value: AuthContextType = {
    user,
    isLoading,
    isAuthenticated: !!user,
    login,
    logout,
    refreshUser,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
