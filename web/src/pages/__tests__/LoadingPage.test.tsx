import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import LoadingPage from '../LoadingPage'

const navigateMock = vi.fn()
const getScanMock = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useParams: () => ({ id: '5' }),
  }
})

vi.mock('@/lib/api', () => ({
  getScan: (...args: unknown[]) => getScanMock(...args),
}))

describe('LoadingPage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows loading spinner while processing', async () => {
    getScanMock.mockImplementation(
      () => new Promise(() => {}),
    )

    render(<LoadingPage />)

    expect(screen.getByText('Scanning in Progress..')).toBeInTheDocument()
  })

  it('redirects to scan page when OCR is ready', async () => {
    const readyScan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: 'JP',
      fullText: 'OCR text',
      createdAt: '2026-02-08T20:30:41+07:00',
    }
    getScanMock.mockResolvedValueOnce(readyScan)

    render(<LoadingPage />)

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/scans/5', {
        replace: true,
        state: { preloadedScan: readyScan },
      })
    })
  })

  it('schedules next polling cycle when OCR is not ready', async () => {
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout')
    getScanMock.mockResolvedValueOnce({
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: '',
      fullText: '',
      createdAt: '2026-02-08T20:30:41+07:00',
    })

    render(<LoadingPage />)

    await waitFor(() => {
      expect(getScanMock).toHaveBeenCalledTimes(1)
      expect(setTimeoutSpy).toHaveBeenCalled()
    })

    setTimeoutSpy.mockRestore()
  })

  it('shows error state when polling fails', async () => {
    getScanMock.mockRejectedValueOnce(new Error('boom'))

    render(<LoadingPage />)

    expect(await screen.findByText('Failed to load scan')).toBeInTheDocument()
  })

  it('does not schedule polling after component unmounts', async () => {
    let resolveRequest: ((value: unknown) => void) | null = null
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout')
    getScanMock.mockImplementationOnce(
      () => new Promise((resolve) => {
        resolveRequest = resolve
      }),
    )

    const { unmount } = render(<LoadingPage />)
    unmount()

    resolveRequest?.({
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: '',
      fullText: '',
      createdAt: '2026-02-08T20:30:41+07:00',
    })
    await Promise.resolve()

    expect(setTimeoutSpy).not.toHaveBeenCalled()
    setTimeoutSpy.mockRestore()
  })
})
