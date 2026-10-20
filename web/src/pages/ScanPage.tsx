import { useCallback, useEffect, useState, type ReactElement } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useScan, isScanOcrReady } from '@/hooks/useScans'
import { useAnalyzeText, useCreateAnnotation, useSynthesizeSpeech } from '@/hooks/useAnnotations'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import { useTextSelection } from '@/hooks/useTextSelection'
import { useSpeechPlayback } from '@/hooks/useSpeechPlayback'
import { formatDate } from '@/lib/api'
import LoadingSpinner from '@/components/scanpage/LoadingSpinner'
import ScanFailureState from '@/components/scanpage/ScanFailureState'
import { getScanFailureCopy, NO_TEXT_SCAN_COPY } from '@/lib/scanStatus'
import ScanImage from '@/components/scanpage/ScanImage'
import type { Scan } from '@/lib/types'
import { SelectionSpeechButton } from '@/components/scanpage/SelectionSpeechButton'
import { useAnnotationDrawerFlow, DEFAULT_MAX_ANNOTATION_VERSIONS } from '@/hooks/useAnnotationDrawerFlow'

// A direct link can open a scan that is still processing; poll until it settles.
const SCAN_POLL_INTERVAL_MS = 3000

type SelectionRect = {
  top: number
  left: number
  width: number
  height: number
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isScanStatus(value: unknown): value is Scan['status'] {
  return value === 'processing' || value === 'ready' || value === 'failed'
}

function isScanSourceType(value: unknown): value is Scan['sourceType'] {
  return value === 'image' || value === 'pdf'
}

function isScan(value: unknown): value is Scan {
  if (!isRecord(value)) return false
  return (
    typeof value.id === 'number' &&
    typeof value.imageUrl === 'string' &&
    isScanSourceType(value.sourceType) &&
    isScanStatus(value.status) &&
    typeof value.createdAt === 'string' &&
    (value.fullText === undefined || typeof value.fullText === 'string') &&
    (value.detectedLanguage === undefined || typeof value.detectedLanguage === 'string') &&
    (value.failureReason === undefined || typeof value.failureReason === 'string') &&
    (value.documentId === undefined || typeof value.documentId === 'number') &&
    (value.pageNumber === undefined || typeof value.pageNumber === 'number')
  )
}

function getPreloadedScan(state: unknown): Scan | undefined {
  if (!isRecord(state) || !isScan(state.preloadedScan)) {
    return undefined
  }
  return state.preloadedScan
}

export default function ScanPage(): ReactElement {
  const location = useLocation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const scanId = id ? Number.parseInt(id, 10) : 0
  const scanHistoryPath = scanId > 0 ? `/history?scanId=${scanId}` : '/history'
  const preloadedScan = getPreloadedScan(location.state)
  const hasReadyPreloadedScan =
    preloadedScan?.id === scanId &&
    isScanOcrReady(preloadedScan)
  const { data: fetchedScan, isLoading, error } = useScan(scanId, {
    enabled: !hasReadyPreloadedScan,
    pollIntervalMs: SCAN_POLL_INTERVAL_MS,
  })
  const scan = fetchedScan ?? (preloadedScan?.id === scanId ? preloadedScan : undefined)
  const analyzeText = useAnalyzeText()
  const createAnnotation = useCreateAnnotation()
  const synthesizeSpeech = useSynthesizeSpeech()
  const [contextText, setContextText] = useState<string>('')
  const [selectionRect, setSelectionRect] = useState<SelectionRect | null>(null)
  const { selectedText, handleSelection, clearSelection } = useTextSelection()
  const speech = useSpeechPlayback()

  const resolveScanIdForExplain = useCallback(async () => {
    if (!scanId) {
      throw new Error('Missing scan')
    }
    return scanId
  }, [scanId])

  const {
    isDrawerOpen,
    currentAnnotation,
    annotationVersion,
    isLoadingAnnotation,
    handleExplain,
    handleSaveAnnotation: saveAnnotation,
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
    onAfterSave: () => {
      setSelectionRect(null)
      speech.stop()
    },
    onDrawerCloseExtra: () => {
      setSelectionRect(null)
      speech.stop()
    },
  })

  useEffect(() => {
    if (!selectedText) {
      speech.stop()
    }
  }, [selectedText, speech])

  const effectiveSelectionRect = selectedText ? selectionRect : null

  const clearSelectedText = useCallback((): void => {
    clearSelection()
    setContextText('')
    setSelectionRect(null)
    speech.stop()
  }, [clearSelection, speech])

  const handleTextSelect = useCallback((): void => {
    const selection = window.getSelection()
    const selectedTextValue = selection?.toString() ?? ''
    if (!selection || selectedTextValue.trim() === '') {
      clearSelectedText()
      return
    }
    handleSelection(selectedTextValue)

    const range = selection.getRangeAt(0)
    const context = range.endContainer.textContent || ''
    setContextText(context)
    const rect = range.getBoundingClientRect()
    setSelectionRect({
      top: rect.top,
      left: rect.left,
      width: rect.width,
      height: rect.height,
    })
  }, [clearSelectedText, handleSelection])

  const handleSaveAnnotation = async (): Promise<void> => {
    if (!scan) return
    await saveAnnotation()
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header
          title="Scan Result"
          rightAction="bookmark"
          rightActionTo={scanHistoryPath}
        />
        <div className="flex-1 flex items-center justify-center p-6" role="status" aria-label="Loading scan">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
        </div>
      </div>
    )
  }

  if (error || !scan) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header
          title="Scan Result"
          rightAction="bookmark"
          rightActionTo={scanHistoryPath}
        />
        <div className="flex-1 flex items-center justify-center p-6">
          <p className="text-gray-600">Scan not found</p>
        </div>
      </div>
    )
  }

  if (!scan.fullText) {
    const header = (
      <Header
        title="Scan Result"
        rightAction="bookmark"
        rightActionTo={scanHistoryPath}
      />
    )

    if (scan.status === 'processing') {
      return (
        <div className="min-h-screen bg-white flex flex-col">
          {header}
          <div className="flex-1 flex items-center justify-center p-6">
            <LoadingSpinner message="Reading the text on your page..." />
          </div>
        </div>
      )
    }

    // A failed scan, or a finished scan whose page had no readable text.
    const copy = scan.status === 'failed' ? getScanFailureCopy(scan.failureReason) : NO_TEXT_SCAN_COPY
    return (
      <div className="min-h-screen bg-white flex flex-col">
        {header}
        <ScanFailureState
          title={copy.title}
          description={copy.description}
          actionLabel="Upload another page"
          onAction={() => navigate('/welcome')}
        />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header
        title="Scan Result"
        rightAction="bookmark"
        rightActionTo={scanHistoryPath}
      />
      <ScrollArea className="flex-1">
        <div className="p-6 max-w-md mx-auto">
          <ScanImage imageUrl={scan.imageUrl} alt="Scanned document" />
          {scan.fullText && (
            <p
              className="text-base leading-relaxed text-gray-900 whitespace-pre-wrap"
              onMouseUp={handleTextSelect}
              onTouchEnd={handleTextSelect}
            >
              {scan.fullText}
            </p>
          )}
          <div className="mt-4 text-sm text-gray-500">
            <p>Detected language: {scan.detectedLanguage || 'Unknown'}</p>
            <p>Created: {formatDate(scan.createdAt)}</p>
          </div>
        </div>
      </ScrollArea>
      <BottomActionBar
        disabled={!selectedText || isLoadingAnnotation}
        isLoading={isLoadingAnnotation || analyzeText.isPending}
        onExplain={handleExplain}
      />
      {selectedText && (
        <SelectionSpeechButton
          selectionRect={effectiveSelectionRect}
          isLoading={synthesizeSpeech.isPending}
          isPlaying={speech.isPlaying}
          onClick={handleSpeechToggle}
        />
      )}
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
