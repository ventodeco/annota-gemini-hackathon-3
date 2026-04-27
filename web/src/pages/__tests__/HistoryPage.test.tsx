import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
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
})
