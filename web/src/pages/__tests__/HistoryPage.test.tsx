import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import HistoryPage from '../HistoryPage'

const useAnnotationsMock = vi.fn()

vi.mock('@/hooks/useAnnotations', () => ({
  useAnnotations: (...args: unknown[]) => useAnnotationsMock(...args),
  useDeleteAnnotation: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

vi.mock('@/components/layout/Header', () => ({
  default: ({ title }: { title: string }) => <div>{title}</div>,
}))

vi.mock('@/components/layout/BottomNavigation', () => ({
  default: () => <div>Bottom Nav</div>,
}))

function LocationProbe() {
  const location = useLocation()
  return <div>{`${location.pathname}${location.search}`}</div>
}

const savedAnnotation = {
  id: 7,
  highlightedText: '勉強',
  nuanceSummary: 'study',
  createdAt: '2026-02-08T20:30:41+07:00',
}

describe('HistoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAnnotationsMock.mockReturnValue({
      data: { data: [], meta: { currentPage: 1, pageSize: 20 } },
      isLoading: false,
      error: null,
    })
  })

  it('requests global annotation history when scanId is not present', () => {
    render(
      <MemoryRouter initialEntries={['/history']}>
        <Routes>
          <Route path="/history" element={<HistoryPage />} />
        </Routes>
      </MemoryRouter>,
    )

    expect(useAnnotationsMock).toHaveBeenCalledWith(1, 20, undefined, undefined, undefined)
    expect(screen.getByText('History')).toBeInTheDocument()
  })

  it('requests per-scan annotation history when scanId is present', () => {
    render(
      <MemoryRouter initialEntries={['/history?scanId=5']}>
        <Routes>
          <Route path="/history" element={<HistoryPage />} />
        </Routes>
      </MemoryRouter>,
    )

    expect(useAnnotationsMock).toHaveBeenCalledWith(1, 20, 5, undefined, undefined)
    expect(screen.getByText('Annotations for OCR #5')).toBeInTheDocument()
  })

  it('requests document page annotation history when documentId and pageNumber are present', () => {
    render(
      <MemoryRouter initialEntries={['/history?documentId=9&pageNumber=3']}>
        <Routes>
          <Route path="/history" element={<HistoryPage />} />
        </Routes>
      </MemoryRouter>,
    )

    expect(useAnnotationsMock).toHaveBeenCalledWith(1, 20, undefined, 9, 3)
    expect(screen.getByText('Annotations for document #9, page 3')).toBeInTheDocument()
  })

  it.each([
    ['/history?scanId=5', 'Annotations for OCR #5', '/annotations/7?scanId=5'],
    ['/history?documentId=9&pageNumber=3', 'Annotations for document #9, page 3', '/annotations/7?documentId=9&pageNumber=3'],
    ['/history?documentId=9', 'Annotations for document #9', '/annotations/7?documentId=9'],
    ['/history', 'History', '/annotations/7'],
  ])('opens annotation detail from %s', (entry, title, detailPath) => {
    useAnnotationsMock.mockReturnValue({
      data: { data: [savedAnnotation], meta: { currentPage: 1, pageSize: 20 } },
      isLoading: false,
      error: null,
    })

    render(
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/annotations/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText(title)).toBeInTheDocument()
    fireEvent.click(screen.getByText('勉強'))
    expect(screen.getByText(detailPath)).toBeInTheDocument()
  })

  it('opens a saved annotation with the keyboard, separately from delete', async () => {
    useAnnotationsMock.mockReturnValue({
      data: { data: [savedAnnotation], meta: { currentPage: 1, pageSize: 20 } },
      isLoading: false,
      error: null,
    })
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/history']}>
        <Routes>
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/annotations/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    )

    await user.tab()
    expect(screen.getByRole('button', { name: /^勉強/ })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Delete annotation: 勉強' })).toHaveFocus()
    await user.tab({ shift: true })
    await user.keyboard('{Enter}')
    expect(screen.getByText('/annotations/7')).toBeInTheDocument()
  })
})
