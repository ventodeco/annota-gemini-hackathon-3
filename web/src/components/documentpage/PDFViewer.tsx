import { useEffect, useRef, useState, useCallback } from 'react'
import * as pdfjsLib from 'pdfjs-dist'
import { TextLayer } from 'pdfjs-dist'
import PageNavigator from './PageNavigator'

pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url,
).toString()

interface PDFViewerProps {
  pdfUrl: string
  currentPage: number
  totalPages: number
  onPrev: () => void
  onNext: () => void
  onTextSelect: (selectedText: string) => void
  isLoading?: boolean
}

export default function PDFViewer({
  pdfUrl,
  currentPage,
  totalPages,
  onPrev,
  onNext,
  onTextSelect,
  isLoading,
}: PDFViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const textLayerRef = useRef<HTMLDivElement>(null)
  const [pdfDoc, setPdfDoc] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
  const [pageRendering, setPageRendering] = useState(false)
  const [renderedPage, setRenderedPage] = useState<number>(0)
  const textLayerInstance = useRef<TextLayer | null>(null)

  // Touch gesture state
  const touchStartX = useRef<number>(0)
  const touchEndX = useRef<number>(0)

  // Initialize PDF document
  useEffect(() => {
    if (!pdfUrl) return

    const loadPdf = async () => {
      try {
        const loadingTask = pdfjsLib.getDocument({
          url: pdfUrl,
          cMapUrl: 'https://unpkg.com/pdfjs-dist@5.5.207/cmaps/',
          cMapPacked: true,
        })
        const pdf = await loadingTask.promise
        setPdfDoc(pdf)
      } catch (err) {
        console.error('Failed to load PDF:', err)
      }
    }
    loadPdf()
  }, [pdfUrl])

  // Render a specific page
  const renderPage = useCallback(async (pageNum: number) => {
    if (!pdfDoc || !canvasRef.current || !textLayerRef.current) return

    setPageRendering(true)
    try {
      const page = await pdfDoc.getPage(pageNum)
      const canvas = canvasRef.current

      const containerWidth = containerRef.current?.clientWidth || 400
      const viewport = page.getViewport({ scale: 1 })
      const scale = containerWidth / viewport.width
      const scaledViewport = page.getViewport({ scale })

      canvas.height = scaledViewport.height
      canvas.width = scaledViewport.width

      // Render PDF page to canvas
      await page.render({
        canvas,
        viewport: scaledViewport,
      }).promise

      // Clean up previous text layer
      if (textLayerInstance.current) {
        textLayerInstance.current.cancel()
        textLayerInstance.current = null
      }

      // Render text layer for selection
      const textContent = await page.getTextContent()
      const textLayerDiv = textLayerRef.current
      textLayerDiv.innerHTML = ''
      textLayerDiv.style.width = `${canvas.width}px`
      textLayerDiv.style.height = `${canvas.height}px`

      // Create new text layer instance
      const textLayer = new TextLayer({
        textContentSource: textContent,
        container: textLayerDiv,
        viewport: scaledViewport,
      })
      textLayerInstance.current = textLayer
      await textLayer.render()

      setRenderedPage(pageNum)
    } catch (err) {
      console.error('Failed to render page:', err)
    } finally {
      setPageRendering(false)
    }
  }, [pdfDoc])

  // Re-render when page changes
  useEffect(() => {
    if (pdfDoc && currentPage !== renderedPage && !pageRendering) {
      renderPage(currentPage)
    }
  }, [pdfDoc, currentPage, renderedPage, pageRendering, renderPage])

  // Initial render when PDF is loaded
  useEffect(() => {
    if (pdfDoc && renderedPage === 0 && !pageRendering) {
      renderPage(currentPage)
    }
  }, [pdfDoc, renderPage, currentPage, renderedPage, pageRendering])

  // Touch handlers for swipe navigation
  const handleTouchStart = (e: React.TouchEvent) => {
    touchStartX.current = e.touches[0].clientX
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    touchEndX.current = e.touches[0].clientX
  }

  const handleTouchEnd = () => {
    const deltaX = touchEndX.current - touchStartX.current
    const minSwipeDistance = 50

    if (Math.abs(deltaX) > minSwipeDistance) {
      if (deltaX > 0 && currentPage > 1) {
        // Swipe right -> previous page
        onPrev()
      } else if (deltaX < 0 && currentPage < totalPages) {
        // Swipe left -> next page
        onNext()
      }
    }
  }

  // Handle text selection
  const handleTextSelection = () => {
    const selection = window.getSelection()
    if (selection && selection.toString().trim()) {
      onTextSelect(selection.toString())
    }
  }

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
        className="flex-1 overflow-y-auto relative"
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        <div className="flex justify-center p-2">
          <div className="relative">
            <canvas
              ref={canvasRef}
              className="max-w-full shadow-lg"
              style={{ display: 'block' }}
            />
            <div
              ref={textLayerRef}
              className="absolute top-0 left-0 text-layer"
              onMouseUp={handleTextSelection}
              onTouchEnd={handleTextSelection}
              style={{
                color: 'transparent',
                pointerEvents: 'auto',
                whiteSpace: 'pre-wrap',
              }}
            />
            {pageRendering && (
              <div className="absolute inset-0 flex items-center justify-center bg-white bg-opacity-75">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900" />
              </div>
            )}
          </div>
        </div>
      </div>
      <PageNavigator
        currentPage={currentPage}
        totalPages={totalPages}
        onPrev={onPrev}
        onNext={onNext}
      />
    </div>
  )
}
