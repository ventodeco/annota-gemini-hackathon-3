import { useEffect, useRef, useState, useCallback } from 'react'
import * as pdfjsLib from 'pdfjs-dist'
import { TextLayer } from 'pdfjs-dist'

pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url,
).toString()

interface ContinuousPDFViewerProps {
  pdfUrl: string
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
  onTextSelect: (selectedText: string) => void
  isLoading?: boolean
}

interface PageContainerProps {
  pageNumber: number
  pdfDoc: pdfjsLib.PDFDocumentProxy
  containerWidth: number
  onTextSelect: (selectedText: string) => void
  onVisible: (pageNumber: number) => void
  isVisible: boolean
}

function PageRenderer({
  pageNumber,
  pdfDoc,
  containerWidth,
  onTextSelect,
  onVisible,
  isVisible,
}: PageContainerProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const textLayerRef = useRef<HTMLDivElement>(null)
  const textLayerInstance = useRef<TextLayer | null>(null)
  const [rendered, setRendered] = useState(false)
  const [rendering, setRendering] = useState(false)
  const observerRef = useRef<IntersectionObserver | null>(null)
  const divRef = useRef<HTMLDivElement>(null)

  const renderPage = useCallback(async () => {
    if (!canvasRef.current || !textLayerRef.current || !pdfDoc) return

    setRendering(true)
    try {
      const page = await pdfDoc.getPage(pageNumber)
      const canvas = canvasRef.current

      const viewport = page.getViewport({ scale: 1 })
      const scale = containerWidth / viewport.width
      const scaledViewport = page.getViewport({ scale })

      canvas.height = scaledViewport.height
      canvas.width = scaledViewport.width

      await page.render({
        canvas,
        viewport: scaledViewport,
      }).promise

      if (textLayerInstance.current) {
        textLayerInstance.current.cancel()
        textLayerInstance.current = null
      }

      const textContent = await page.getTextContent()
      const textLayerDiv = textLayerRef.current
      textLayerDiv.innerHTML = ''
      textLayerDiv.style.width = `${canvas.width}px`
      textLayerDiv.style.height = `${canvas.height}px`

      const textLayer = new TextLayer({
        textContentSource: textContent,
        container: textLayerDiv,
        viewport: scaledViewport,
      })
      textLayerInstance.current = textLayer
      await textLayer.render()

      setRendered(true)
    } catch (err) {
      console.error(`Failed to render page ${pageNumber}:`, err)
    } finally {
      setRendering(false)
    }
  }, [pdfDoc, pageNumber, containerWidth])

  useEffect(() => {
    if (!divRef.current) return

    observerRef.current = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting && entry.intersectionRatio > 0.3) {
            onVisible(pageNumber)
          }
        })
      },
      { threshold: 0.3 }
    )

    observerRef.current.observe(divRef.current)

    return () => {
      if (observerRef.current) {
        observerRef.current.disconnect()
      }
    }
  }, [pageNumber, onVisible])

  useEffect(() => {
    if (isVisible && !rendered && !rendering) {
      renderPage()
    }
  }, [isVisible, rendered, rendering, renderPage])

  const handleTextSelection = () => {
    const selection = window.getSelection()
    if (selection && selection.toString().trim()) {
      onTextSelect(selection.toString())
    }
  }

  return (
    <div
      ref={divRef}
      className="flex justify-center p-2"
      data-page={pageNumber}
    >
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
        {rendering && (
          <div className="absolute inset-0 flex items-center justify-center bg-white bg-opacity-75">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900" />
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
}: ContinuousPDFViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [pdfDoc, setPdfDoc] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
  const [visiblePages, setVisiblePages] = useState<Set<number>>(new Set([1]))
  const [containerWidth, setContainerWidth] = useState(400)

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

  useEffect(() => {
    const updateWidth = () => {
      if (containerRef.current) {
        setContainerWidth(containerRef.current.clientWidth)
      }
    }

    updateWidth()
    window.addEventListener('resize', updateWidth)
    return () => window.removeEventListener('resize', updateWidth)
  }, [])

  const handlePageVisible = useCallback(
    (pageNumber: number) => {
      setVisiblePages((prev) => {
        const next = new Set(prev)
        next.add(pageNumber)
        return next
      })
      if (pageNumber !== currentPage) {
        onPageChange(pageNumber)
      }
    },
    [currentPage, onPageChange]
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
        {Array.from({ length: totalPages }, (_, i) => i + 1).map((pageNum) => (
          <PageRenderer
            key={pageNum}
            pageNumber={pageNum}
            pdfDoc={pdfDoc}
            containerWidth={containerWidth}
            onTextSelect={onTextSelect}
            onVisible={handlePageVisible}
            isVisible={visiblePages.has(pageNum)}
          />
        ))}
      </div>
      <div className="fixed bottom-24 right-4 bg-white bg-opacity-90 px-3 py-1 rounded-full text-sm text-gray-600 shadow">
        Page {currentPage} of {totalPages}
      </div>
    </div>
  )
}
