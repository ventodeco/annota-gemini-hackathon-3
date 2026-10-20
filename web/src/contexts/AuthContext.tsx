import { createContext, useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useAuth as useClerkAuth, useClerk, useUser } from '@clerk/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteAccount, getUserProfile } from '@/lib/api'
import {
  ACCOUNT_DELETED_MESSAGE,
  clearUserScopedCaches,
  setUnauthorizedHandler,
  tokenSession,
  type TokenSession,
} from '@/lib/authSession'
import type { PreferredLanguage } from '@/lib/types'

/** Identity from Clerk combined with ANNOTA-owned preferences. */
export interface AuthUser {
  email: string
  avatarUrl?: string
  preferredLanguage: PreferredLanguage
}

export interface AuthContextType {
  user: AuthUser | null
  /** True until Clerk, the cache-owner check and the profile request settle. */
  isLoading: boolean
  /** Clerk reports an active session. */
  isSignedIn: boolean
  /** Signed in and the ANNOTA profile loaded. */
  isAuthenticated: boolean
  /** Error from loading the ANNOTA profile (e.g. legacy link required). */
  profileError: Error | null
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

// eslint-disable-next-line react-refresh/only-export-components
export const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const { isLoaded, isSignedIn, userId } = useClerkAuth()
  const { user: clerkUser } = useUser()
  const clerk = useClerk()
  const queryClient = useQueryClient()

  // The account whose data the query/browser caches may hold. Queries keys are
  // not user-scoped, so the app stays in a loading state until caches belong
  // to the current Clerk user.
  // null means the caches hold no account's data (initial load or cleared).
  const [cacheOwner, setCacheOwner] = useState<string | null>(null)
  const currentUserId = userId ?? null
  if (isLoaded && cacheOwner === null && currentUserId !== null) {
    // First signed-in user on empty caches: nothing to clear.
    setCacheOwner(currentUserId)
  }
  const cacheReady = isLoaded && cacheOwner === currentUserId

  useEffect(() => {
    if (!isLoaded || cacheOwner === null || cacheOwner === currentUserId) return
    // Sign-out or account switch: drop the previous account's cached data
    // before anything renders for the next state.
    let cancelled = false
    void clearUserScopedCaches(queryClient).then(() => {
      if (!cancelled) setCacheOwner(currentUserId)
    })
    return () => {
      cancelled = true
    }
  }, [isLoaded, cacheOwner, currentUserId, queryClient])

  const signedIn = isLoaded && Boolean(isSignedIn)
  const profileQuery = useQuery({
    queryKey: ['user', currentUserId],
    queryFn: getUserProfile,
    enabled: cacheReady && signedIn,
    retry: false,
  })

  const isSigningOut = useRef(false)
  const logout = useCallback(async () => {
    if (isSigningOut.current) return
    isSigningOut.current = true
    try {
      await clearUserScopedCaches(queryClient)
      await clerk.signOut({ redirectUrl: '/login' })
    } finally {
      isSigningOut.current = false
    }
  }, [clerk, queryClient])

  // Ends one specific session after an API 401. Ownership is re-checked after
  // every await so an account that signed in meanwhile is left untouched.
  const endSession = useCallback(
    async (session: TokenSession) => {
      const isStillCurrent = () =>
        clerk.user?.id === session.userId &&
        (session.sessionId === null || clerk.session?.id === session.sessionId)
      if (isSigningOut.current || !isStillCurrent()) return
      isSigningOut.current = true
      try {
        await clearUserScopedCaches(queryClient)
        if (!isStillCurrent()) return
        await clerk.signOut({
          redirectUrl: '/login',
          ...(session.sessionId ? { sessionId: session.sessionId } : {}),
        })
      } finally {
        isSigningOut.current = false
      }
    },
    [clerk, queryClient],
  )

  const isFinishingDeletion = useRef(false)
  useEffect(() => {
    setUnauthorizedHandler(({ message, token }) => {
      // Only react to failures from the session that is signed in right now;
      // a late 401 from a previous account must not affect the next one.
      const session = token ? tokenSession(token) : null
      if (!token || session === null || session.userId !== clerk.user?.id) return
      if (isSigningOut.current || isFinishingDeletion.current) return
      if (message === ACCOUNT_DELETED_MESSAGE) {
        // A previous deletion may not have removed the Clerk user yet; the API
        // still accepts this one call from the deleted account to finish it.
        // Both the cleanup and the sign-out stay bound to that session.
        isFinishingDeletion.current = true
        void deleteAccount({ token })
          .catch(() => undefined)
          .finally(() => {
            isFinishingDeletion.current = false
            void endSession(session)
          })
        return
      }
      void endSession(session)
    })
    return () => setUnauthorizedHandler(null)
  }, [clerk, endSession])

  const { refetch } = profileQuery
  const refreshUser = useCallback(async () => {
    await refetch()
  }, [refetch])

  const user: AuthUser | null =
    signedIn && clerkUser && profileQuery.data
      ? {
          email: clerkUser.primaryEmailAddress?.emailAddress ?? '',
          avatarUrl: clerkUser.imageUrl || undefined,
          preferredLanguage: profileQuery.data.preferredLanguage,
        }
      : null

  const isLoading = !cacheReady || (signedIn && profileQuery.isPending)

  const value: AuthContextType = {
    user,
    isLoading,
    isSignedIn: signedIn,
    isAuthenticated: user !== null,
    profileError: profileQuery.error,
    logout,
    refreshUser,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
