import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React, { StrictMode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Scan } from '@/lib/types'
import LoadingPage from '../LoadingPage'

const navigateMock = vi.fn()
const getScanMock = vi.fn()
let routeId: string | undefined = '5'

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
  useParams: () => ({ id: routeId }),
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
    routeId = '5'
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

  it('offers a retry when checking the scan fails', async () => {
    const processingScan: Scan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      sourceType: 'image',
      status: 'processing',
      createdAt: '2026-02-08T20:30:41+07:00',
    }
    getScanMock.mockRejectedValueOnce(new Error('boom'))
    getScanMock.mockResolvedValue(processingScan)

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't check this scan")
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('Scanning in Progress..')).toBeInTheDocument()
    expect(getScanMock).toHaveBeenCalledTimes(2)
  })

  it('shows a readable failure instead of the raw reason code', async () => {
    getScanMock.mockResolvedValue({
      id: 5,
      imageUrl: '/uploads/5.jpg',
      sourceType: 'image',
      status: 'failed',
      failureReason: 'ocr_timeout',
      createdAt: '2026-02-08T20:30:41+07:00',
    })

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent('Reading this page took too long')
    expect(screen.queryByText(/ocr_timeout/)).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Upload another page' }))
    expect(navigateMock).toHaveBeenCalledWith('/welcome')
  })

  it('keeps polling a slow scan instead of leaving after 30 seconds', async () => {
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
      await vi.advanceTimersByTimeAsync(45000)
    })

    expect(navigateMock).not.toHaveBeenCalled()
    expect(getScanMock.mock.calls.length).toBeGreaterThan(10)
    expect(screen.getByRole('status')).toHaveTextContent('Scanning in Progress..')
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

  it('navigates once when OCR becomes ready while polling', async () => {
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
    const readyScan: Scan = {
      ...processingScan,
      detectedLanguage: 'JP',
      fullText: 'OCR text',
      status: 'ready',
    }
    getScanMock.mockResolvedValueOnce(processingScan)
    getScanMock.mockResolvedValue(readyScan)

    renderPage()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100)
    })
    expect(navigateMock).toHaveBeenCalledWith('/scans/5', {
      replace: true,
      state: { preloadedScan: readyScan },
    })

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000)
    })
    expect(navigateMock).toHaveBeenCalledTimes(1)
  })

  it.each(['abc', '0', '-3'])('redirects to welcome for invalid scan id %s', async (invalidId) => {
    routeId = invalidId

    renderPage()

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/welcome', { replace: true })
    })
    expect(getScanMock).not.toHaveBeenCalled()
  })
})
