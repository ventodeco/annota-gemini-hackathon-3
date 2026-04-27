import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import ContinuousPDFViewer from '../ContinuousPDFViewer'
import * as pdfjsLib from 'pdfjs-dist'

const mockOnPageChange = vi.fn()
const mockOnTextSelect = vi.fn()

class MockIntersectionObserver implements IntersectionObserver {
  static instances: MockIntersectionObserver[] = []

  readonly root: Element | null = null
  readonly rootMargin: string = ''
  readonly thresholds: ReadonlyArray<number> = []
  private callback: IntersectionObserverCallback
  private elements: Set<Element> = new Set()

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    MockIntersectionObserver.instances.push(this)
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

  hasObserved(element: Element): boolean {
    return this.elements.has(element)
  }

  simulateIntersection(target: Element, intersectionRatio: number, isIntersecting = true): void {
    const rect = target.getBoundingClientRect()
    const entry: IntersectionObserverEntry = {
      boundingClientRect: rect,
      intersectionRatio,
      intersectionRect: rect,
      isIntersecting,
      rootBounds: null,
      target,
      time: 0,
    }
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
    MockIntersectionObserver.instances = []
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

    it('should show recoverable PDF load error when PDF.js rejects', async () => {
      vi.mocked(pdfjsLib.getDocument).mockImplementationOnce(() => {
        throw new Error('bad pdf')
      })

      render(
        <ContinuousPDFViewer
          pdfUrl="broken.pdf"
          currentPage={1}
          totalPages={3}
          onPageChange={mockOnPageChange}
          onTextSelect={mockOnTextSelect}
          isLoading={false}
        />
      )

      expect(await screen.findByText('Unable to load this PDF.')).toBeInTheDocument()
      expect(screen.getByText('Try uploading a text-based PDF or re-open the document.')).toBeInTheDocument()
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

    it('reports only the dominant visible page after scroll settles', async () => {
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

      const pageTwo = await waitFor(() => {
        const node = document.querySelector('[data-page="2"]')
        expect(node).toBeInstanceOf(HTMLElement)
        if (!(node instanceof HTMLElement)) {
          throw new Error('Expected page 2 to render')
        }
        return node
      })
      const pageThree = await waitFor(() => {
        const node = document.querySelector('[data-page="3"]')
        expect(node).toBeInstanceOf(HTMLElement)
        if (!(node instanceof HTMLElement)) {
          throw new Error('Expected page 3 to render')
        }
        return node
      })

      const pageTwoObserver = MockIntersectionObserver.instances.find((observer) => observer.hasObserved(pageTwo))
      const pageThreeObserver = MockIntersectionObserver.instances.find((observer) => observer.hasObserved(pageThree))
      if (!pageTwoObserver || !pageThreeObserver) {
        throw new Error('Expected page observers to be registered')
      }

      vi.useFakeTimers()
      try {
        pageTwoObserver.simulateIntersection(pageTwo, 0.45)
        pageThreeObserver.simulateIntersection(pageThree, 0.75)

        expect(mockOnPageChange).not.toHaveBeenCalled()

        await act(async () => {
          vi.advanceTimersByTime(160)
        })

        expect(mockOnPageChange).toHaveBeenCalledTimes(1)
        expect(mockOnPageChange).toHaveBeenCalledWith(3, { source: 'scroll' })
      } finally {
        vi.useRealTimers()
      }
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

    it('shows a text unavailable notice when the PDF page has no extractable text', async () => {
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

      expect(await screen.findByText(/Text selection is unavailable/i)).toBeInTheDocument()
    })

    it('should pass selected text to onTextSelect on mouse up', async () => {
      const selectionHost = document.createElement('div')
      selectionHost.textContent = 'お母さん、ちょっと来て！'
      document.body.appendChild(selectionHost)
      const selectedTextNode = selectionHost.firstChild
      const selection = window.getSelection()
      if (!selectedTextNode || !selection) {
        throw new Error('Expected selection APIs to be available')
      }
      const range = document.createRange()
      range.selectNodeContents(selectedTextNode)
      selection.removeAllRanges()
      selection.addRange(range)

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

      fireEvent.mouseUp(textLayer)

      expect(mockOnTextSelect).toHaveBeenCalledWith('お母さん、ちょっと来て！')
      selection.removeAllRanges()
      selectionHost.remove()
    })

    it('should clear selection when no text is selected', async () => {
      window.getSelection()?.removeAllRanges()

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

      fireEvent.mouseUp(textLayer)

      expect(mockOnTextSelect).toHaveBeenCalledWith('')
    })
  })
})
