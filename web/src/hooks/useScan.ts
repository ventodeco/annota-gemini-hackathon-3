import { useQuery } from '@tanstack/react-query'
import { getScan } from '@/lib/api'

export function useScan(scanId: number | undefined, enabled: boolean = true) {
  return useQuery({
    queryKey: ['scan', scanId],
    queryFn: () => {
      if (!scanId) throw new Error('Scan ID is required')
      return getScan(scanId)
    },
    enabled: enabled && !!scanId,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false

      const hasFullText = !!data.full_ocr_text
      if (hasFullText) return false

      return 2000
    },
  })
}
