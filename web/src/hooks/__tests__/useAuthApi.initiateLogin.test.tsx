import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { useInitiateLogin } from '../useAuthApi'
import { getGoogleAuthUrl } from '@/lib/api'
import { useAuth } from '@/contexts/useAuth'

vi.mock('@/lib/api', () => ({
  getGoogleAuthUrl: vi.fn(),
  exchangeGoogleCode: vi.fn(),
  setAuthToken: vi.fn(),
  clearAuthToken: vi.fn(),
}))

vi.mock('@/contexts/useAuth', () => ({
  useAuth: vi.fn(),
}))

describe('useInitiateLogin', () => {
  let assignMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useAuth).mockReturnValue({
      refreshUser: vi.fn().mockResolvedValue(undefined),
    } as unknown as ReturnType<typeof useAuth>)
    vi.mocked(getGoogleAuthUrl).mockResolvedValue({ ssoRedirection: 'https://accounts.google.com/o/oauth2/auth' })
    assignMock = vi.fn()
    const mockedLocation = Object.create(window.location) as Location
    Object.defineProperty(mockedLocation, 'assign', {
      value: assignMock,
      configurable: true,
      writable: true,
    })
    vi.stubGlobal(
      'location',
      mockedLocation,
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('calls auth state endpoint and redirects in the same tab', async () => {
    const { result } = renderHook(() => useInitiateLogin())

    await act(async () => {
      await result.current.initiateLogin()
    })

    expect(getGoogleAuthUrl).toHaveBeenCalledTimes(1)
    expect(assignMock).toHaveBeenCalledWith('https://accounts.google.com/o/oauth2/auth')
    expect(result.current.isOpening).toBe(true)
  })

  it('resets opening state when fetching auth URL fails', async () => {
    vi.mocked(getGoogleAuthUrl).mockRejectedValue(new Error('network error'))
    const { result } = renderHook(() => useInitiateLogin())

    await act(async () => {
      await result.current.initiateLogin()
    })

    await waitFor(() => {
      expect(assignMock).not.toHaveBeenCalled()
      expect(result.current.isOpening).toBe(false)
    })
  })
})
