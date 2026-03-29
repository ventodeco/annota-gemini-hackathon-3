import { useParams } from 'react-router-dom'
import { useState, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import ContinuousPDFViewer from '@/components/documentpage/ContinuousPDFViewer'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import { useDocument } from '@/hooks/useDocument'
import { useTextSelection } from '@/hooks/useTextSelection'
import { useAnalyzeText, useCreateAnnotation, useSynthesizeSpeech } from '@/hooks/useAnnotations'
import { createScanFromPage, getDocumentFile } from '@/lib/api'
import { useSpeechPlayback } from '@/hooks/useSpeechPlayback'
import type { Annotation } from '@/lib/types'

const MAX_ANNOTATION_VERSIONS = 2

export default function DocumentPage() {
  const { id } = useParams<{ id: string }>()
  const documentId = id ? parseInt(id, 10) : undefined

  const { data: document, isLoading, error } = useDocument(documentId)
  const [currentPage, setCurrentPage] = useState(1)
  const [pdfUrl, setPdfUrl] = useState<string>('')
  const [isPdfLoading, setIsPdfLoading] = useState(false)
  const objectUrlRef = useRef<string>('')

  const { selectedText, handleSelection, clearSelection } = useTextSelection()
  const analyzeText = useAnalyzeText()
  const createAnnotation = useCreateAnnotation()
  const synthesizeSpeech = useSynthesizeSpeech()
  const speech = useSpeechPlayback()

  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [annotationVersion, setAnnotationVersion] = useState(1)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)
  const [contextText, setContextText] = useState('')
  const [bridgeScanId, setBridgeScanId] = useState<number | null>(null)

  useEffect(() => {
    if (!documentId || !document) return

    if (!pdfUrl) {
      setIsPdfLoading(true)
      const blobPromise = getDocumentFile(documentId)
      blobPromise
        .then((blob) => {
          const url = URL.createObjectURL(blob)
          objectUrlRef.current = url
          setPdfUrl(url)
        })
        .catch((err) => {
          console.error('Failed to load PDF:', err)
          toast.error('Failed to load PDF. Please try again.')
        })
        .finally(() => {
          setIsPdfLoading(false)
        })
    }

    return () => {
      if (objectUrlRef.current) {
        URL.revokeObjectURL(objectUrlRef.current)
      }
    }
  }, [documentId, document])

  useEffect(() => {
    return () => {
      speech.stop()
    }
  }, [speech])

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    clearSelection()
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setBridgeScanId(null)
  }

  const handleTextSelect = () => {
    const selection = window.getSelection()
    if (!selection || selection.toString().trim() === '') {
      clearSelection()
      setContextText('')
      return
    }
    handleSelection(selection.toString())

    const range = selection.getRangeAt(0)
    const context = range.endContainer.textContent || ''
    setContextText(context)
  }

  const handleExplain = async () => {
    if (!selectedText || !documentId) return

    setIsLoadingAnnotation(true)

    try {
      let scanId = bridgeScanId
      if (!scanId) {
        const bridgeResult = await createScanFromPage(documentId, currentPage)
        scanId = bridgeResult.scanId
        setBridgeScanId(scanId)
      }

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
      setAnnotationVersion(1)
      setIsDrawerOpen(true)
    } catch (err) {
      console.error('Failed to analyze text:', err)
      toast.error('Failed to analyze text. Please try again.')
    } finally {
      setIsLoadingAnnotation(false)
    }
  }

  const handleSpeechToggle = async () => {
    if (!selectedText) return

    if (speech.isPlaying || synthesizeSpeech.isPending) {
      speech.stop()
      return
    }

    try {
      const audioBlob = await synthesizeSpeech.mutateAsync({
        highlightedText: selectedText,
        contextText,
      })
      await speech.play(audioBlob)
    } catch (err) {
      console.error('Failed to play speech:', err)
      toast.error('Failed to play audio. Please try again.')
    }
  }

  const handleSaveAnnotation = async () => {
    if (!currentAnnotation || !currentAnnotation.scan_id) return

    try {
      await createAnnotation.mutateAsync({
        scanId: currentAnnotation.scan_id,
        highlightedText: currentAnnotation.highlighted_text,
        contextText: currentAnnotation.context_text,
        nuanceData: currentAnnotation.nuance_data,
      })
      setIsDrawerOpen(false)
      setCurrentAnnotation(null)
      setAnnotationVersion(1)
      clearSelection()
    } catch (err) {
      console.error('Failed to save annotation:', err)
      toast.error('Failed to save annotation. Please try again.')
    }
  }

  const handleRegenerateAnnotation = async () => {
    if (!currentAnnotation || analyzeText.isPending || annotationVersion >= MAX_ANNOTATION_VERSIONS) {
      return
    }

    try {
      const result = await analyzeText.mutateAsync({
        textToAnalyze: currentAnnotation.highlighted_text,
        context: currentAnnotation.context_text ?? '',
      })

      setCurrentAnnotation({
        ...currentAnnotation,
        nuance_data: result,
        created_at: new Date().toISOString(),
      })
      setAnnotationVersion((prev) => Math.min(prev + 1, MAX_ANNOTATION_VERSIONS))
    } catch (err) {
      console.error('Failed to regenerate annotation:', err)
      toast.error('Failed to regenerate annotation. Please try again.')
    }
  }

  const handleDrawerClose = () => {
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setAnnotationVersion(1)
    clearSelection()
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Document" />
        <div className="flex-1 flex items-center justify-center p-6">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
        </div>
      </div>
    )
  }

  if (error || !document) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Document" />
        <div className="flex-1 flex items-center justify-center p-6">
          <p className="text-gray-600">Document not found</p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header title={document.filename} />
      <ContinuousPDFViewer
        pdfUrl={pdfUrl}
        currentPage={currentPage}
        totalPages={document.pageCount}
        onPageChange={handlePageChange}
        onTextSelect={handleTextSelect}
        isLoading={isPdfLoading}
      />
      <BottomActionBar
        disabled={!selectedText || isLoadingAnnotation}
        isLoading={isLoadingAnnotation || analyzeText.isPending}
        onExplain={handleExplain}
        onSpeech={selectedText ? handleSpeechToggle : undefined}
        isPlaying={speech.isPlaying}
        isSpeechLoading={synthesizeSpeech.isPending}
      />
      <AnnotationDrawer
        isOpen={isDrawerOpen}
        onClose={handleDrawerClose}
        annotation={currentAnnotation}
        onRegenerate={handleRegenerateAnnotation}
        onSave={handleSaveAnnotation}
        isRegenerating={analyzeText.isPending}
        isSaving={createAnnotation.isPending}
        version={annotationVersion}
        maxVersions={MAX_ANNOTATION_VERSIONS}
      />
    </div>
  )
}
