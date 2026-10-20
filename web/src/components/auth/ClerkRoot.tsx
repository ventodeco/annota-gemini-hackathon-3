import type { ReactNode } from 'react'
import { ClerkProvider } from '@clerk/react'
import { useNavigate } from 'react-router-dom'
import { AuthProvider } from '@/contexts/AuthContext'

/**
 * Clerk session provider wired to React Router so Clerk components navigate
 * in-app instead of reloading. Must render inside <BrowserRouter>.
 */
export function ClerkRoot({ publishableKey, children }: { publishableKey: string; children: ReactNode }) {
  const navigate = useNavigate()

  return (
    <ClerkProvider
      publishableKey={publishableKey}
      routerPush={(to) => navigate(to)}
      routerReplace={(to) => navigate(to, { replace: true })}
      signInUrl="/login"
      signUpUrl="/sign-up"
      afterSignOutUrl="/login"
    >
      <AuthProvider>{children}</AuthProvider>
    </ClerkProvider>
  )
}

export function AuthNotConfigured() {
  return (
    <main className="min-h-screen bg-white flex flex-col items-center justify-center p-6">
      <div className="flex flex-col items-center gap-4 max-w-md w-full text-center" role="alert">
        <img src="/logo-anota.png" alt="ANNOTA" className="h-12 w-auto" />
        <h1 className="text-lg font-semibold text-gray-900">Sign-in is not configured</h1>
        <p className="text-base text-gray-700 leading-relaxed">
          This build has no Clerk publishable key. Set <code>VITE_CLERK_PUBLISHABLE_KEY</code> and rebuild
          the app to enable Google and email sign-in.
        </p>
      </div>
    </main>
  )
}
