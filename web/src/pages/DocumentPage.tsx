import { useCallback, useEffect, useMemo, useRef, useState, type ReactElement } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import ContinuousPDFViewer from '@/components/documentpage/ContinuousPDFViewer'
import BottomActionBar from '@/components/layout/BottomActionBar'
import Header from '@/components/layout/Header'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import { DEFAULT_MAX_ANNOTATION_VERSIONS, useAnnotationDrawerFlow } from '@/hooks/useAnnotationDrawerFlow'
import { useAnalyzeText, useCreateAnnotation, useSynthesizeSpeech } from '@/hooks/useAnnotations'
import { useDocument, useUpdateDocumentProgress } from '@/hooks/useDocument'
import { useSpeechPlayback } from '@/hooks/useSpeechPlayback'
import { useTextSelection } from '@/hooks/useTextSelection'
import { createScanFromPage, getDocumentFile } from '@/lib/api'
import type { Document } from '@/lib/types'

type DocumentPdfSectionProps = {
  documentId: number
  document: Document
  currentPage: number
  onPageChange: (page: number, change: PageChange) => void
  onTextSelect: (selectedText: string) => void
}

type PageChangeSource = 'scroll' | 'navigation'
type PageChange = { source: PageChangeSource }

const DOCUMENT_PROGRESS_SAVE_DELAY_MS = 500

function DocumentPdfSection({
  documentId,
  document,
  currentPage,
  onPageChange,
  onTextSelect,
}: DocumentPdfSectionProps): ReactElement {
  const [pdfUrl, setPdfUrl] = useState<string>('')
  const [loadError, setLoadError] = useState<boolean>(false)
  const objectUrlRef = useRef('')

  const isPdfLoading = !pdfUrl && !loadError

  useEffect(() => {
    let cancelled = false
    getDocumentFile(documentId)
      .then((blob) => {
        if (cancelled) return
        const url = URL.createObjectURL(blob)
        objectUrlRef.current = url
        setPdfUrl(url)
      })
      .catch((err) => {
        if (cancelled) return
        console.error('Failed to load PDF:', err)
        toast.error('Failed to load PDF. Please try again.')
        setLoadError(true)
      })

    return () => {
      cancelled = true
      if (objectUrlRef.current) {
        URL.revokeObjectURL(objectUrlRef.current)
        objectUrlRef.current = ''
      }
    }
  }, [documentId])

  if (loadError) {
    return (
      <div className="flex-1 flex items-center justify-center p-6">
        <p className="text-gray-600">Failed to load PDF</p>
      </div>
    )
  }

  return (
    <ContinuousPDFViewer
      pdfUrl={pdfUrl}
      currentPage={currentPage}
      totalPages={document.pageCount}
      onPageChange={onPageChange}
      onTextSelect={onTextSelect}
      isLoading={isPdfLoading}
    />
  )
}

export default function DocumentPage(): ReactElement {
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const documentId = id ? Number.parseInt(id, 10) : undefined

  const { data: document, isLoading, error } = useDocument(documentId)
  const [pageOverride, setPageOverride] = useState<{ documentId: number; page: number } | null>(null)
  const [bridgeScanId, setBridgeScanId] = useState<number | null>(null)
  const updateProgress = useUpdateDocumentProgress(documentId)
  const progressSaveTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(null)
  const lastRequestedProgressRef = useRef<number | null>(null)
  const lastSavedProgressRef = useRef<number | null>(null)

  const { selectedText, handleSelection, clearSelection } = useTextSelection()
  const analyzeText = useAnalyzeText()
  const createAnnotation = useCreateAnnotation()
  const synthesizeSpeech = useSynthesizeSpeech()
  const speech = useSpeechPlayback()

  const [contextText, setContextText] = useState<string>('')

  const resetTextSelectionState = useCallback(() => {
    clearSelection()
    setContextText('')
  }, [clearSelection])

  const initialPage = useMemo(() => {
    if (!document) return 1
    const requestedPage = Number.parseInt(searchParams.get('page') || '', 10)
    return Number.isInteger(requestedPage) && requestedPage >= 1 && requestedPage <= document.pageCount
      ? requestedPage
      : document.lastPageNumber || 1
  }, [document, searchParams])

  const currentPage =
    pageOverride && pageOverride.documentId === documentId ? pageOverride.page : initialPage

  useEffect(() => {
    lastRequestedProgressRef.current = document?.lastPageNumber ?? null
    lastSavedProgressRef.current = document?.lastPageNumber ?? null
  }, [document?.id, document?.lastPageNumber])

  useEffect(() => {
    return () => {
      if (progressSaveTimerRef.current !== null) {
        window.clearTimeout(progressSaveTimerRef.current)
      }
    }
  }, [])

  const resolveScanIdForExplain = useCallback(async (): Promise<number> => {
    if (!documentId) {
      throw new Error('Missing document')
    }
    if (bridgeScanId !== null) {
      return bridgeScanId
    }
    const bridgeResult = await createScanFromPage(documentId, currentPage)
    setBridgeScanId(bridgeResult.scanId)
    return bridgeResult.scanId
  }, [documentId, currentPage, bridgeScanId])

  const {
    isDrawerOpen,
    currentAnnotation,
    annotationVersion,
    isLoadingAnnotation,
    resetAnnotationState,
    handleExplain,
    handleSaveAnnotation,
    handleRegenerateAnnotation,
    handleDrawerClose,
    handleSpeechToggle,
  } = useAnnotationDrawerFlow({
    selectedText,
    contextText,
    analyzeText,
    createAnnotation,
    synthesizeSpeech,
    speech,
    clearSelection,
    reportError: toast.error,
    resolveScanIdForExplain,
  })

  useEffect(() => {
    return () => {
      speech.stop()
    }
  }, [speech])

  useEffect(() => {
    if (!selectedText) {
      speech.stop()
    }
  }, [selectedText, speech])

  const scheduleProgressSave = useCallback((page: number): void => {
    if (!document || page < 1 || page > document.pageCount) {
      return
    }
    if (lastRequestedProgressRef.current === page) {
      return
    }

    lastRequestedProgressRef.current = page
    if (progressSaveTimerRef.current !== null) {
      window.clearTimeout(progressSaveTimerRef.current)
      progressSaveTimerRef.current = null
    }

    if (lastSavedProgressRef.current === page) {
      return
    }

    progressSaveTimerRef.current = window.setTimeout(() => {
      progressSaveTimerRef.current = null
      updateProgress.mutate(page, {
        onSuccess: () => {
          lastSavedProgressRef.current = page
        },
      })
    }, DOCUMENT_PROGRESS_SAVE_DELAY_MS)
  }, [document, updateProgress])

  const handlePageChange = useCallback((page: number, change: PageChange): void => {
    const pageChanged = page !== currentPage
    if (documentId && pageChanged) {
      setPageOverride({ documentId, page })
    }

    if (change.source === 'navigation') {
      clearSelection()
      resetAnnotationState()
      setBridgeScanId(null)
    }

    if (pageChanged) {
      scheduleProgressSave(page)
    }
  }, [clearSelection, currentPage, documentId, resetAnnotationState, scheduleProgressSave])

  const handleTextSelect = useCallback((selectedTextFromViewer: string): void => {
    const selectedTextValue = selectedTextFromViewer.trim()
    if (selectedTextValue === '') {
      resetTextSelectionState()
      return
    }

    const selectionResult = handleSelection(selectedTextValue)
    if (!selectionResult.valid) {
      resetTextSelectionState()
      return
    }

    const selection = window.getSelection()
    if (!selection || selection.rangeCount === 0) {
      setContextText(selectedTextValue)
      return
    }

    const range = selection.getRangeAt(0)
    const context = range.cloneContents().textContent?.trim() ?? ''
    setContextText(context || selectedTextValue)
  }, [handleSelection, resetTextSelectionState])

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

  if (error || !document || documentId === undefined) {
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
      <DocumentPdfSection
        key={documentId}
        documentId={documentId}
        document={document}
        currentPage={currentPage}
        onPageChange={handlePageChange}
        onTextSelect={handleTextSelect}
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
        maxVersions={DEFAULT_MAX_ANNOTATION_VERSIONS}
      />
    </div>
  )
}
