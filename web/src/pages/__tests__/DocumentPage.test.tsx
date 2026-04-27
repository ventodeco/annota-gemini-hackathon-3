import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import DocumentPage from '../DocumentPage'

const getDocumentMock = vi.fn()
const getDocumentFileMock = vi.fn()

vi.mock('@/lib/api', () => ({
  getDocument: (...args: unknown[]) => getDocumentMock(...args),
  getDocumentFile: (...args: unknown[]) => getDocumentFileMock(...args),
  getDocumentPage: vi.fn(),
  createScanFromPage: vi.fn(),
  analyzeText: vi.fn(),
  createAnnotation: vi.fn(),
  synthesizeSpeech: vi.fn(),
  getAuthToken: () => 'mock-token',
}))

vi.mock('@/components/documentpage/ContinuousPDFViewer', () => ({
  default: () => React.createElement('div', { 'data-testid': 'continuous-pdf-viewer' }, 'Continuous PDF Viewer'),
}))

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => ({ isAuthenticated: true, isLoading: false, user: { id: 1 } }),
}))

describe('DocumentPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
})
