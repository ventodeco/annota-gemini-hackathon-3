import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
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
  default: ({ title, onBack }: { title: string; onBack?: () => void }) => (
    <div>
      <span>{title}</span>
      <button onClick={onBack}>Back</button>
    </div>
  ),
}))

vi.mock('@/components/layout/BottomNavigation', () => ({
  default: () => <div>Bottom Nav</div>,
}))

function HistoryRouteProbe() {
  const location = useLocation()
  return <div>{`History search: ${location.search}`}</div>
}

describe('AnnotationDetailPage', () => {
  it('preserves scanId query param when navigating back to history', async () => {
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/annotations/7?scanId=5']}>
        <Routes>
          <Route path="/annotations/:id" element={<AnnotationDetailPage />} />
          <Route path="/history" element={<HistoryRouteProbe />} />
        </Routes>
      </MemoryRouter>,
    )

    await user.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByText('History search: ?scanId=5')).toBeInTheDocument()
  })
})
