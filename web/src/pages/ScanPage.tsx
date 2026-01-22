import { useParams } from 'react-router-dom'
import { useState } from 'react'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useScan } from '@/hooks/useScans'
import { useAnalyzeText, useCreateAnnotation } from '@/hooks/useAnnotations'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import type { Annotation } from '@/lib/types'
import { useTextSelection } from '@/hooks/useTextSelection'
import { getScanImageUrl, formatDate } from '@/lib/api'

export default function ScanPage() {
  const { id } = useParams<{ id: string }>()
  const scanId = id ? parseInt(id, 10) : 0
  const { data: scan, isLoading, error } = useScan(scanId)
  const analyzeText = useAnalyzeText()
  const createAnnotation = useCreateAnnotation()
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)
  const [contextText, setContextText] = useState('')
  const { selectedText, handleSelection, clearSelection } = useTextSelection()

  const handleTextSelect = () => {
    const selection = window.getSelection()
    if (!selection || selection.toString().trim() === '') {
      clearSelection()
      return
    }
    handleSelection(selection.toString())

    const range = selection.getRangeAt(0)
    const context = range.endContainer.textContent || ''
    setContextText(context)
  }

  const handleExplain = async () => {
    if (!selectedText) return

    setIsLoadingAnnotation(true)

    try {
      const result = await analyzeText.mutateAsync({
        textToAnalyze: selectedText,
        context: contextText,
      })

      const annotation: Annotation = {
        id: Date.now(),
        user_id: 0,
        scan_id: scanId,
        highlighted_text: selectedText,
        context_text: contextText,
        nuance_data: result,
        is_bookmarked: true,
        created_at: new Date().toISOString(),
      }

      setCurrentAnnotation(annotation)
      setIsDrawerOpen(true)
    } catch (err) {
      console.error('Failed to analyze text:', err)
      alert('Failed to analyze text. Please try again.')
    } finally {
      setIsLoadingAnnotation(false)
    }
  }

  const handleSaveAnnotation = async () => {
    if (!currentAnnotation || !scan) return

    try {
      await createAnnotation.mutateAsync({
        scanId: scan.id,
        highlightedText: currentAnnotation.highlighted_text,
        contextText: currentAnnotation.context_text,
        nuanceData: currentAnnotation.nuance_data,
      })
      setIsDrawerOpen(false)
      setCurrentAnnotation(null)
      clearSelection()
    } catch (err) {
      console.error('Failed to save annotation:', err)
      alert('Failed to save annotation. Please try again.')
    }
  }

  const handleDrawerClose = () => {
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    clearSelection()
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Scan Result" />
        <div className="flex-1 flex items-center justify-center p-6">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
        </div>
      </div>
    )
  }

  if (error || !scan) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Scan Result" />
        <div className="flex-1 flex items-center justify-center p-6">
          <p className="text-gray-600">Scan not found</p>
        </div>
      </div>
    )
  }

  const imageUrl = getScanImageUrl(scan.image_url)

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header title="Scan Result" />
      <ScrollArea className="flex-1">
        <div className="p-6">
          {imageUrl && (
            <img
              src={imageUrl}
              alt="Scanned document"
              className="w-full mb-6 rounded-lg"
            />
          )}
          {scan.full_ocr_text && (
            <p
              className="text-base leading-relaxed text-gray-900 whitespace-pre-wrap"
              onMouseUp={handleTextSelect}
              onTouchEnd={handleTextSelect}
            >
              {scan.full_ocr_text}
            </p>
          )}
          <div className="mt-4 text-sm text-gray-500">
            <p>Detected language: {scan.detected_language || 'Unknown'}</p>
            <p>Created: {formatDate(scan.created_at)}</p>
          </div>
        </div>
      </ScrollArea>
      <BottomActionBar
        disabled={!selectedText || isLoadingAnnotation}
        isLoading={isLoadingAnnotation || analyzeText.isPending}
        onExplain={handleExplain}
      />
      <AnnotationDrawer
        isOpen={isDrawerOpen}
        onClose={handleDrawerClose}
        annotation={currentAnnotation}
        onSave={handleSaveAnnotation}
        isSaving={createAnnotation.isPending}
      />
    </div>
  )
}
