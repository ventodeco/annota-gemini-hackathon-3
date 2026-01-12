import { useMutation } from '@tanstack/react-query'
import { annotate, type AnnotateRequest } from '@/lib/api'

export function useAnnotation(scanID: string) {
  return useMutation({
    mutationFn: (request: AnnotateRequest) => annotate(scanID, request),
  })
}
