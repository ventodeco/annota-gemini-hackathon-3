import { useQuery } from '@tanstack/react-query'
import { getScan } from '@/lib/api'

export function useScan(scanID: string | undefined, enabled: boolean = true) {
  return useQuery({
    queryKey: ['scan', scanID],
    queryFn: () => {
      if (!scanID) throw new Error('Scan ID is required')
      return getScan(scanID)
    },
    enabled: enabled && !!scanID,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      
      const status = data.status
      if (status === 'ocr_done' || status === 'failed' || 
          status === 'failed_overloaded' || status === 'failed_auth') {
        return false
      }
      
      return 2000
    },
  })
}
