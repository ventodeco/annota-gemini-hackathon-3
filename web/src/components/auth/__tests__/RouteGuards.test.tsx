import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { ProtectedRoute, PublicRoute } from '../RouteGuards'
import type { AuthContextType } from '@/contexts/AuthContext'
import { ApiError } from '@/lib/api'
import { safeRedirectPath, tokenSession } from '@/lib/authSession'

const authState = vi.hoisted((): { value: AuthContextType } => ({
  value: {
    user: null,
    isLoading: false,
    isSignedIn: false,
    isAuthenticated: false,
    profileError: null,
    logout: async () => undefined,
    refreshUser: async () => undefined,
  },
}))

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => authState.value,
}))

vi.mock('@clerk/react', () => ({
  getToken: vi.fn(),
}))

function setAuth(patch: Partial<AuthContextType>) {
  authState.value = { ...authState.value, ...patch }
}

function LocationProbe() {
  const location = useLocation()
  return <p data-testid="location">{`${location.pathname}${location.search}`}</p>
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/login/*"
          element={
            <PublicRoute>
              <p>sign-in form</p>
            </PublicRoute>
          }
        />
        <Route
          path="/documents/:id"
          element={
            <ProtectedRoute>
              <p>private document</p>
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </Routes>
      <LocationProbe />
    </MemoryRouter>,
  )
}

describe('route guards', () => {
  beforeEach(() => {
    setAuth({
      user: null,
      isLoading: false,
      isSignedIn: false,
      isAuthenticated: false,
      profileError: null,
      logout: vi.fn(async () => undefined),
      refreshUser: vi.fn(async () => undefined),
    })
  })

  it('shows a busy state while the session is resolving', () => {
    setAuth({ isLoading: true })
    renderAt('/documents/5')
    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true')
    expect(screen.queryByText('private document')).not.toBeInTheDocument()
  })

  it('sends signed-out deep links to sign-in with a return path', () => {
    renderAt('/documents/5?page=3')
    expect(screen.getByText('sign-in form')).toBeInTheDocument()
    expect(screen.getAllByTestId('location')[0]).toHaveTextContent(
      `/login?redirect_url=${encodeURIComponent('/documents/5?page=3')}`,
    )
  })

  it('returns a signed-in user to the requested deep link', () => {
    setAuth({ isSignedIn: true, isAuthenticated: true })
    renderAt(`/login?redirect_url=${encodeURIComponent('/documents/5?page=3')}`)
    expect(screen.getByText('private document')).toBeInTheDocument()
  })

  it('ignores cross-origin redirect targets', () => {
    setAuth({ isSignedIn: true, isAuthenticated: true })
    renderAt(`/login?redirect_url=${encodeURIComponent('//evil.example/steal')}`)
    expect(screen.getAllByTestId('location')[0]).toHaveTextContent('/welcome')
  })

  it('asks a legacy user to continue with Google instead of creating a new library', async () => {
    const logout = vi.fn(async () => undefined)
    setAuth({
      isSignedIn: true,
      logout,
      profileError: new ApiError(409, JSON.stringify({ error: 'Conflict', message: 'legacy_google_link_required' })),
    })
    renderAt('/documents/5')

    expect(screen.getByRole('heading', { name: 'Restore your existing library' })).toBeInTheDocument()
    expect(screen.queryByText('private document')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Continue with Google' }))
    expect(logout).toHaveBeenCalled()
  })

  it('offers a retry when the identity service is unavailable', async () => {
    const refreshUser = vi.fn(async () => undefined)
    setAuth({
      isSignedIn: true,
      refreshUser,
      profileError: new ApiError(503, JSON.stringify({ error: 'Service Unavailable', message: 'Authentication service unavailable' })),
    })
    renderAt('/documents/5')

    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(refreshUser).toHaveBeenCalled()
  })
})

describe('tokenSession', () => {
  it('reads the Clerk user and session ids from a base64url payload', () => {
    const payload = btoa(JSON.stringify({ sub: 'user_123', sid: 'sess_9', azp: 'http://localhost:5173' }))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '')
    expect(tokenSession(`h.${payload}.s`)).toEqual({ userId: 'user_123', sessionId: 'sess_9' })
  })

  it.each(['', 'not-a-jwt', 'h.%%%.s', `h.${btoa('{"sub":42}')}.s`])('returns null for %s', (token) => {
    expect(tokenSession(token)).toBeNull()
  })
})

describe('safeRedirectPath', () => {
  it.each([
    ['/documents/5?page=2#p', '/documents/5?page=2#p'],
    [`${window.location.origin}/history`, '/history'],
    ['https://evil.example/x', null],
    ['//evil.example/x', null],
    ['/login', null],
    ['/login/factor-one', null],
    ['/sign-up', null],
    [null, null],
  ])('%s -> %s', (input, expected) => {
    expect(safeRedirectPath(input)).toBe(expected)
  })
})
