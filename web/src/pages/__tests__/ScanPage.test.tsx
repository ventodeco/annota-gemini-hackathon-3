import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ScanPage from '../ScanPage'

const useScanMock = vi.fn()

vi.mock('@/hooks/useScans', () => ({
  useScan: (...args: unknown[]) => useScanMock(...args),
  isScanOcrReady: (scan: { fullText?: string } | undefined) =>
    Boolean(scan?.fullText && scan.fullText.trim().length > 0),
}))

vi.mock('@/hooks/useAnnotations', () => ({
  useAnalyzeText: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateAnnotation: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

vi.mock('@/hooks/useTextSelection', () => ({
  useTextSelection: () => ({
    selectedText: '',
    handleSelection: vi.fn(),
    clearSelection: vi.fn(),
  }),
}))

vi.mock('@/components/layout/Header', () => ({
  default: ({ title }: { title: string }) => <div>{title}</div>,
}))

vi.mock('@/components/layout/BottomActionBar', () => ({
  default: () => <div>bottom-action</div>,
}))

vi.mock('@/components/scanpage/AnnotationDrawer', () => ({
  AnnotationDrawer: () => null,
}))

describe('ScanPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
})
