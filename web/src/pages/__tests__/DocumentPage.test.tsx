import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import DocumentPage from '../DocumentPage'

const getDocumentMock = vi.fn()
const getDocumentFileMock = vi.fn()
const updateDocumentProgressMock = vi.fn()

type PageChangeSource = 'scroll' | 'navigation'

type ContinuousPDFViewerMockProps = {
  currentPage: number
  onPageChange: (page: number, change: { source: PageChangeSource }) => void
  onTextSelect: (selectedText: string) => void
}

vi.mock('@/lib/api', () => ({
  getDocument: (...args: unknown[]) => getDocumentMock(...args),
  getDocumentFile: (...args: unknown[]) => getDocumentFileMock(...args),
  updateDocumentProgress: (...args: unknown[]) => updateDocumentProgressMock(...args),
  getDocumentPage: vi.fn(),
  createScanFromPage: vi.fn(),
  analyzeText: vi.fn(),
  createAnnotation: vi.fn(),
  synthesizeSpeech: vi.fn(),
  getAuthToken: () => 'mock-token',
}))

vi.mock('@/components/documentpage/ContinuousPDFViewer', () => ({
  default: (props: ContinuousPDFViewerMockProps) => React.createElement(
    'div',
    { 'data-testid': 'continuous-pdf-viewer' },
    React.createElement('span', null, 'Continuous PDF Viewer'),
    React.createElement('span', { 'data-testid': 'current-page' }, props.currentPage),
    React.createElement(
      'button',
      { type: 'button', onClick: () => props.onTextSelect('選択中') },
      'Select text'
    ),
    React.createElement(
      'button',
      { type: 'button', onClick: () => props.onPageChange(2, { source: 'scroll' }) },
      'Scroll to page 2'
    ),
    React.createElement(
      'button',
      { type: 'button', onClick: () => props.onPageChange(1, { source: 'scroll' }) },
      'Scroll to page 1'
    )
  ),
}))

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => ({ isAuthenticated: true, isLoading: false, user: { id: 1 } }),
}))

describe('DocumentPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    updateDocumentProgressMock.mockResolvedValue({ status: 'saved' })
  })

  function renderPage(id = '1') {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
      React.createElement(QueryClientProvider, { client: queryClient },
        React.createElement(MemoryRouter, { initialEntries: [`/documents/${id}`] },
          React.createElement(Routes, null,
            React.createElement(Route, { path: '/documents/:id', element: React.createElement(DocumentPage) })
          )
        )
      )
    )
  }

  it('renders continuous PDF viewer directly without page list', async () => {
    getDocumentMock.mockResolvedValueOnce({ id: 1, filename: 'test.pdf', pageCount: 3, createdAt: '2026-03-21' })
    getDocumentFileMock.mockResolvedValueOnce(new Blob(['fake pdf'], { type: 'application/pdf' }))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('test.pdf')).toBeInTheDocument()
      expect(screen.getByTestId('continuous-pdf-viewer')).toBeInTheDocument()
    })
  })

  it('shows error for non-existent document', async () => {
    getDocumentMock.mockRejectedValueOnce(new Error('Not found'))
    renderPage('999')
    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument()
    })
  })

  it('keeps selected PDF text active when scroll updates the visible page', async () => {
    getDocumentMock.mockResolvedValueOnce({
      id: 1,
      filename: 'test.pdf',
      pageCount: 3,
      lastPageNumber: 1,
      createdAt: '2026-03-21',
    })
    getDocumentFileMock.mockResolvedValueOnce(new Blob(['fake pdf'], { type: 'application/pdf' }))
    renderPage()

    const explainButton = await screen.findByRole('button', { name: /explain this/i })
    expect(explainButton).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /select text/i }))
    expect(explainButton).not.toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /scroll to page 2/i }))
    expect(explainButton).not.toBeDisabled()
    await waitFor(() => {
      expect(updateDocumentProgressMock).toHaveBeenCalledTimes(1)
    })
  })

  it('cancels a pending progress save when scroll returns to the saved page', async () => {
    getDocumentMock.mockResolvedValueOnce({
      id: 1,
      filename: 'test.pdf',
      pageCount: 3,
      lastPageNumber: 1,
      createdAt: '2026-03-21',
    })
    getDocumentFileMock.mockResolvedValueOnce(new Blob(['fake pdf'], { type: 'application/pdf' }))
    renderPage()

    await screen.findByTestId('continuous-pdf-viewer')
    vi.useFakeTimers()
    try {
      fireEvent.click(screen.getByRole('button', { name: /scroll to page 2/i }))
      expect(screen.getByTestId('current-page')).toHaveTextContent('2')

      fireEvent.click(screen.getByRole('button', { name: /scroll to page 1/i }))
      expect(screen.getByTestId('current-page')).toHaveTextContent('1')

      await act(async () => {
        vi.advanceTimersByTime(600)
      })

      expect(updateDocumentProgressMock).not.toHaveBeenCalled()
    } finally {
      vi.useRealTimers()
    }
  })
})
