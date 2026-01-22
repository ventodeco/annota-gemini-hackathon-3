import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { createAnnotation, getAnnotations, analyzeText } from '@/lib/api'
import type { CreateAnnotationRequest, AnalyzeRequest, NuanceData } from '@/lib/types'

export function useAnnotations(page = 1, size = 20) {
  return useQuery({
    queryKey: ['annotations', page, size],
    queryFn: () => getAnnotations(page, size),
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

export function useAnalyzeText() {
  return useMutation({
    mutationFn: (data: AnalyzeRequest) => analyzeText(data),
  })
}

export function useNuanceSummary(nuance: NuanceData | undefined): string {
  if (!nuance) return ''
  if (nuance.meaning.length > 100) {
    return nuance.meaning.substring(0, 100) + '...'
  }
  return nuance.meaning
}
