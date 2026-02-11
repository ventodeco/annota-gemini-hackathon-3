import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useParams } from 'react-router-dom'
import OCRHistoryPage from '../OCRHistoryPage'

const useScansMock = vi.fn()

vi.mock('@/hooks/useScans', () => ({
  useScans: (...args: unknown[]) => useScansMock(...args),
  useDeleteScan: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

vi.mock('@/components/layout/Header', () => ({
  default: ({ title }: { title: string }) => <div>{title}</div>,
}))

vi.mock('@/components/layout/BottomNavigation', () => ({
  default: () => <div>Bottom Nav</div>,
}))

function ScanRouteProbe() {
  const { id } = useParams<{ id: string }>()
  return <div>{`Scan route ${id}`}</div>
}

describe('OCRHistoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders OCR history and opens selected scan', async () => {
    const user = userEvent.setup()
    useScansMock.mockReturnValue({
      data: {
        data: [
          {
            id: 12,
            imageUrl: '/uploads/12.jpg',
            detectedLanguage: 'JP',
            createdAt: '2026-02-09T00:00:00Z',
          },
        ],
        meta: { currentPage: 1, pageSize: 20 },
      },
      isLoading: false,
      error: null,
    })

    render(
      <MemoryRouter initialEntries={['/scans-history']}>
        <Routes>
          <Route path="/scans-history" element={<OCRHistoryPage />} />
          <Route path="/scans/:id" element={<ScanRouteProbe />} />
        </Routes>
      </MemoryRouter>,
    )

    await user.click(screen.getByRole('button', { name: 'Open OCR #12' }))
    expect(screen.getByText('Scan route 12')).toBeInTheDocument()
  })

  it('shows empty state with Start scanning CTA', () => {
    useScansMock.mockReturnValue({
      data: { data: [], meta: { currentPage: 1, pageSize: 20 } },
      isLoading: false,
      error: null,
    })

    render(
      <MemoryRouter initialEntries={['/scans-history']}>
        <Routes>
          <Route path="/scans-history" element={<OCRHistoryPage />} />
          <Route path="/welcome" element={<div>Welcome route</div>} />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText('No OCR scans yet.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Start scanning' })).toHaveAttribute('href', '/welcome')
  })
})
