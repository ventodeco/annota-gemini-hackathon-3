import { useQuery } from '@tanstack/react-query'
import { getScan } from '@/lib/api'
import type { Scan } from '@/lib/types'

type UseScanOptions = {
  enabled?: boolean
  pollIntervalMs?: number
}

export function isScanOcrReady(scan: Pick<Scan, 'fullText'> | null | undefined): boolean {
  return Boolean(scan?.fullText && scan.fullText.trim().length > 0)
}

export function useScan(scanId: number | undefined, options: UseScanOptions = {}) {
  const { enabled = true, pollIntervalMs = 2000 } = options

  return useQuery({
    queryKey: ['scan', scanId],
    queryFn: () => {
      if (!scanId) throw new Error('Scan ID is required')
      return getScan(scanId)
    },
    enabled: enabled && !!scanId,
    refetchInterval: (query) => {
      const data = query.state.data as Scan | undefined
      if (!data) return false
      if (isScanOcrReady(data)) return false
      if (pollIntervalMs <= 0) return false
      return pollIntervalMs
    },
  })
}
