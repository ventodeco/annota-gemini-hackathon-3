import { useQuery, useQueryClient, type Query } from '@tanstack/react-query'
import { getEntitlements } from '@/lib/api'
import type { Entitlements } from '@/lib/entitlements'

export const ENTITLEMENTS_QUERY_KEY = ['entitlements'] as const

const MIN_REFRESH_MS = 30_000

// Refresh just after the daily reset so the recovered allowance appears
// without a reload. The server stays authoritative for every admission.
function refreshAtReset(query: Query<Entitlements, Error, Entitlements, typeof ENTITLEMENTS_QUERY_KEY>): number | false {
  const resetAt = query.state.data?.usage.resetAt
  if (!resetAt) return false
  return Math.max(Date.parse(resetAt) - Date.now() + 1_000, MIN_REFRESH_MS)
}

export function useEntitlements() {
  return useQuery({
    queryKey: ENTITLEMENTS_QUERY_KEY,
    queryFn: getEntitlements,
    staleTime: MIN_REFRESH_MS,
    refetchInterval: refreshAtReset,
    // Intervals pause in hidden tabs and the app disables focus refetching
    // globally; refresh on return so a reset that happened meanwhile shows.
    refetchOnWindowFocus: true,
  })
}

/** Returns a callback that marks the allowance stale after metered work. */
export function useInvalidateEntitlements(): () => void {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: ENTITLEMENTS_QUERY_KEY })
  }
}
