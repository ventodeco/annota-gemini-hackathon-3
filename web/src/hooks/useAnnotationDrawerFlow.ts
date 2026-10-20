import { useState, useCallback, useRef } from 'react'
import type { UseMutationResult } from '@tanstack/react-query'
import type {
  Annotation,
  AnalyzeRequest,
  AnalyzeResponse,
  CreateAnnotationRequest,
  CreateAnnotationResponse,
  SynthesizeSpeechRequest,
} from '@/lib/types'
import { getUsageErrorMessage } from '@/lib/usage'

export const DEFAULT_MAX_ANNOTATION_VERSIONS = 2

type SpeechPlaybackControls = {
  isPlaying: boolean
  stop: () => void
  play: (blob: Blob) => Promise<void>
}

type UseAnnotationDrawerFlowOptions = {
  selectedText: string
  contextText: string
  analyzeText: UseMutationResult<AnalyzeResponse, Error, AnalyzeRequest>
  createAnnotation: UseMutationResult<CreateAnnotationResponse, Error, CreateAnnotationRequest>
  synthesizeSpeech: UseMutationResult<Blob, Error, SynthesizeSpeechRequest>
  speech: SpeechPlaybackControls
  clearSelection: () => void
  /** Receives the caught error too, so callers can show its support reference. */
  reportError: (message: string, error?: unknown) => void
  resolveScanIdForExplain: () => Promise<number>
  maxVersions?: number
  onAfterSave?: () => void
  onDrawerCloseExtra?: () => void
  analyzeErrorMessage?: string
  saveErrorMessage?: string
  regenerateErrorMessage?: string
  speechErrorMessage?: string
}

type UseAnnotationDrawerFlowResult = {
  isDrawerOpen: boolean
  currentAnnotation: Annotation | null
  annotationVersion: number
  isLoadingAnnotation: boolean
  resetAnnotationState: () => void
  handleExplain: () => Promise<void>
  handleSaveAnnotation: () => Promise<void>
  handleRegenerateAnnotation: () => Promise<void>
  handleDrawerClose: () => void
  handleSpeechToggle: () => Promise<void>
}

export function useAnnotationDrawerFlow(
  options: UseAnnotationDrawerFlowOptions,
): UseAnnotationDrawerFlowResult {
  const {
    selectedText,
    contextText,
    analyzeText,
    createAnnotation,
    synthesizeSpeech,
    speech,
    clearSelection,
    reportError,
    resolveScanIdForExplain,
    maxVersions = DEFAULT_MAX_ANNOTATION_VERSIONS,
    onAfterSave,
    onDrawerCloseExtra,
    analyzeErrorMessage = 'Failed to analyze text. Please try again.',
    saveErrorMessage = 'Failed to save annotation. Please try again.',
    regenerateErrorMessage = 'Failed to regenerate annotation. Please try again.',
    speechErrorMessage = 'Failed to play audio. Please try again.',
  } = options

  const [isDrawerOpen, setIsDrawerOpen] = useState<boolean>(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [annotationVersion, setAnnotationVersion] = useState<number>(1)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState<boolean>(false)
  // Bumped on every reset (page change, view change, close) so a request
  // that finishes afterwards can't show an explanation for the old selection.
  const requestGenerationRef = useRef(0)

  const handleExplain = useCallback(async (): Promise<void> => {
    if (!selectedText) return

    const generation = requestGenerationRef.current
    const isStale = (): boolean => generation !== requestGenerationRef.current
    setIsLoadingAnnotation(true)
    try {
      const scanId = await resolveScanIdForExplain()
      if (isStale()) return
      const result = await analyzeText.mutateAsync({
        textToAnalyze: selectedText,
        context: contextText,
        scanId,
      })
      if (isStale()) return

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
      if (isStale()) return
      console.error('Failed to analyze text:', err)
      reportError(getUsageErrorMessage(err) ?? analyzeErrorMessage, err)
    } finally {
      setIsLoadingAnnotation(false)
    }
  }, [
    selectedText,
    contextText,
    analyzeText,
    resolveScanIdForExplain,
    reportError,
    analyzeErrorMessage,
  ])

  const resetAnnotationState = useCallback((): void => {
    requestGenerationRef.current += 1
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setAnnotationVersion(1)
  }, [])

  const handleSaveAnnotation = useCallback(async (): Promise<void> => {
    if (!currentAnnotation?.scan_id) return

    try {
      await createAnnotation.mutateAsync({
        scanId: currentAnnotation.scan_id,
        highlightedText: currentAnnotation.highlighted_text,
        contextText: currentAnnotation.context_text,
        nuanceData: currentAnnotation.nuance_data,
      })
      resetAnnotationState()
      clearSelection()
      onAfterSave?.()
    } catch (err) {
      console.error('Failed to save annotation:', err)
      reportError(saveErrorMessage, err)
    }
  }, [
    currentAnnotation,
    createAnnotation,
    clearSelection,
    onAfterSave,
    reportError,
    saveErrorMessage,
    resetAnnotationState,
  ])

  const handleRegenerateAnnotation = useCallback(async (): Promise<void> => {
    if (!currentAnnotation || analyzeText.isPending || annotationVersion >= maxVersions) {
      return
    }

    const generation = requestGenerationRef.current
    try {
      // Same scan as the first explanation, so the server uses the same page context.
      const result = await analyzeText.mutateAsync({
        textToAnalyze: currentAnnotation.highlighted_text,
        context: currentAnnotation.context_text ?? '',
        ...(currentAnnotation.scan_id ? { scanId: currentAnnotation.scan_id } : {}),
      })
      if (generation !== requestGenerationRef.current) return

      setCurrentAnnotation({
        ...currentAnnotation,
        nuance_data: result,
        created_at: new Date().toISOString(),
      })
      setAnnotationVersion((prev) => Math.min(prev + 1, maxVersions))
    } catch (err) {
      if (generation !== requestGenerationRef.current) return
      console.error('Failed to regenerate annotation:', err)
      reportError(getUsageErrorMessage(err) ?? regenerateErrorMessage, err)
    }
  }, [
    currentAnnotation,
    analyzeText,
    annotationVersion,
    maxVersions,
    reportError,
    regenerateErrorMessage,
  ])

  const handleDrawerClose = useCallback((): void => {
    resetAnnotationState()
    clearSelection()
    onDrawerCloseExtra?.()
  }, [resetAnnotationState, clearSelection, onDrawerCloseExtra])

  const handleSpeechToggle = useCallback(async (): Promise<void> => {
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
      speech.stop()
      console.error('Failed to synthesize speech:', err)
      reportError(getUsageErrorMessage(err) ?? speechErrorMessage, err)
    }
  }, [selectedText, contextText, synthesizeSpeech, speech, reportError, speechErrorMessage])

  return {
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
  }
}
