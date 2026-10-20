import type { ReactNode } from 'react'
import { Navigate, useLocation, useSearchParams } from 'react-router-dom'
import { useAuth } from '@/contexts/useAuth'
import { ApiError } from '@/lib/api'
import { safeRedirectPath } from '@/lib/authSession'
import { Button } from '@/components/ui/button'

function FullPageSpinner() {
  return (
    <div className="min-h-screen flex items-center justify-center" role="status" aria-busy="true">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      <span className="sr-only">Checking your session…</span>
    </div>
  )
}

type AccountIssue = 'legacy-link' | 'verify-email' | 'unavailable'

function accountIssueFor(error: Error): AccountIssue {
  if (error instanceof ApiError) {
    if (error.status === 409 && error.serverMessage === 'legacy_google_link_required') return 'legacy-link'
    if (error.status === 403) return 'verify-email'
  }
  return 'unavailable'
}

const ACCOUNT_ISSUE_COPY: Record<AccountIssue, { title: string; body: string; action: string }> = {
  'legacy-link': {
    title: 'Restore your existing library',
    body: 'This email already has an ANNOTA library created with Google sign-in. Continue with Google using the same account to restore it.',
    action: 'Continue with Google',
  },
  'verify-email': {
    title: 'Verify your email',
    body: 'ANNOTA needs a verified email address. Sign in again and complete email verification.',
    action: 'Sign in again',
  },
  unavailable: {
    title: 'We could not load your account',
    body: 'The sign-in service is temporarily unavailable. Please try again in a moment.',
    action: 'Try again',
  },
}

export function AccountStatusScreen({ error }: { error: Error }) {
  const { logout, refreshUser } = useAuth()
  const issue = accountIssueFor(error)
  const copy = ACCOUNT_ISSUE_COPY[issue]

  const handleAction = () => {
    if (issue === 'unavailable') {
      void refreshUser()
      return
    }
    void logout()
  }

  return (
    <main className="min-h-screen bg-white flex flex-col items-center justify-center p-6">
      <div className="flex flex-col items-center gap-4 max-w-md w-full text-center" role="alert">
        <img src="/logo-anota.png" alt="ANNOTA" className="h-12 w-auto" />
        <h1 className="text-lg font-semibold text-gray-900">{copy.title}</h1>
        <p className="text-base text-gray-700 leading-relaxed">{copy.body}</p>
        <Button type="button" onClick={handleAction} className="min-h-11 rounded-full px-6">
          {copy.action}
        </Button>
        {issue !== 'unavailable' && (
          <Button type="button" variant="ghost" onClick={() => void logout()} className="min-h-11">
            Sign out
          </Button>
        )}
      </div>
    </main>
  )
}

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isSignedIn, isLoading, profileError } = useAuth()
  const location = useLocation()

  if (isLoading) return <FullPageSpinner />

  if (!isSignedIn) {
    const returnTo = `${location.pathname}${location.search}${location.hash}`
    return <Navigate to={`/login?redirect_url=${encodeURIComponent(returnTo)}`} replace />
  }

  if (profileError) return <AccountStatusScreen error={profileError} />

  return <>{children}</>
}

export function PublicRoute({ children }: { children: ReactNode }) {
  const { isSignedIn, isLoading } = useAuth()
  const [searchParams] = useSearchParams()

  if (isLoading) return <FullPageSpinner />

  if (isSignedIn) {
    return <Navigate to={safeRedirectPath(searchParams.get('redirect_url')) ?? '/welcome'} replace />
  }

  return <>{children}</>
}
