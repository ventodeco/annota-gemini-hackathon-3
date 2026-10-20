import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useEntitlements } from '@/hooks/useEntitlements'
import type { Entitlements, FeatureAllowance } from '@/lib/entitlements'
import {
  USAGE_FEATURE_COPY,
  formatResetTime,
  formatUnits,
  getAllowanceState,
  type AllowanceState,
} from '@/lib/usage'

const BAR_COLORS: Record<AllowanceState, string> = {
  available: 'bg-emerald-600',
  low: 'bg-amber-500',
  exhausted: 'bg-red-600',
  disabled: 'bg-gray-300',
  paused: 'bg-gray-400',
}

const STATUS_TEXT_COLORS: Record<AllowanceState, string> = {
  available: 'text-gray-600',
  low: 'text-amber-700',
  exhausted: 'text-red-700',
  disabled: 'text-gray-500',
  paused: 'text-gray-700',
}

function describeAllowance(allowance: FeatureAllowance, state: AllowanceState, resetTime: string): string {
  switch (state) {
    case 'disabled':
      return 'Not available during the beta.'
    case 'exhausted':
      return `Daily limit reached. Resets at ${resetTime}.`
    case 'paused':
      return `Beta capacity is used up for today. Resets at ${resetTime}.`
    default:
      return `${formatUnits(allowance.remaining, allowance.feature)} left of ${allowance.limit.toLocaleString()}`
  }
}

function AllowanceRow({ allowance, resetTime }: { allowance: FeatureAllowance; resetTime: string }) {
  const state = getAllowanceState(allowance)
  const percent = allowance.limit > 0 ? Math.round((allowance.remaining / allowance.limit) * 100) : 0

  return (
    <li className="space-y-1.5">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-sm font-medium text-gray-900">{USAGE_FEATURE_COPY[allowance.feature].title}</span>
        <span className={`text-xs text-right ${STATUS_TEXT_COLORS[state]}`}>
          {describeAllowance(allowance, state, resetTime)}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100" aria-hidden="true">
        <div className={`h-full rounded-full ${BAR_COLORS[state]}`} style={{ width: `${percent}%` }} />
      </div>
    </li>
  )
}

export type AllowanceViewStatus = 'pending' | 'error' | 'success'

interface AllowanceViewProps {
  status: AllowanceViewStatus
  entitlements?: Entitlements
  onRetry: () => void
  isRetrying?: boolean
}

function AllowanceBody({ status, entitlements, onRetry, isRetrying = false }: AllowanceViewProps) {
  if (status === 'pending') {
    return (
      <p role="status" aria-busy="true" className="text-xs text-gray-500">
        Loading your allowance…
      </p>
    )
  }
  if (status === 'error' || !entitlements) {
    return (
      <div role="alert" className="space-y-2">
        <p className="text-xs text-red-700">Your allowance couldn't be loaded.</p>
        <Button type="button" variant="outline" size="sm" className="min-h-11" onClick={onRetry} disabled={isRetrying}>
          {isRetrying ? 'Retrying…' : 'Try again'}
        </Button>
      </div>
    )
  }

  const resetTime = formatResetTime(entitlements.usage.resetAt)
  return (
    <>
      <ul className="space-y-3">
        {entitlements.usage.features.map((allowance) => (
          <AllowanceRow key={allowance.feature} allowance={allowance} resetTime={resetTime} />
        ))}
      </ul>
      <p className="text-xs text-gray-500">
        Allowances reset daily at {resetTime} (00:00 UTC). Replaying audio you just heard doesn't use more speech.
      </p>
    </>
  )
}

/** Presentational allowance card; AllowancePanel supplies live data. */
export function AllowanceView(props: AllowanceViewProps) {
  return (
    <Card className="p-4 space-y-3" aria-labelledby="allowance-heading">
      <div>
        <h2 id="allowance-heading" className="text-sm font-medium">
          Today's beta allowance
        </h2>
        <p className="text-xs text-gray-500 mt-0.5">What you can still do today.</p>
      </div>
      <AllowanceBody {...props} />
    </Card>
  )
}

export function AllowancePanel() {
  const { status, data, refetch, isRefetching } = useEntitlements()
  return (
    <AllowanceView
      status={status}
      entitlements={data}
      onRetry={() => void refetch()}
      isRetrying={isRefetching}
    />
  )
}

/** One-line image-scan allowance shown next to the upload actions. */
export function ScanAllowanceLine({ allowance, resetAt }: { allowance: FeatureAllowance; resetAt: string }) {
  const state = getAllowanceState(allowance)
  const resetTime = formatResetTime(resetAt)

  if (state === 'available' || state === 'low') {
    return (
      <p className={`text-center text-sm ${STATUS_TEXT_COLORS[state]}`}>
        {allowance.remaining} of {allowance.limit} image scans left today
      </p>
    )
  }
  return (
    <p role="status" className={`text-center text-sm ${STATUS_TEXT_COLORS[state]}`}>
      {describeAllowance(allowance, state, resetTime)} PDF reading is still available.
    </p>
  )
}

export function ScanAllowanceNotice() {
  const { data } = useEntitlements()
  const ocr = data?.usage.features.find((allowance) => allowance.feature === 'ocr')
  // Non-critical hint: stay quiet while loading or on error; the server
  // still returns an actionable error if an upload is rejected.
  if (!data || !ocr) return null
  return <ScanAllowanceLine allowance={ocr} resetAt={data.usage.resetAt} />
}
