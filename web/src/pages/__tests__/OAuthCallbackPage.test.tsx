import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import OAuthCallbackPage from '../OAuthCallbackPage'
import { AuthContext, type AuthContextType } from '@/contexts/AuthContext'
import { exchangeGoogleCode, setAuthToken, clearAuthToken } from '@/lib/api'

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    exchangeGoogleCode: vi.fn(),
    setAuthToken: vi.fn(),
    clearAuthToken: vi.fn(),
  }
})

function createAuthContextValue(overrides: Partial<AuthContextType> = {}): AuthContextType {
  return {
    user: null,
    isLoading: false,
    isAuthenticated: false,
    login: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function renderCallbackRoute(entry: string, authContextValue?: AuthContextType) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  return render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <AuthContext.Provider value={authContextValue ?? createAuthContextValue()}>
          <MemoryRouter initialEntries={[entry]}>
            <Routes>
              <Route path="/auth/callback" element={<OAuthCallbackPage />} />
              <Route path="/welcome" element={<div>Welcome Route</div>} />
              <Route path="/login" element={<div>Login Route</div>} />
            </Routes>
          </MemoryRouter>
        </AuthContext.Provider>
      </QueryClientProvider>
    </React.StrictMode>,
  )
}

describe('OAuthCallbackPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('navigates to welcome when code exchange succeeds', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(exchangeGoogleCode).mockResolvedValue({
      token: 'jwt-token',
      expirySeconds: 1800,
      expiresAt: '2026-02-09T00:00:00Z',
    })

    renderCallbackRoute('/auth/callback?state=test-state&code=test-code', createAuthContextValue({ refreshUser }))

    await waitFor(() => {
      expect(exchangeGoogleCode).toHaveBeenCalled()
    })

    await waitFor(() => {
      expect(screen.getByText('Welcome Route')).toBeInTheDocument()
    })

    expect(setAuthToken).toHaveBeenCalledWith('jwt-token')
    expect(refreshUser).toHaveBeenCalled()
  })

  it('shows error state when callback contains OAuth error', async () => {
    renderCallbackRoute('/auth/callback?error=access_denied')

    expect(await screen.findByText('Login failed. Please try again.')).toBeInTheDocument()
    expect(exchangeGoogleCode).not.toHaveBeenCalled()
  })

  it('shows error state when state or code is missing', async () => {
    renderCallbackRoute('/auth/callback?state=only-state')

    expect(await screen.findByText('Login failed. Please try again.')).toBeInTheDocument()
    expect(exchangeGoogleCode).not.toHaveBeenCalled()
  })

  it('shows error state when exchange fails and can navigate back to login', async () => {
    vi.mocked(exchangeGoogleCode).mockRejectedValue(new Error('Invalid state'))
    const user = userEvent.setup()

    renderCallbackRoute('/auth/callback?state=test-state&code=test-code')

    expect(await screen.findByText('Login failed. Please try again.')).toBeInTheDocument()
    expect(clearAuthToken).toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Back to Login' }))
    expect(screen.getByText('Login Route')).toBeInTheDocument()
  })
})
