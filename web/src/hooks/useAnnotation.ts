import { useMutation } from '@tanstack/react-query'
import { analyzeText, createAnnotation } from '@/lib/api'
import type { NuanceData } from '@/lib/types'

interface AnnotateRequest {
  textToAnalyze: string
  context: string
}

export function useAnnotation(scanId: number) {
  return useMutation({
    mutationFn: async ({ textToAnalyze, context }: AnnotateRequest) => {
      const nuanceData: NuanceData = await analyzeText({
        textToAnalyze,
        context,
      })
      return createAnnotation({
        scanId,
        highlightedText: textToAnalyze,
        contextText: context,
        nuanceData,
      })
    },
  })
}
