import { ApiError } from './api'
import { isUsageFeature, type FeatureAllowance, type UsageFeature } from './entitlements'

interface FeatureCopy {
  title: string
  /** Noun used in sentences, e.g. "image scans left". */
  noun: string
  unitSingular: string
  unitPlural: string
}

export const USAGE_FEATURE_COPY: Record<UsageFeature, FeatureCopy> = {
  ocr: { title: 'Image scans', noun: 'image scan', unitSingular: 'page', unitPlural: 'pages' },
  explanation: { title: 'Explanations', noun: 'explanation', unitSingular: 'explanation', unitPlural: 'explanations' },
  speech: { title: 'Speech', noun: 'speech', unitSingular: 'character', unitPlural: 'characters' },
}

export type AllowanceState = 'available' | 'low' | 'exhausted' | 'disabled' | 'paused'

/** Derives how a feature's allowance should be presented. */
export function getAllowanceState(allowance: FeatureAllowance): AllowanceState {
  if (allowance.limit === 0) return 'disabled'
  if (allowance.remaining === 0) return 'exhausted'
  if (!allowance.globalAvailable) return 'paused'
  if (allowance.remaining <= Math.ceil(allowance.limit * 0.2)) return 'low'
  return 'available'
}

export function formatUnits(count: number, feature: UsageFeature): string {
  const copy = USAGE_FEATURE_COPY[feature]
  return `${count.toLocaleString()} ${count === 1 ? copy.unitSingular : copy.unitPlural}`
}

/** Formats the next reset in the reader's local time, e.g. "9:00 AM". */
export function formatResetTime(resetAt: string): string {
  return new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' }).format(new Date(resetAt))
}

function readFeature(error: ApiError): UsageFeature | null {
  const feature = error.data?.feature
  return isUsageFeature(feature) ? feature : null
}

function resetSuffix(error: ApiError): string {
  const resetAt = error.data?.resetAt
  if (typeof resetAt !== 'string' || Number.isNaN(Date.parse(resetAt))) return ''
  return ` It resets at ${formatResetTime(resetAt)}.`
}

/**
 * Returns an actionable message for usage-related API errors (quota, global
 * budget, burst throttling, provider limits), or null for other errors.
 */
export function getUsageErrorMessage(error: unknown): string | null {
  if (!(error instanceof ApiError)) return null
  const feature = readFeature(error)
  const noun = feature ? USAGE_FEATURE_COPY[feature].noun : 'AI'

  switch (error.code) {
    case 'usage_quota_exceeded':
      return `You've used today's ${noun} allowance.${resetSuffix(error)}`
    case 'usage_budget_exhausted':
      return `Today's beta capacity for ${noun} is used up.${resetSuffix(error)} Please try again after the reset.`
    case 'usage_request_too_large': {
      const max = error.data?.max
      if (feature && typeof max === 'number') {
        return `That selection is too long for ${noun}. Select up to ${formatUnits(max, feature)}.`
      }
      return 'That request is too large. Please select less text.'
    }
    case 'usage_unavailable':
      return 'Your allowance could not be checked. Please try again shortly.'
    case 'rate_limited':
      return 'Too many requests at once. Please wait a moment and try again.'
    case 'ai_provider_limited':
      return 'The AI service is busy right now. Please try again in a moment.'
    default:
      return null
  }
}
