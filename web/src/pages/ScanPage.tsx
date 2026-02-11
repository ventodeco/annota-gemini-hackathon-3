import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { useCallback, useEffect, useRef, useState } from 'react'
import { MoreVertical, Bookmark, Trash2 } from 'lucide-react'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useScan, isScanOcrReady, useDeleteScan } from '@/hooks/useScans'
import { useAnalyzeText, useCreateAnnotation, useSynthesizeSpeech } from '@/hooks/useAnnotations'
import { AnnotationDrawer } from '@/components/scanpage/AnnotationDrawer'
import type { Annotation } from '@/lib/types'
import { useTextSelection } from '@/hooks/useTextSelection'
import { getScanImageUrl, formatDate } from '@/lib/api'
import LoadingSpinner from '@/components/scanpage/LoadingSpinner'
import type { Scan } from '@/lib/types'
import { SelectionSpeechButton } from '@/components/scanpage/SelectionSpeechButton'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { toast } from 'sonner'

type ScanPageLocationState = {
  preloadedScan?: Scan
}

const MAX_ANNOTATION_VERSIONS = 2

type SelectionRect = {
  top: number
  left: number
  width: number
  height: number
}

export default function ScanPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const scanId = id ? parseInt(id, 10) : 0
  const scanHistoryPath = scanId > 0 ? `/history?scanId=${scanId}` : '/history'
  const preloadedScan = (location.state as ScanPageLocationState | null)?.preloadedScan
  const hasReadyPreloadedScan =
    preloadedScan?.id === scanId &&
    isScanOcrReady(preloadedScan)
  const { data: fetchedScan, isLoading, error } = useScan(scanId, {
    enabled: !hasReadyPreloadedScan,
    pollIntervalMs: 0,
  })
  const scan = fetchedScan ?? (preloadedScan?.id === scanId ? preloadedScan : undefined)
  const analyzeText = useAnalyzeText()
  const createAnnotation = useCreateAnnotation()
  const synthesizeSpeech = useSynthesizeSpeech()
  const deleteScan = useDeleteScan()
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [annotationVersion, setAnnotationVersion] = useState(1)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)
  const [isPlayingSpeech, setIsPlayingSpeech] = useState(false)
  const [contextText, setContextText] = useState('')
  const [selectionRect, setSelectionRect] = useState<SelectionRect | null>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const audioUrlRef = useRef<string | null>(null)
  const speechRequestIdRef = useRef(0)
  const { selectedText, handleSelection, clearSelection } = useTextSelection()

  const stopSpeechPlayback = useCallback(() => {
    speechRequestIdRef.current += 1
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.onended = null
      audioRef.current = null
    }
    if (audioUrlRef.current) {
      URL.revokeObjectURL(audioUrlRef.current)
      audioUrlRef.current = null
    }
    setIsPlayingSpeech(false)
  }, [])

  useEffect(() => {
    return () => stopSpeechPlayback()
  }, [stopSpeechPlayback])

  useEffect(() => {
    if (!selectedText) {
      setSelectionRect(null)
      stopSpeechPlayback()
    }
  }, [selectedText, stopSpeechPlayback])

  const handleTextSelect = () => {
    const selection = window.getSelection()
    if (!selection || selection.toString().trim() === '') {
      clearSelection()
      setContextText('')
      setSelectionRect(null)
      stopSpeechPlayback()
      return
    }
    handleSelection(selection.toString())

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
      setAnnotationVersion(1)
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
      setAnnotationVersion(1)
      clearSelection()
      setSelectionRect(null)
      stopSpeechPlayback()
    } catch (err) {
      console.error('Failed to save annotation:', err)
      alert('Failed to save annotation. Please try again.')
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
      alert('Failed to regenerate annotation. Please try again.')
    }
  }

  const handleDrawerClose = () => {
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setAnnotationVersion(1)
    clearSelection()
    setSelectionRect(null)
    stopSpeechPlayback()
  }

  const handleDeleteScan = async () => {
    if (!scanId || !scan) return
    try {
      await deleteScan.mutateAsync(scanId)
      setShowDeleteDialog(false)
      toast.success('Scan removed')
      navigate('/scans-history')
    } catch (err) {
      toast.error('Failed to remove scan')
    }
  }

  const scanHeaderRightSlot = scan ? (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-10 w-10 text-gray-900"
          aria-label="More options"
        >
          <MoreVertical className="w-6 h-6" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => navigate(scanHistoryPath)}>
          <Bookmark className="w-4 h-4 mr-2" />
          Annotations
        </DropdownMenuItem>
        <DropdownMenuItem
          variant="destructive"
          onClick={() => setShowDeleteDialog(true)}
        >
          <Trash2 className="w-4 h-4 mr-2" />
          Delete scan
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ) : undefined

  const handleSpeechToggle = async () => {
    if (!selectedText) {
      return
    }
    if (isPlayingSpeech || synthesizeSpeech.isPending) {
      stopSpeechPlayback()
      return
    }

    const requestId = speechRequestIdRef.current + 1
    speechRequestIdRef.current = requestId
    setIsPlayingSpeech(true)

    try {
      const audioBlob = await synthesizeSpeech.mutateAsync({
        highlightedText: selectedText,
        contextText,
      })

      if (speechRequestIdRef.current != requestId) {
        setIsPlayingSpeech(false)
        return
      }

      const audioUrl = URL.createObjectURL(audioBlob)
      const audio = new Audio(audioUrl)
      audioUrlRef.current = audioUrl
      audioRef.current = audio
      audio.onended = () => {
        if (audioRef.current) {
          audioRef.current = null
        }
        if (audioUrlRef.current) {
          URL.revokeObjectURL(audioUrlRef.current)
          audioUrlRef.current = null
        }
        setIsPlayingSpeech(false)
      }
      await audio.play()
    } catch (err) {
      stopSpeechPlayback()
      console.error('Failed to synthesize speech:', err)
      alert('Failed to synthesize speech. Please try again.')
    }
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header
          title="Scan Result"
          rightSlot={scanHeaderRightSlot}
          rightAction="bookmark"
          rightActionTo={scanHistoryPath}
        />
        <div className="flex-1 flex items-center justify-center p-6">
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
          rightSlot={scanHeaderRightSlot}
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
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header
          title="Scan Result"
          rightSlot={scanHeaderRightSlot}
          rightAction="bookmark"
          rightActionTo={scanHistoryPath}
        />
        <div className="flex-1 flex items-center justify-center p-6">
          <LoadingSpinner />
        </div>
      </div>
    )
  }

  const imageUrl = getScanImageUrl(scan.imageUrl)

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header title="Scan Result" rightSlot={scanHeaderRightSlot} />
      <ScrollArea className="flex-1">
        <div className="p-6">
          {imageUrl && (
            <img
              src={imageUrl}
              alt="Scanned document"
              className="w-full mb-6 rounded-lg"
            />
          )}
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
          selectionRect={selectionRect}
          isLoading={synthesizeSpeech.isPending}
          isPlaying={isPlayingSpeech}
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
        maxVersions={MAX_ANNOTATION_VERSIONS}
      />

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove scan?</AlertDialogTitle>
            <AlertDialogDescription>
              Remove this scan and its image? Annotations will remain in History.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteScan}
              disabled={deleteScan.isPending}
              className="bg-red-600 hover:bg-red-700"
            >
              {deleteScan.isPending ? 'Removing...' : 'Remove'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
