import React from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AnnotationDetailPage from '../AnnotationDetailPage'

vi.mock('@/hooks/useAnnotationById', () => ({
  useAnnotationById: () => ({
    data: {
      id: 7,
      highlightedText: 'test',
      contextText: 'context',
      nuanceData: {
        meaning: 'meaning',
        usageExample: '',
        usageTiming: '',
        wordBreakdown: '',
        alternativeMeaning: '',
      },
      createdAt: '2026-02-09T00:00:00Z',
    },
    isLoading: false,
    error: null,
  }),
}))

vi.mock('@/components/layout/Header', () => ({
  default: ({ title, onBack }: { title: string; onBack?: () => void }) => {
    const navigate = useNavigate()
    const location = useLocation()
    const handleBack = onBack ?? (() => navigate(`/history${location.search}`))
    return (
      <div>
        <span>{title}</span>
        <button onClick={handleBack}>Back</button>
      </div>
    )
  },
}))

vi.mock('@/components/layout/BottomNavigation', () => ({
  default: () => <div>Bottom Nav</div>,
}))

vi.mock('@/hooks/useAnnotations', () => ({
  useDeleteAnnotation: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

function HistoryRouteProbe() {
  const location = useLocation()
  return <div>{`History search: ${location.search}`}</div>
}

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <MemoryRouter initialEntries={['/annotations/7?scanId=5']}>
      <Routes>
        <Route path="/annotations/:id" element={children} />
        <Route path="/history" element={<HistoryRouteProbe />} />
      </Routes>
    </MemoryRouter>
  </QueryClientProvider>
)

describe('AnnotationDetailPage', () => {
  it('preserves scanId query param when navigating back to history', async () => {
    const user = userEvent.setup()

    render(<AnnotationDetailPage />, { wrapper })

    await user.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByText('History search: ?scanId=5')).toBeInTheDocument()
  })
})
