import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { analyzeText, createScanFromPage } from '@/lib/api'
import type { NuanceData } from '@/lib/types'
import DocumentPage from '../DocumentPage'

const getDocumentMock = vi.fn()
const getDocumentFileMock = vi.fn()
const getDocumentPageMock = vi.fn()
const updateDocumentProgressMock = vi.fn()

type PageChangeSource = 'scroll' | 'navigation'

type ContinuousPDFViewerMockProps = {
  currentPage: number
  onPageChange: (page: number, change: { source: PageChangeSource }) => void
  onTextSelect: (selectedText: string, pageNumber?: number) => void
}

type ExtractedTextReaderMockProps = {
  pageText: string
  pageNumber: number
  isLoading: boolean
  onTextSelect: (selectedText: string) => void
  onPageChange: (page: number, change: { source: 'navigation' }) => void
}

function nuance(meaning: string): NuanceData {
  return {
    translation: meaning,
    meaning,
    usageExample: 'example',
    usageTiming: 'timing',
    wordBreakdown: 'breakdown',
    alternativeMeaning: 'alternative',
  }
}

vi.mock('@/lib/api', () => ({
  getDocument: (...args: unknown[]) => getDocumentMock(...args),
  getDocumentFile: (...args: unknown[]) => getDocumentFileMock(...args),
  updateDocumentProgress: (...args: unknown[]) => updateDocumentProgressMock(...args),
  getDocumentPage: (...args: unknown[]) => getDocumentPageMock(...args),
  createScanFromPage: vi.fn(),
  analyzeText: vi.fn(),
  createAnnotation: vi.fn(),
  synthesizeSpeech: vi.fn(),
}))

vi.mock('@/components/documentpage/ExtractedTextReader', () => ({
  default: (props: ExtractedTextReaderMockProps) => React.createElement(
    'div',
    { 'data-testid': 'extracted-text-reader' },
    props.isLoading
      ? React.createElement('span', null, 'Loading reader text')
      : React.createElement('span', null, props.pageText),
    React.createElement('span', { 'data-testid': 'reader-current-page' }, props.pageNumber),
    React.createElement(
      'button',
      { type: 'button', onClick: () => props.onTextSelect('読者モード選択') },
      'Select reader text'
    ),
    React.createElement(
      'button',
      { type: 'button', onClick: () => props.onPageChange(props.pageNumber + 1, { source: 'navigation' }) },
      'Reader next page'
    )
  ),
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
      { type: 'button', onClick: () => props.onTextSelect('一ページ目の文', 1) },
      'Pick page 1 text'
    ),
    React.createElement(
      'button',
      { type: 'button', onClick: () => props.onTextSelect('二ページ目の文', 2) },
      'Pick page 2 text'
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
    getDocumentPageMock.mockResolvedValue({ pageNumber: 1, text: '抽出された本文', totalPages: 3 })
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

  it('explains against the page the text was selected on, not the page scrolled to', async () => {
    vi.mocked(createScanFromPage).mockImplementation(async (_documentId: number, pageNumber: number) => ({
      scanId: 100 + pageNumber,
    }))
    vi.mocked(analyzeText).mockResolvedValue(nuance('meaning'))
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
    fireEvent.click(screen.getByRole('button', { name: 'Pick page 1 text' }))
    fireEvent.click(screen.getByRole('button', { name: /scroll to page 2/i }))
    await waitFor(() => expect(screen.getByTestId('current-page')).toHaveTextContent('2'))

    fireEvent.click(explainButton)
    await waitFor(() => {
      expect(analyzeText).toHaveBeenCalledWith(expect.objectContaining({ textToAnalyze: '一ページ目の文', scanId: 101 }))
    })
    expect(createScanFromPage).toHaveBeenCalledWith(1, 1)
    const drawer = await screen.findByLabelText('Annotation drawer')
    fireEvent.keyDown(drawer, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByLabelText('Annotation drawer')).not.toBeInTheDocument())

    // A new selection on page 2 must not reuse page 1's scan.
    fireEvent.click(screen.getByRole('button', { name: 'Pick page 2 text' }))
    fireEvent.click(explainButton)
    await waitFor(() => {
      expect(analyzeText).toHaveBeenLastCalledWith(expect.objectContaining({ textToAnalyze: '二ページ目の文', scanId: 102 }))
    })
    expect(createScanFromPage).toHaveBeenLastCalledWith(1, 2)
  })

  it('drops a pending explanation when the reader changes view', async () => {
    let resolveAnalyze: (value: NuanceData) => void = () => undefined
    vi.mocked(createScanFromPage).mockResolvedValue({ scanId: 101 })
    vi.mocked(analyzeText).mockImplementation(() => new Promise((resolve) => { resolveAnalyze = resolve }))
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
    fireEvent.click(screen.getByRole('button', { name: 'Pick page 1 text' }))
    fireEvent.click(explainButton)
    await waitFor(() => expect(analyzeText).toHaveBeenCalledTimes(1))

    fireEvent.click(screen.getByRole('button', { name: 'Reader View' }))
    await act(async () => {
      resolveAnalyze(nuance('stale meaning'))
      // Let the mutation settle and any drawer state commit.
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    expect(screen.queryByLabelText('Annotation drawer')).not.toBeInTheDocument()
  })

  it('drops a pending explanation when a different text is selected', async () => {
    let resolveAnalyze: (value: NuanceData) => void = () => undefined
    vi.mocked(createScanFromPage).mockResolvedValue({ scanId: 101 })
    vi.mocked(analyzeText).mockImplementation(() => new Promise((resolve) => { resolveAnalyze = resolve }))
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
    fireEvent.click(screen.getByRole('button', { name: 'Pick page 1 text' }))
    fireEvent.click(explainButton)
    await waitFor(() => expect(analyzeText).toHaveBeenCalledTimes(1))

    // Re-reporting the same selection keeps the request; a new one replaces it.
    fireEvent.click(screen.getByRole('button', { name: 'Pick page 1 text' }))
    fireEvent.click(screen.getByRole('button', { name: /scroll to page 2/i }))
    fireEvent.click(screen.getByRole('button', { name: 'Pick page 2 text' }))
    await act(async () => {
      resolveAnalyze(nuance('stale meaning'))
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    expect(screen.queryByLabelText('Annotation drawer')).not.toBeInTheDocument()
  })

  it('records the open with the stored page so last opened updates without moving progress', async () => {
    getDocumentMock.mockResolvedValueOnce({
      id: 1,
      filename: 'test.pdf',
      pageCount: 3,
      lastPageNumber: 2,
      createdAt: '2026-03-21',
    })
    getDocumentFileMock.mockResolvedValueOnce(new Blob(['fake pdf'], { type: 'application/pdf' }))
    renderPage()

    await screen.findByTestId('continuous-pdf-viewer')
    await waitFor(() => expect(updateDocumentProgressMock).toHaveBeenCalledWith(1, 2))
    expect(updateDocumentProgressMock).toHaveBeenCalledTimes(1)
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
    await waitFor(() => expect(updateDocumentProgressMock).toHaveBeenCalledWith(1, 1))
    updateDocumentProgressMock.mockClear()
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

  it('switches to reader view and uses extracted text for selection', async () => {
    getDocumentMock.mockResolvedValueOnce({
      id: 1,
      filename: 'test.pdf',
      pageCount: 3,
      lastPageNumber: 1,
      createdAt: '2026-03-21',
    })
    getDocumentFileMock.mockResolvedValueOnce(new Blob(['fake pdf'], { type: 'application/pdf' }))
    getDocumentPageMock.mockResolvedValueOnce({ pageNumber: 1, text: '抽出された本文', totalPages: 3 })
    renderPage()

    const explainButton = await screen.findByRole('button', { name: /explain this/i })
    expect(explainButton).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /reader view/i }))

    expect(await screen.findByText('抽出された本文')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /select reader text/i }))

    expect(explainButton).not.toBeDisabled()
  })

  it('clears selected text when switching between PDF and reader views', async () => {
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
    fireEvent.click(screen.getByRole('button', { name: /select text/i }))
    expect(explainButton).not.toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /reader view/i }))

    expect(explainButton).toBeDisabled()
  })

  it('saves progress when reader view navigation changes page', async () => {
    getDocumentMock.mockResolvedValueOnce({
      id: 1,
      filename: 'test.pdf',
      pageCount: 3,
      lastPageNumber: 1,
      createdAt: '2026-03-21',
    })
    getDocumentFileMock.mockResolvedValueOnce(new Blob(['fake pdf'], { type: 'application/pdf' }))
    getDocumentPageMock.mockResolvedValue({ pageNumber: 1, text: '抽出された本文', totalPages: 3 })
    renderPage()

    await screen.findByRole('button', { name: /reader view/i })
    fireEvent.click(screen.getByRole('button', { name: /reader view/i }))
    await screen.findByTestId('extracted-text-reader')

    vi.useFakeTimers()
    try {
      fireEvent.click(screen.getByRole('button', { name: /reader next page/i }))

      expect(screen.getByTestId('reader-current-page')).toHaveTextContent('2')

      await act(async () => {
        vi.advanceTimersByTime(600)
      })

      expect(updateDocumentProgressMock).toHaveBeenCalledWith(1, 2)
    } finally {
      vi.useRealTimers()
    }
  })
})
