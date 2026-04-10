import { useState, useCallback } from 'react'
import type { UseMutationResult } from '@tanstack/react-query'
import type {
  Annotation,
  AnalyzeRequest,
  AnalyzeResponse,
  CreateAnnotationRequest,
  CreateAnnotationResponse,
  SynthesizeSpeechRequest,
} from '@/lib/types'

export const DEFAULT_MAX_ANNOTATION_VERSIONS = 2

export function useAnnotationDrawerFlow(options: {
  selectedText: string
  contextText: string
  analyzeText: UseMutationResult<AnalyzeResponse, Error, AnalyzeRequest>
  createAnnotation: UseMutationResult<CreateAnnotationResponse, Error, CreateAnnotationRequest>
  synthesizeSpeech: UseMutationResult<Blob, Error, SynthesizeSpeechRequest>
  speech: {
    isPlaying: boolean
    stop: () => void
    play: (blob: Blob) => Promise<void>
  }
  clearSelection: () => void
  reportError: (message: string) => void
  resolveScanIdForExplain: () => Promise<number>
  maxVersions?: number
  onAfterSave?: () => void
  onDrawerCloseExtra?: () => void
  analyzeErrorMessage?: string
  saveErrorMessage?: string
  regenerateErrorMessage?: string
  speechErrorMessage?: string
}) {
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

  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [currentAnnotation, setCurrentAnnotation] = useState<Annotation | null>(null)
  const [annotationVersion, setAnnotationVersion] = useState(1)
  const [isLoadingAnnotation, setIsLoadingAnnotation] = useState(false)

  const handleExplain = useCallback(async () => {
    if (!selectedText) return

    setIsLoadingAnnotation(true)
    try {
      const scanId = await resolveScanIdForExplain()
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
      reportError(analyzeErrorMessage)
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

  const resetAnnotationState = useCallback(() => {
    setIsDrawerOpen(false)
    setCurrentAnnotation(null)
    setAnnotationVersion(1)
  }, [])

  const handleSaveAnnotation = useCallback(async () => {
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
      reportError(saveErrorMessage)
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

  const handleRegenerateAnnotation = useCallback(async () => {
    if (!currentAnnotation || analyzeText.isPending || annotationVersion >= maxVersions) {
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
      setAnnotationVersion((prev) => Math.min(prev + 1, maxVersions))
    } catch (err) {
      console.error('Failed to regenerate annotation:', err)
      reportError(regenerateErrorMessage)
    }
  }, [
    currentAnnotation,
    analyzeText,
    annotationVersion,
    maxVersions,
    reportError,
    regenerateErrorMessage,
  ])

  const handleDrawerClose = useCallback(() => {
    resetAnnotationState()
    clearSelection()
    onDrawerCloseExtra?.()
  }, [resetAnnotationState, clearSelection, onDrawerCloseExtra])

  const handleSpeechToggle = useCallback(async () => {
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
      reportError(speechErrorMessage)
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
