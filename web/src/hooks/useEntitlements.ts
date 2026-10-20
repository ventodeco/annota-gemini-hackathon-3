import { useQuery, useQueryClient, type Query } from '@tanstack/react-query'
import { getEntitlements } from '@/lib/api'
import type { Entitlements } from '@/lib/entitlements'

export const ENTITLEMENTS_QUERY_KEY = ['entitlements'] as const

const STALE_MS = 30_000
// Background OCR settles after the upload response, possibly refunding a
// page; a bounded foreground refresh surfaces that without a reload.
const FOREGROUND_REFRESH_MS = 60_000
const MIN_REFRESH_MS = 5_000

// Refresh at least every minute while visible, and just after the daily
// reset so a recovered allowance appears. The server stays authoritative for
// every admission.
function nextRefresh(query: Query<Entitlements, Error, Entitlements, typeof ENTITLEMENTS_QUERY_KEY>): number | false {
  const resetAt = query.state.data?.usage.resetAt
  if (!resetAt) return false
  const untilReset = Date.parse(resetAt) - Date.now() + 1_000
  return Math.min(Math.max(untilReset, MIN_REFRESH_MS), FOREGROUND_REFRESH_MS)
}

export function useEntitlements() {
  return useQuery({
    queryKey: ENTITLEMENTS_QUERY_KEY,
    queryFn: getEntitlements,
    staleTime: STALE_MS,
    refetchInterval: nextRefresh,
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
