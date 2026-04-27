import { useQuery } from '@tanstack/react-query'
import { getScan } from '@/lib/api'
import type { Scan } from '@/lib/types'

type UseScanOptions = {
  enabled?: boolean
  pollIntervalMs?: number
  retry?: boolean | number
}

export function isScanOcrReady(scan: Pick<Scan, 'fullText' | 'status'> | null | undefined): boolean {
  return scan?.status === 'ready' || Boolean(scan?.fullText && scan.fullText.trim().length > 0)
}

export function isScanFailed(scan: Pick<Scan, 'status'> | null | undefined): boolean {
  return scan?.status === 'failed'
}

export function useScan(scanId: number | undefined, options: UseScanOptions = {}) {
  const { enabled = true, pollIntervalMs = 2000, retry } = options

  return useQuery<Scan, Error>({
    queryKey: ['scan', scanId],
    queryFn: () => {
      if (!scanId) throw new Error('Scan ID is required')
      return getScan(scanId)
    },
    enabled: enabled && !!scanId,
    ...(retry !== undefined ? { retry } : {}),
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data || isScanOcrReady(data) || isScanFailed(data) || pollIntervalMs <= 0) {
        return false
      }
      return pollIntervalMs
    },
  })
}
