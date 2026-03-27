import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import DocumentPage from '../DocumentPage'

const getDocumentMock = vi.fn()
const getDocumentPageMock = vi.fn()

vi.mock('@/lib/api', () => ({
  getDocument: (...args: unknown[]) => getDocumentMock(...args),
  getDocumentPage: (...args: unknown[]) => getDocumentPageMock(...args),
  createScanFromPage: vi.fn(),
  analyzeText: vi.fn(),
  createAnnotation: vi.fn(),
  synthesizeSpeech: vi.fn(),
  getAuthToken: () => 'mock-token',
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

  it('renders page list with correct number of pages', async () => {
    getDocumentMock.mockResolvedValueOnce({ id: 1, filename: 'test.pdf', pageCount: 3, createdAt: '2026-03-21' })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('test.pdf')).toBeInTheDocument()
      expect(screen.getByText('Page 1')).toBeInTheDocument()
      expect(screen.getByText('Page 2')).toBeInTheDocument()
      expect(screen.getByText('Page 3')).toBeInTheDocument()
    })
  })

  it('shows reader view when page selected', async () => {
    getDocumentMock.mockResolvedValueOnce({ id: 1, filename: 'test.pdf', pageCount: 3, createdAt: '2026-03-21' })
    getDocumentPageMock.mockResolvedValueOnce({ pageNumber: 1, text: 'Content of page 1', totalPages: 3 })
    renderPage()
    await waitFor(() => expect(screen.getByText('Page 1')).toBeInTheDocument())
    await userEvent.click(screen.getByText('Page 1'))
    await waitFor(() => {
      expect(screen.getByText('Content of page 1')).toBeInTheDocument()
      expect(screen.getByText('1 / 3')).toBeInTheDocument()
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
