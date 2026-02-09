import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ScanPage from '../ScanPage'

const useScanMock = vi.fn()
const analyzeMutateAsyncMock = vi.fn()
const createMutateAsyncMock = vi.fn()
const clearSelectionMock = vi.fn()

vi.mock('@/hooks/useScans', () => ({
  useScan: (...args: unknown[]) => useScanMock(...args),
  isScanOcrReady: (scan: { fullText?: string } | undefined) =>
    Boolean(scan?.fullText && scan.fullText.trim().length > 0),
}))

vi.mock('@/hooks/useAnnotations', () => ({
  useAnalyzeText: () => ({ mutateAsync: analyzeMutateAsyncMock, isPending: false }),
  useCreateAnnotation: () => ({ mutateAsync: createMutateAsyncMock, isPending: false }),
}))

vi.mock('@/hooks/useTextSelection', () => ({
  useTextSelection: () => ({
    selectedText: '今月はCVRが前月比+1.2pt',
    handleSelection: vi.fn(),
    clearSelection: clearSelectionMock,
  }),
}))

vi.mock('@/components/layout/Header', () => ({
  default: ({ title }: { title: string }) => <div>{title}</div>,
}))

vi.mock('@/components/layout/BottomActionBar', () => ({
  default: ({
    onExplain,
    onBookmark,
  }: {
    onExplain: () => void
    onBookmark?: () => void
  }) => (
    <>
      <button onClick={onExplain}>Explain</button>
      <button onClick={onBookmark}>Bookmark</button>
    </>
  ),
}))

vi.mock('@/components/scanpage/AnnotationDrawer', () => ({
  AnnotationDrawer: ({
    annotation,
    onRegenerate,
    onSave,
    version,
  }: {
    annotation: { nuance_data?: { meaning?: string } } | null
    onRegenerate?: () => void
    onSave?: () => void
    version?: number
  }) =>
    annotation ? (
      <div>
        <span>{`Version ${version}/2`}</span>
        <span>{annotation.nuance_data?.meaning}</span>
        <button onClick={onRegenerate}>Regenerate</button>
        <button onClick={onSave}>Save Annotation</button>
      </div>
    ) : null,
}))

describe('ScanPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    analyzeMutateAsyncMock.mockReset()
    createMutateAsyncMock.mockReset()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/scans/5']}>
        <Routes>
          <Route path="/scans/:id" element={children} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )

  it('shows loading spinner when OCR text is not ready yet', () => {
    useScanMock.mockReturnValue({
      data: {
        id: 5,
        imageUrl: '/uploads/5.jpg',
        detectedLanguage: '',
        fullText: '',
        createdAt: '2026-02-08T20:30:41+07:00',
      },
      isLoading: false,
      error: null,
    })

    render(<ScanPage />, { wrapper })

    expect(screen.getByText('Processing...')).toBeInTheDocument()
  })

  it('renders OCR text once available', () => {
    useScanMock.mockReturnValue({
      data: {
        id: 5,
        imageUrl: '/uploads/5.jpg',
        detectedLanguage: 'JP',
        fullText: 'これはテストです',
        createdAt: '2026-02-08T20:30:41+07:00',
      },
      isLoading: false,
      error: null,
    })

    render(<ScanPage />, { wrapper })

    expect(screen.getByText('これはテストです')).toBeInTheDocument()
  })

  it('skips fetching when preloaded scan with OCR text is provided from loading page', () => {
    const preloadedScan = {
      id: 5,
      imageUrl: '/uploads/5.jpg',
      detectedLanguage: 'JP',
      fullText: 'preloaded OCR',
      createdAt: '2026-02-08T20:30:41+07:00',
    }

    useScanMock.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    })

    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter initialEntries={[{ pathname: '/scans/5', state: { preloadedScan } }]}>
          <Routes>
            <Route path="/scans/:id" element={<ScanPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )

    expect(useScanMock).toHaveBeenCalledWith(5, { enabled: false, pollIntervalMs: 0 })
    expect(screen.getByText('preloaded OCR')).toBeInTheDocument()
  })

  it('regenerates up to version 2/2 and saves regenerated annotation payload', async () => {
    const user = userEvent.setup()

    useScanMock.mockReturnValue({
      data: {
        id: 5,
        imageUrl: '/uploads/5.jpg',
        detectedLanguage: 'JP',
        fullText: 'これはテストです',
        createdAt: '2026-02-08T20:30:41+07:00',
      },
      isLoading: false,
      error: null,
    })

    analyzeMutateAsyncMock
      .mockResolvedValueOnce({
        meaning: 'initial meaning',
        usageExample: 'initial example',
        usageTiming: 'initial timing',
        wordBreakdown: 'initial breakdown',
        alternativeMeaning: 'initial alt',
      })
      .mockResolvedValueOnce({
        meaning: 'regenerated meaning',
        usageExample: 'regenerated example',
        usageTiming: 'regenerated timing',
        wordBreakdown: 'regenerated breakdown',
        alternativeMeaning: 'regenerated alt',
      })

    createMutateAsyncMock.mockResolvedValueOnce({})

    render(<ScanPage />, { wrapper })

    await user.click(screen.getByRole('button', { name: 'Explain' }))

    await waitFor(() => {
      expect(screen.getByText('Version 1/2')).toBeInTheDocument()
      expect(screen.getByText('initial meaning')).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: 'Regenerate' }))

    await waitFor(() => {
      expect(screen.getByText('Version 2/2')).toBeInTheDocument()
      expect(screen.getByText('regenerated meaning')).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: 'Regenerate' }))

    expect(analyzeMutateAsyncMock).toHaveBeenCalledTimes(2)

    await user.click(screen.getByRole('button', { name: 'Save Annotation' }))

    await waitFor(() => {
      expect(createMutateAsyncMock).toHaveBeenCalledWith(
        expect.objectContaining({
          scanId: 5,
          highlightedText: '今月はCVRが前月比+1.2pt',
          contextText: '',
          nuanceData: expect.objectContaining({ meaning: 'regenerated meaning' }),
        }),
      )
      expect(clearSelectionMock).toHaveBeenCalled()
    })
  })

  it('navigates to per-scan history when bookmark is clicked', async () => {
    const user = userEvent.setup()
    useScanMock.mockReturnValue({
      data: {
        id: 5,
        imageUrl: '/uploads/5.jpg',
        detectedLanguage: 'JP',
        fullText: 'これはテストです',
        createdAt: '2026-02-08T20:30:41+07:00',
      },
      isLoading: false,
      error: null,
    })

    const HistoryProbe = () => {
      const location = useLocation()
      return <div>{`History Route ${location.search}`}</div>
    }

    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter initialEntries={['/scans/5']}>
          <Routes>
            <Route path="/scans/:id" element={<ScanPage />} />
            <Route path="/history" element={<HistoryProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )

    await user.click(screen.getByRole('button', { name: 'Bookmark' }))
    await waitFor(() => {
      expect(screen.getByText('History Route ?scanId=5')).toBeInTheDocument()
    })
  })
})
