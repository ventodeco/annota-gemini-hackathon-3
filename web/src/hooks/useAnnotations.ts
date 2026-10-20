import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createAnnotation,
  deleteAnnotation,
  getAnnotations,
  analyzeText,
  synthesizeSpeech,
} from '@/lib/api'
import type {
  CreateAnnotationRequest,
  AnalyzeRequest,
  NuanceData,
  SynthesizeSpeechRequest,
} from '@/lib/types'
import { useInvalidateEntitlements } from './useEntitlements'

// Replays of the same selection reuse audio instead of spending speech
// allowance again. The query cache is cleared on sign-out and account switch.
const SPEECH_CACHE_MS = 30 * 60_000

export function useAnnotations(page = 1, size = 20, scanId?: number, documentId?: number, pageNumber?: number) {
  return useQuery({
    queryKey: ['annotations', page, size, scanId, documentId, pageNumber],
    queryFn: () => getAnnotations(page, size, scanId, documentId, pageNumber),
  })
}

export function useCreateAnnotation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateAnnotationRequest) => createAnnotation(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['annotations'] })
    },
  })
}

export function useDeleteAnnotation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (annotationId: number) => deleteAnnotation(annotationId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['annotations'] })
    },
  })
}

export function useAnalyzeText() {
  const invalidateEntitlements = useInvalidateEntitlements()

  return useMutation({
    mutationFn: (data: AnalyzeRequest) => analyzeText(data),
    onSettled: invalidateEntitlements,
  })
}

export function useSynthesizeSpeech() {
  const queryClient = useQueryClient()
  const invalidateEntitlements = useInvalidateEntitlements()

  return useMutation({
    mutationFn: (data: SynthesizeSpeechRequest) =>
      queryClient.fetchQuery({
        queryKey: ['speech', data.highlightedText, data.contextText ?? ''],
        queryFn: () => synthesizeSpeech(data),
        staleTime: Infinity,
        gcTime: SPEECH_CACHE_MS,
        retry: false,
      }),
    onSettled: invalidateEntitlements,
  })
}

export function useNuanceSummary(nuance: NuanceData | undefined): string {
  if (!nuance) return ''
  const summary = nuance.translation || nuance.meaning || nuance.contextualExplanation || ''
  if (summary.length > 100) {
    return summary.substring(0, 100) + '...'
  }
  return summary
}
