import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import React, { StrictMode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Scan } from '@/lib/types'
import LoadingPage from '../LoadingPage'

const navigateMock = vi.fn()
const getScanMock = vi.fn()

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
  useParams: () => ({ id: '5' }),
}))

vi.mock('@/lib/api', () => ({
  getScan: (...args: unknown[]) => getScanMock(...args),
}))

describe('LoadingPage', () => {
  const renderPage = (element: React.ReactElement = <LoadingPage />) => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    })

    return render(
      <QueryClientProvider client={queryClient}>
        {element}
      </QueryClientProvider>,
    )
  }

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

    renderPage()

    expect(screen.getByRole('heading', { name: 'Scan Result' })).toBeInTheDocument()
    expect(screen.getByText('Scanning in Progress..')).toBeInTheDocument()
  })

  it('redirects to scan page when OCR is ready', async () => {
    const readyScan: Scan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: 'JP',
      fullText: 'OCR text',
      sourceType: 'image',
      status: 'ready',
      createdAt: '2026-02-08T20:30:41+07:00',
    }
    getScanMock.mockResolvedValueOnce(readyScan)

    renderPage()

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/scans/5', {
        replace: true,
        state: { preloadedScan: readyScan },
      })
    })
  })

  it('schedules periodic polling when OCR is not ready', async () => {
    vi.useFakeTimers()

    const processingPartial: Scan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: '',
      fullText: '',
      sourceType: 'image',
      status: 'processing',
      createdAt: '2026-02-08T20:30:41+07:00',
    }
    getScanMock.mockResolvedValueOnce(processingPartial)
    getScanMock.mockResolvedValue(processingPartial)

    renderPage()

    await act(async () => {
      await Promise.resolve()
      await vi.advanceTimersByTimeAsync(3100)
    })
    expect(getScanMock.mock.calls.length).toBeGreaterThan(1)
  })

  it('shows error state when polling fails', async () => {
    getScanMock.mockRejectedValueOnce(new Error('boom'))

    renderPage()

    expect(await screen.findByRole('heading', { name: 'Scan Result' })).toBeInTheDocument()
    expect(await screen.findByText('Failed to load scan')).toBeInTheDocument()
  })

  it('navigates to scan page when timeout is reached', async () => {
    vi.useFakeTimers()

    const processingScan: Scan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: '',
      fullText: '',
      sourceType: 'image',
      status: 'processing',
      createdAt: '2026-02-08T20:30:41+07:00',
    }
    getScanMock.mockResolvedValue(processingScan)
    renderPage()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000)
      await vi.runOnlyPendingTimersAsync()
    })

    expect(navigateMock).toHaveBeenCalledWith('/scans/5', {
      replace: true,
      state: undefined,
    })
  })

  it('navigates once under StrictMode when OCR is already ready', async () => {
    const readyScan: Scan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: 'JP',
      fullText: 'OCR text',
      sourceType: 'image',
      status: 'ready',
      createdAt: '2026-02-08T20:30:41+07:00',
    }
    getScanMock.mockResolvedValue(readyScan)

    renderPage(
      <StrictMode>
        <LoadingPage />
      </StrictMode>,
    )

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/scans/5', {
        replace: true,
        state: { preloadedScan: readyScan },
      })
    })
    expect(navigateMock).toHaveBeenCalledTimes(1)
  })
})
