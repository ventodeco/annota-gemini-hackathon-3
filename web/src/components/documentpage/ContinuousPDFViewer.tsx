import { useCallback, useEffect, useRef, useState, type CSSProperties, type ReactElement } from 'react'
import * as pdfjsLib from 'pdfjs-dist'
import { TextLayer } from 'pdfjs-dist'
import { ensurePdfjsWorker, getPdfDocumentLoadOptions } from '@/lib/pdf/initPdfWorker'
import { renderPdfPageToCanvas } from '@/lib/pdf/renderPage'

type PageChangeSource = 'scroll' | 'navigation'
type PageChange = { source: PageChangeSource }

interface ContinuousPDFViewerProps {
  pdfUrl: string
  currentPage: number
  totalPages: number
  onPageChange: (page: number, change: PageChange) => void
  onTextSelect: (selectedText: string) => void
  isLoading?: boolean
}

interface PageContainerProps {
  pageNumber: number
  pdfDoc: pdfjsLib.PDFDocumentProxy
  containerWidth: number
  onTextSelect: (selectedText: string) => void
  onVisible: (pageNumber: number, intersectionRatio: number) => void
  isVisible: boolean
}

type PdfPageStyle = CSSProperties & Record<'--scale-factor' | '--total-scale-factor' | '--user-unit', string>
const VISIBLE_PAGE_THRESHOLD = 0.3
const SCROLL_PAGE_CHANGE_DELAY_MS = 150

function isSelectionInsideTextLayer(selection: Selection, textLayer: HTMLDivElement): boolean {
  if (selection.rangeCount === 0) {
    return false
  }

  const range = selection.getRangeAt(0)
  let ancestor: Node | null = range.commonAncestorContainer
  if (ancestor.nodeType === Node.TEXT_NODE) {
    ancestor = ancestor.parentNode
  }

  return ancestor ? textLayer.contains(ancestor) : false
}

function PageRenderer({
  pageNumber,
  pdfDoc,
  containerWidth,
  onTextSelect,
  onVisible,
  isVisible,
}: PageContainerProps): ReactElement {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const textLayerRef = useRef<HTMLDivElement>(null)
  const textLayerInstance = useRef<TextLayer | null>(null)
  const [rendered, setRendered] = useState<boolean>(false)
  const [rendering, setRendering] = useState<boolean>(false)
  const [pageStyle, setPageStyle] = useState<PdfPageStyle | null>(null)
  const [hasSelectableText, setHasSelectableText] = useState<boolean | null>(null)
  const divRef = useRef<HTMLDivElement>(null)

  const renderPage = useCallback(async (): Promise<void> => {
    if (!canvasRef.current || !textLayerRef.current || !pdfDoc) return

    setRendering(true)
    try {
      const metrics = await renderPdfPageToCanvas({
        pdfDoc,
        pageNumber,
        containerWidth,
        canvas: canvasRef.current,
        textLayerDiv: textLayerRef.current,
        textLayerInstanceRef: textLayerInstance,
      })
      setPageStyle({
        width: `${metrics.width}px`,
        height: `${metrics.height}px`,
        '--scale-factor': `${metrics.scale}`,
        '--total-scale-factor': `${metrics.scale}`,
        '--user-unit': '1',
      })
      setHasSelectableText(metrics.hasSelectableText)
      setRendered(true)
    } catch (err) {
      console.error(`Failed to render page ${pageNumber}:`, err)
    } finally {
      setRendering(false)
    }
  }, [pdfDoc, pageNumber, containerWidth])

  useEffect(() => {
    if (!divRef.current) return

    const observer = new IntersectionObserver(
      (entries): void => {
        entries.forEach((entry) => {
          onVisible(pageNumber, entry.isIntersecting ? entry.intersectionRatio : 0)
        })
      },
      { threshold: VISIBLE_PAGE_THRESHOLD }
    )

    observer.observe(divRef.current)

    return () => {
      observer.disconnect()
    }
  }, [pageNumber, onVisible])

  useEffect(() => {
    if (isVisible && !rendered && !rendering) {
      renderPage()
    }
  }, [isVisible, rendered, rendering, renderPage])

  const handleTextSelection = useCallback((): void => {
    const selection = window.getSelection()
    onTextSelect(selection ? selection.toString() : '')
  }, [onTextSelect])

  useEffect(() => {
    const handleSelectionChange = (): void => {
      const selection = window.getSelection()
      if (!selection || selection.rangeCount === 0 || selection.toString().trim() === '') {
        return
      }

      const textLayer = textLayerRef.current
      if (!textLayer) {
        return
      }

      if (isSelectionInsideTextLayer(selection, textLayer)) {
        onTextSelect(selection.toString())
      }
    }

    document.addEventListener('selectionchange', handleSelectionChange)
    return () => document.removeEventListener('selectionchange', handleSelectionChange)
  }, [onTextSelect])

  return (
    <div
      ref={divRef}
      className="flex justify-center p-2"
      data-page={pageNumber}
    >
      <div className="pdf-page shadow-lg" style={pageStyle ?? undefined}>
        <canvas
          ref={canvasRef}
          className="block h-full w-full"
        />
        <div
          ref={textLayerRef}
          className="textLayer absolute inset-0"
          onMouseUp={handleTextSelection}
          onTouchEnd={handleTextSelection}
        />
        {rendering && (
          <div className="absolute inset-0 flex items-center justify-center bg-white bg-opacity-75">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900" />
          </div>
        )}
        {hasSelectableText === false && (
          <div className="absolute left-3 right-3 bottom-3 rounded-lg bg-white/90 px-3 py-2 text-center text-xs text-gray-600 shadow">
            Text selection is unavailable. Use OCR text for highlight and TTS.
          </div>
        )}
      </div>
    </div>
  )
}

export default function ContinuousPDFViewer({
  pdfUrl,
  currentPage,
  totalPages,
  onPageChange,
  onTextSelect,
  isLoading,
}: ContinuousPDFViewerProps): ReactElement {
  const containerRef = useRef<HTMLDivElement>(null)
  const [pdfDoc, setPdfDoc] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
  const [loadError, setLoadError] = useState<boolean>(false)
  const [visiblePages, setVisiblePages] = useState<Set<number>>(new Set([1]))
  const [containerWidth, setContainerWidth] = useState<number>(400)
  const visiblePageRatiosRef = useRef<Map<number, number>>(new Map())
  const scrollPageChangeTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(null)
  const currentPageRef = useRef(currentPage)
  const didInitialScrollRef = useRef(false)

  useEffect(() => {
    currentPageRef.current = currentPage
  }, [currentPage])

  useEffect(() => {
    if (!pdfUrl) return

    const loadPdf = async (): Promise<void> => {
      setLoadError(false)
      setPdfDoc(null)
      setVisiblePages(new Set([1]))
      visiblePageRatiosRef.current.clear()
      didInitialScrollRef.current = false
      try {
        ensurePdfjsWorker()
        const loadingTask = pdfjsLib.getDocument(getPdfDocumentLoadOptions(pdfUrl))
        const pdf = await loadingTask.promise
        setPdfDoc(pdf)
      } catch (err) {
        console.error('Failed to load PDF:', err)
        setLoadError(true)
      }
    }
    loadPdf()
  }, [pdfUrl])

  useEffect(() => {
    return () => {
      if (scrollPageChangeTimerRef.current !== null) {
        window.clearTimeout(scrollPageChangeTimerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    const updateWidth = (): void => {
      if (containerRef.current) {
        setContainerWidth(containerRef.current.clientWidth)
      }
    }

    updateWidth()
    window.addEventListener('resize', updateWidth)
    return () => window.removeEventListener('resize', updateWidth)
  }, [])

  useEffect(() => {
    if (!pdfDoc || !containerRef.current || currentPage < 1 || didInitialScrollRef.current) return
    const pageNode = containerRef.current.querySelector(`[data-page="${currentPage}"]`)
    if (pageNode instanceof HTMLElement && typeof pageNode.scrollIntoView === 'function') {
      pageNode.scrollIntoView({ block: 'start' })
    }
    didInitialScrollRef.current = true
  }, [currentPage, pdfDoc])

  const handlePageVisible = useCallback(
    (pageNumber: number, intersectionRatio: number) => {
      if (intersectionRatio > VISIBLE_PAGE_THRESHOLD) {
        visiblePageRatiosRef.current.set(pageNumber, intersectionRatio)
        setVisiblePages((prev) => {
          if (prev.has(pageNumber)) {
            return prev
          }
          const next = new Set(prev)
          next.add(pageNumber)
          return next
        })
      } else {
        visiblePageRatiosRef.current.delete(pageNumber)
      }

      if (scrollPageChangeTimerRef.current !== null) {
        window.clearTimeout(scrollPageChangeTimerRef.current)
      }

      scrollPageChangeTimerRef.current = window.setTimeout(() => {
        let dominantPage = currentPageRef.current
        let dominantRatio = VISIBLE_PAGE_THRESHOLD
        visiblePageRatiosRef.current.forEach((ratio, candidatePage) => {
          if (ratio > dominantRatio) {
            dominantPage = candidatePage
            dominantRatio = ratio
          }
        })

        if (dominantPage !== currentPageRef.current) {
          onPageChange(dominantPage, { source: 'scroll' })
        }
      }, SCROLL_PAGE_CHANGE_DELAY_MS)
    },
    [onPageChange]
  )

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      </div>
    )
  }

  if (!pdfUrl) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p className="text-gray-500">No PDF loaded</p>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="flex-1 flex items-center justify-center p-6 text-center">
        <div>
          <p className="font-medium text-gray-900">Unable to load this PDF.</p>
          <p className="mt-2 text-sm text-gray-500">
            Try uploading a text-based PDF or re-open the document.
          </p>
        </div>
      </div>
    )
  }

  if (!pdfDoc) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p className="text-gray-500">Loading PDF...</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col flex-1">
      <div
        ref={containerRef}
        className="flex-1 overflow-y-auto"
      >
        {Array.from({ length: totalPages }, (_, index) => {
          const pageNum = index + 1
          return (
            <PageRenderer
              key={pageNum}
              pageNumber={pageNum}
              pdfDoc={pdfDoc}
              containerWidth={containerWidth}
              onTextSelect={onTextSelect}
              onVisible={handlePageVisible}
              isVisible={visiblePages.has(pageNum)}
            />
          )
        })}
      </div>
      <div className="fixed bottom-24 right-4 bg-white bg-opacity-90 px-3 py-1 rounded-full text-sm text-gray-600 shadow">
        Page {currentPage} of {totalPages}
      </div>
    </div>
  )
}
