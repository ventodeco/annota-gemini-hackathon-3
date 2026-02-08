import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
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
      </Routes>
    </MemoryRouter>,
  )
}

describe('OAuthCallbackPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'opener', {
      value: null,
      configurable: true,
      writable: true,
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'opener', {
      value: null,
      configurable: true,
      writable: true,
    })
  })

  it('calls exchange mutation and reaches success state', async () => {
    const postMessage = vi.fn()
    Object.defineProperty(window, 'opener', {
      value: { postMessage },
      configurable: true,
      writable: true,
    })

    const mutate = vi.fn((_: unknown, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.()
    })
    vi.mocked(useLogin).mockReturnValue({ mutate } as unknown as ReturnType<typeof useLogin>)

    renderCallbackRoute('/auth/callback?state=test-state&code=test-code')

    await waitFor(() => {
      expect(mutate).toHaveBeenCalled()
    })

    expect(await screen.findByText('Login successful! Redirecting...')).toBeInTheDocument()
    expect(postMessage).toHaveBeenCalledWith({ type: 'oauth-success' }, window.location.origin)
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

  it('posts oauth-error when exchange fails in popup mode', async () => {
    const postMessage = vi.fn()
    Object.defineProperty(window, 'opener', {
      value: { postMessage },
      configurable: true,
      writable: true,
    })

    const mutate = vi.fn((_: unknown, options?: { onError?: () => void }) => {
      options?.onError?.()
    })
    vi.mocked(useLogin).mockReturnValue({ mutate } as unknown as ReturnType<typeof useLogin>)

    renderCallbackRoute('/auth/callback?state=test-state&code=test-code')

    expect(await screen.findByText('Login failed. Please try again.')).toBeInTheDocument()
    expect(postMessage).toHaveBeenCalledWith({ type: 'oauth-error' }, window.location.origin)
  })
})
