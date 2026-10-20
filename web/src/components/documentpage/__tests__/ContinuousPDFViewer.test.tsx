import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import ContinuousPDFViewer from '../ContinuousPDFViewer'
import * as pdfjsLib from 'pdfjs-dist'

const pdfMock = vi.hoisted(() => ({ renderedPages: new Array<number>() }))

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
  version: 'test-version',
  getDocument: vi.fn(() => ({
    promise: Promise.resolve({
      getPage: vi.fn((pageNumber: number) =>
        Promise.resolve({
          getViewport: vi.fn(() => ({ width: 400, height: 600 })),
          render: vi.fn(() => {
            pdfMock.renderedPages.push(pageNumber)
            return { promise: Promise.resolve() }
          }),
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
    pdfMock.renderedPages.length = 0
    window.getSelection()?.removeAllRanges()
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

  describe('Long documents', () => {
    it('renders pages near the current page or on screen, and releases others unless they hold the selection', async () => {
      const renderedPages = pdfMock.renderedPages
      const props = {
        pdfUrl: 'long.pdf',
        totalPages: 10,
        onPageChange: mockOnPageChange,
        onTextSelect: mockOnTextSelect,
        isLoading: false,
      }
      const { rerender } = render(<ContinuousPDFViewer {...props} currentPage={1} />)

      await waitFor(() => {
        expect([...renderedPages].sort((a, b) => a - b)).toEqual([1, 2, 3])
      })
      const pageNode = (page: number): HTMLElement => {
        const node = document.querySelector(`[data-page="${page}"] .pdf-page`)
        if (!(node instanceof HTMLElement)) {
          throw new Error(`Expected page ${page}`)
        }
        return node
      }
      const canvasFor = (page: number): HTMLCanvasElement => {
        const canvas = pageNode(page).querySelector('canvas')
        if (!(canvas instanceof HTMLCanvasElement)) {
          throw new Error(`Expected canvas for page ${page}`)
        }
        return canvas
      }
      // Unrendered pages take page 1's proportions (600/400) instead of collapsing.
      const placeholderWidth = Number.parseFloat(pageNode(9).style.width)
      expect(Number.parseFloat(pageNode(9).style.height)).toBeCloseTo(placeholderWidth * 1.5)

      // A page taller than the viewport renders while any of it is on screen, even below the
      // 30% threshold that moves the current page.
      const pageFiveWrapper = document.querySelector('[data-page="5"]')
      if (!(pageFiveWrapper instanceof HTMLElement)) {
        throw new Error('Expected page 5 wrapper')
      }
      const pageFiveObserver = MockIntersectionObserver.instances.find((observer) => observer.hasObserved(pageFiveWrapper))
      if (!pageFiveObserver) {
        throw new Error('Expected page 5 observer')
      }
      act(() => {
        pageFiveObserver.simulateIntersection(pageFiveWrapper, 0.2)
      })
      await waitFor(() => {
        expect(renderedPages).toContain(5)
      })
      act(() => {
        pageFiveObserver.simulateIntersection(pageFiveWrapper, 0, false)
      })

      // The reader selects text on page 2, then scrolls far away.
      const pageTwoTextLayer = pageNode(2).querySelector('.textLayer')
      if (!(pageTwoTextLayer instanceof HTMLElement)) {
        throw new Error('Expected page 2 text layer')
      }
      const selectedSpan = document.createElement('span')
      selectedSpan.textContent = '二ページ目の文章'
      pageTwoTextLayer.appendChild(selectedSpan)
      const selection = window.getSelection()
      if (!selectedSpan.firstChild || !selection) {
        throw new Error('Expected selection APIs to be available')
      }
      const range = document.createRange()
      range.selectNodeContents(selectedSpan.firstChild)
      selection.removeAllRanges()
      selection.addRange(range)
      fireEvent(document, new Event('selectionchange'))
      fireEvent.mouseUp(pageTwoTextLayer)
      expect(mockOnTextSelect).toHaveBeenLastCalledWith('二ページ目の文章', 2)

      rerender(<ContinuousPDFViewer {...props} currentPage={8} />)

      await waitFor(() => {
        expect([...renderedPages].sort((a, b) => a - b)).toEqual([1, 2, 3, 5, 6, 7, 8, 9, 10])
      })
      await waitFor(() => {
        expect(canvasFor(1).width).toBe(0)
      })
      // Released pages keep their measured size, so the scroll layout doesn't move.
      expect(pageNode(1).style.height).toBe('600px')
      expect(canvasFor(3).width).toBe(0)
      // The page holding the selection stays rendered and the selection still reports page 2.
      expect(canvasFor(2).width).toBe(400)
      expect(pageTwoTextLayer).toContainElement(selectedSpan)
      expect(mockOnTextSelect).not.toHaveBeenCalledWith('')

      // Once the selection clears, page 2 is released like any other far page.
      selection.removeAllRanges()
      fireEvent(document, new Event('selectionchange'))
      await waitFor(() => {
        expect(canvasFor(2).width).toBe(0)
      })
      expect(pageNode(2).style.height).toBe('600px')
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
        expect(node).toBeInstanceOf(HTMLElement)
        if (!(node instanceof HTMLElement)) {
          throw new Error('Expected text layer')
        }
        return node
      })
      const selectedSpan = document.createElement('span')
      selectedSpan.textContent = 'お母さん、ちょっと来て！'
      textLayer.appendChild(selectedSpan)
      const selectedTextNode = selectedSpan.firstChild
      const selection = window.getSelection()
      if (!selectedTextNode || !selection) {
        throw new Error('Expected selection APIs to be available')
      }
      const range = document.createRange()
      range.selectNodeContents(selectedTextNode)
      selection.removeAllRanges()
      selection.addRange(range)

      fireEvent.mouseUp(textLayer)

      expect(mockOnTextSelect).toHaveBeenCalledWith('お母さん、ちょっと来て！', 1)
      selection.removeAllRanges()
    })

    it('should pass selected text to onTextSelect on touch end', async () => {
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
        expect(node).toBeInstanceOf(HTMLElement)
        if (!(node instanceof HTMLElement)) {
          throw new Error('Expected text layer')
        }
        return node
      })
      const selectedSpan = document.createElement('span')
      selectedSpan.textContent = 'お母さん、ちょっと来て！'
      textLayer.appendChild(selectedSpan)
      const selectedTextNode = selectedSpan.firstChild
      const selection = window.getSelection()
      if (!selectedTextNode || !selection) {
        throw new Error('Expected selection APIs to be available')
      }
      const range = document.createRange()
      range.selectNodeContents(selectedTextNode)
      selection.removeAllRanges()
      selection.addRange(range)

      fireEvent.touchEnd(textLayer)

      expect(mockOnTextSelect).toHaveBeenCalledWith('お母さん、ちょっと来て！', 1)
      selection.removeAllRanges()
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

    it('registers a single document selectionchange listener for all pages', async () => {
      const addEventListenerSpy = vi.spyOn(document, 'addEventListener')

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

      await waitFor(() => {
        expect(document.querySelectorAll('.textLayer')).toHaveLength(3)
      })

      const selectionChangeListeners = addEventListenerSpy.mock.calls.filter(
        ([eventName]) => eventName === 'selectionchange'
      )
      expect(selectionChangeListeners).toHaveLength(1)
      addEventListenerSpy.mockRestore()
    })

    it('reports a selection inside the second page once', async () => {
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

      const pageTwoTextLayer = await waitFor(() => {
        const node = document.querySelectorAll('.textLayer').item(1)
        expect(node).toBeInstanceOf(HTMLElement)
        if (!(node instanceof HTMLElement)) {
          throw new Error('Expected page 2 text layer')
        }
        return node
      })
      const selectedSpan = document.createElement('span')
      selectedSpan.textContent = '二ページ目の文章'
      pageTwoTextLayer.appendChild(selectedSpan)
      const selectedTextNode = selectedSpan.firstChild
      const selection = window.getSelection()
      if (!selectedTextNode || !selection) {
        throw new Error('Expected selection APIs to be available')
      }
      const range = document.createRange()
      range.selectNodeContents(selectedTextNode)
      selection.removeAllRanges()
      selection.addRange(range)

      fireEvent(document, new Event('selectionchange'))

      expect(mockOnTextSelect).toHaveBeenCalledTimes(1)
      expect(mockOnTextSelect).toHaveBeenCalledWith('二ページ目の文章', 2)
    })

    it('keeps the PDF text layer selectable on touch devices', () => {
      const css = readFileSync(path.join(process.cwd(), 'src/index.css'), 'utf8')

      expect(css).toMatch(/\.pdf-page \.textLayer\s*\{[^}]*touch-action:\s*manipulation;/s)
      expect(css).toMatch(/\.pdf-page \.textLayer\s*\{[^}]*-webkit-touch-callout:\s*default;/s)
    })
  })
})
