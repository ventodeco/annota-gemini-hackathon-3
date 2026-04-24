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
  return useMutation({
    mutationFn: (data: AnalyzeRequest) => analyzeText(data),
  })
}

export function useSynthesizeSpeech() {
  return useMutation({
    mutationFn: (data: SynthesizeSpeechRequest) => synthesizeSpeech(data),
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
