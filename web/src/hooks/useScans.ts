import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { createScan, deleteScan, getScans } from '@/lib/api'
import type { Scan } from '@/lib/types'
export { useScan, isScanOcrReady } from './useScan'

export function useScans(page = 1, size = 20) {
  return useQuery({
    queryKey: ['scans', page, size],
    queryFn: () => getScans(page, size),
  })
}

export function useCreateScan() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (image: File) => createScan(image),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scans'] })
    },
  })
}

export function useDeleteScan() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (scanId: number) => deleteScan(scanId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scans'] })
    },
  })
}

export function useScanImageUrl(scan: Scan | undefined): string {
  if (!scan || !scan.imageUrl) return ''
  if (scan.imageUrl.startsWith('/')) {
    return `${import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'}${scan.imageUrl}`
  }
  return scan.imageUrl
}
