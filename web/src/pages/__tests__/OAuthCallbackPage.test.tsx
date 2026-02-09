import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import OAuthCallbackPage from '../OAuthCallbackPage'
import { useLogin } from '@/hooks/useAuthApi'

vi.mock('@/hooks/useAuthApi', () => ({
  useLogin: vi.fn(),
}))

function renderCallbackRoute(entry: string) {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/auth/callback" element={<OAuthCallbackPage />} />
        <Route path="/welcome" element={<div>Welcome Route</div>} />
        <Route path="/login" element={<div>Login Route</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('OAuthCallbackPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('calls exchange mutation, shows success, and navigates to welcome', async () => {
    const mutate = vi.fn((_: unknown, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.()
    })
    vi.mocked(useLogin).mockReturnValue({ mutate } as unknown as ReturnType<typeof useLogin>)

    renderCallbackRoute('/auth/callback?state=test-state&code=test-code')

    await waitFor(() => {
      expect(mutate).toHaveBeenCalled()
    })

    expect(await screen.findByText('Login successful! Redirecting...')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.getByText('Welcome Route')).toBeInTheDocument()
    }, { timeout: 2500 })
  })

  it('shows error state when callback contains OAuth error', async () => {
    const mutate = vi.fn()
    vi.mocked(useLogin).mockReturnValue({ mutate } as unknown as ReturnType<typeof useLogin>)

    renderCallbackRoute('/auth/callback?error=access_denied')

    expect(await screen.findByText('Login failed. Please try again.')).toBeInTheDocument()
    expect(mutate).not.toHaveBeenCalled()
  })

  it('does not attempt login when state or code is missing', async () => {
    const mutate = vi.fn()
    vi.mocked(useLogin).mockReturnValue({ mutate } as unknown as ReturnType<typeof useLogin>)

    renderCallbackRoute('/auth/callback?state=only-state')

    expect(await screen.findByText('Processing your login...')).toBeInTheDocument()
    expect(mutate).not.toHaveBeenCalled()
  })

  it('shows error state when exchange fails and can navigate back to login', async () => {
    const user = userEvent.setup()
    const mutate = vi.fn((_: unknown, options?: { onError?: () => void }) => {
      options?.onError?.()
    })
    vi.mocked(useLogin).mockReturnValue({ mutate } as unknown as ReturnType<typeof useLogin>)

    renderCallbackRoute('/auth/callback?state=test-state&code=test-code')

    expect(await screen.findByText('Login failed. Please try again.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Back to Login' }))
    expect(screen.getByText('Login Route')).toBeInTheDocument()
  })
})
