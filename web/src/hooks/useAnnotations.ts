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

export function useAnnotations(page = 1, size = 20, scanId?: number) {
  return useQuery({
    queryKey: ['annotations', page, size, scanId],
    queryFn: () => getAnnotations(page, size, scanId),
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
  if (nuance.meaning.length > 100) {
    return nuance.meaning.substring(0, 100) + '...'
  }
  return nuance.meaning
}
