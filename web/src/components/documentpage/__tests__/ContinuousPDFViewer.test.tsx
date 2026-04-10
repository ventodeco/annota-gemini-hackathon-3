import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import ContinuousPDFViewer from '../ContinuousPDFViewer'

const mockOnPageChange = vi.fn()
const mockOnTextSelect = vi.fn()

class MockIntersectionObserver implements IntersectionObserver {
  readonly root: Element | null = null
  readonly rootMargin: string = ''
  readonly thresholds: ReadonlyArray<number> = []
  private callback: IntersectionObserverCallback
  private elements: Set<Element> = new Set()

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
  }

  observe(element: Element): void {
    this.elements.add(element)
  }

  unobserve(element: Element): void {
    this.elements.delete(element)
  }

  disconnect(): void {
    this.elements.clear()
  }

  takeRecords(): IntersectionObserverEntry[] {
    return []
  }

  simulateIntersection(entry: IntersectionObserverEntry): void {
    this.callback([entry], this)
  }
}

let mockIntersectionObserver: typeof MockIntersectionObserver

vi.mock('pdfjs-dist', () => ({
  getDocument: vi.fn(() => ({
    promise: Promise.resolve({
      getPage: vi.fn(() =>
        Promise.resolve({
          getViewport: vi.fn(() => ({ width: 400, height: 600 })),
          render: vi.fn(() => ({
            promise: Promise.resolve(),
          })),
          getTextContent: vi.fn(() =>
            Promise.resolve({
              items: [],
            }),
          ),
        }),
      ),
      numPages: 3,
    }),
  })),
  GlobalWorkerOptions: {
    workerSrc: '',
  },
  TextLayer: class {
    render = vi.fn(() => Promise.resolve())
    cancel = vi.fn()
  },
}))

describe('ContinuousPDFViewer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockIntersectionObserver = MockIntersectionObserver
    vi.stubGlobal('IntersectionObserver', mockIntersectionObserver)
  })

  describe('Rendering', () => {
    it('should show loading spinner when isLoading is true', () => {
      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={3}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={true}
        />
      )
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('should show "No PDF loaded" when pdfUrl is empty', () => {
      render(
        <ContinuousPDFViewer
          pdfUrl=""
          currentPage={1}
          totalPages={3}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )
      expect(screen.getByText('No PDF loaded')).toBeInTheDocument()
    })

    it('should show "Loading PDF..." when pdfDoc is not yet loaded', () => {
      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={3}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )
      expect(screen.getByText('Loading PDF...')).toBeInTheDocument()
    })
  })

  describe('Props', () => {
    it('should render correct page indicator with currentPage and totalPages', async () => {
      vi.stubGlobal('IntersectionObserver', mockIntersectionObserver)

      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={2}
          totalPages={5}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )
      await waitFor(() => {
        expect(screen.getByText(/Page 2 of 5/)).toBeInTheDocument()
      })
    })
  })

  describe('Edge Cases', () => {
    it('should handle single page document', async () => {
      vi.stubGlobal('IntersectionObserver', mockIntersectionObserver)

      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={1}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )
      await waitFor(() => {
        expect(screen.getByText(/Page 1 of 1/)).toBeInTheDocument()
      })
    })

    it('should handle zero totalPages gracefully', async () => {
      vi.stubGlobal('IntersectionObserver', mockIntersectionObserver)

      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={0}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )
      await waitFor(() => {
        expect(screen.getByText(/Page 1 of 0/)).toBeInTheDocument()
      })
    })
  })

  describe('Text selection', () => {
    it('should render the PDF.js-compatible textLayer container', async () => {
      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={1}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )

      await waitFor(() => {
        expect(document.querySelector('.textLayer')).toBeInTheDocument()
      })
    })

    it('should pass selected text to onTextSelect on mouse up', async () => {
      const getSelectionSpy = vi.spyOn(window, 'getSelection')
      getSelectionSpy.mockReturnValue({
        toString: () => 'お母さん、ちょっと来て！',
      } as Selection)

      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={1}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )

      const textLayer = await waitFor(() => {
        const node = document.querySelector('.textLayer')
        expect(node).toBeInTheDocument()
        return node
      })

      fireEvent.mouseUp(textLayer as Element)

      expect(mockOnTextSelect).toHaveBeenCalledWith('お母さん、ちょっと来て！')
      getSelectionSpy.mockRestore()
    })

    it('should clear selection when no text is selected', async () => {
      const getSelectionSpy = vi.spyOn(window, 'getSelection')
      getSelectionSpy.mockReturnValue({
        toString: () => '',
      } as Selection)

      render(
        <ContinuousPDFViewer
          pdfUrl="test.pdf"
          currentPage={1}
          totalPages={1}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )

      const textLayer = await waitFor(() => {
        const node = document.querySelector('.textLayer')
        expect(node).toBeInTheDocument()
        return node
      })

      fireEvent.mouseUp(textLayer as Element)

      expect(mockOnTextSelect).toHaveBeenCalledWith('')
      getSelectionSpy.mockRestore()
    })
  })
})
