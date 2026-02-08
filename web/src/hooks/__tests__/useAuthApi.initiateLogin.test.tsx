import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { useInitiateLogin } from '../useAuthApi'
import { getGoogleAuthUrl } from '@/lib/api'
import { useAuth } from '@/contexts/useAuth'

const mockNavigate = vi.fn()

vi.mock('@/lib/api', () => ({
  getGoogleAuthUrl: vi.fn(),
  exchangeGoogleCode: vi.fn(),
  setAuthToken: vi.fn(),
  clearAuthToken: vi.fn(),
}))

vi.mock('@/contexts/useAuth', () => ({
  useAuth: vi.fn(),
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

describe('useInitiateLogin', () => {
  const originalOpen = window.open
  const originalSetInterval = window.setInterval
  const originalClearInterval = window.clearInterval

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useAuth).mockReturnValue({
      refreshUser: vi.fn().mockResolvedValue(undefined),
    } as unknown as ReturnType<typeof useAuth>)
    vi.mocked(getGoogleAuthUrl).mockResolvedValue({ ssoRedirection: 'https://accounts.google.com/o/oauth2/auth' })

    Object.defineProperty(window, 'open', {
      configurable: true,
      writable: true,
      value: vi.fn(),
    })

    Object.defineProperty(window, 'setInterval', {
      configurable: true,
      writable: true,
      value: vi.fn(() => 1),
    })

    Object.defineProperty(window, 'clearInterval', {
      configurable: true,
      writable: true,
      value: vi.fn(),
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'open', {
      configurable: true,
      writable: true,
      value: originalOpen,
    })
    Object.defineProperty(window, 'setInterval', {
      configurable: true,
      writable: true,
      value: originalSetInterval,
    })
    Object.defineProperty(window, 'clearInterval', {
      configurable: true,
      writable: true,
      value: originalClearInterval,
    })
  })

  it('calls auth state endpoint, opens popup, and handles oauth-success message', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useAuth).mockReturnValue({
      refreshUser,
    } as unknown as ReturnType<typeof useAuth>)

    const fakePopup = { closed: false }
    vi.mocked(window.open).mockReturnValue(fakePopup as Window)

    const { result } = renderHook(() => useInitiateLogin())

    await act(async () => {
      await result.current.initiateLogin()
    })

    expect(getGoogleAuthUrl).toHaveBeenCalledTimes(1)
    expect(window.open).toHaveBeenCalledTimes(1)
    expect(result.current.isOpening).toBe(true)

    await act(async () => {
      window.dispatchEvent(new MessageEvent('message', {
        origin: window.location.origin,
        data: { type: 'oauth-success' },
      }))
    })

    await waitFor(() => {
      expect(refreshUser).toHaveBeenCalledTimes(1)
      expect(mockNavigate).toHaveBeenCalledWith('/welcome')
      expect(result.current.isOpening).toBe(false)
    })
  })

  it('handles popup blocked case', async () => {
    vi.mocked(window.open).mockReturnValue(null)

    const { result } = renderHook(() => useInitiateLogin())

    await act(async () => {
      await result.current.initiateLogin()
    })

    expect(window.open).toHaveBeenCalledTimes(1)
    await waitFor(() => {
      expect(result.current.isOpening).toBe(false)
    })
  })

  it('handles oauth-error message from popup', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useAuth).mockReturnValue({
      refreshUser,
    } as unknown as ReturnType<typeof useAuth>)

    const fakePopup = { closed: false }
    vi.mocked(window.open).mockReturnValue(fakePopup as Window)

    const { result } = renderHook(() => useInitiateLogin())

    await act(async () => {
      await result.current.initiateLogin()
    })

    await act(async () => {
      window.dispatchEvent(new MessageEvent('message', {
        origin: window.location.origin,
        data: { type: 'oauth-error' },
      }))
    })

    await waitFor(() => {
      expect(refreshUser).not.toHaveBeenCalled()
      expect(mockNavigate).not.toHaveBeenCalled()
      expect(result.current.isOpening).toBe(false)
    })
  })
})
