import { useQuery } from '@tanstack/react-query'
import { getAnnotation } from '@/lib/api'

export function useAnnotationById(annotationId: number | undefined) {
  return useQuery({
    queryKey: ['annotation', annotationId],
    queryFn: () => {
      if (!annotationId) throw new Error('Annotation ID is required')
      return getAnnotation(annotationId)
    },
    enabled: !!annotationId,
  })
}
