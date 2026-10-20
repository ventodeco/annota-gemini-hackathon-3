// Daily beta allowance returned by GET /v1/entitlements/me. Validated at the
// API boundary so components can trust the shape.

export type UsageFeature = 'ocr' | 'explanation' | 'speech'

export interface FeatureAllowance {
  feature: UsageFeature
  unit: string
  limit: number
  used: number
  remaining: number
  maxPerRequest?: number
  globalAvailable: boolean
}

export interface UsageAllowance {
  period: string
  timezone: string
  resetAt: string
  features: FeatureAllowance[]
}

export interface Entitlements {
  plan: string
  usage: UsageAllowance
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0
}

export function isUsageFeature(value: unknown): value is UsageFeature {
  return value === 'ocr' || value === 'explanation' || value === 'speech'
}

function parseFeature(value: unknown): FeatureAllowance | null {
  if (!isRecord(value)) return null
  const { feature, unit, limit, used, remaining, maxPerRequest, globalAvailable } = value
  if (!isUsageFeature(feature) || typeof unit !== 'string' || typeof globalAvailable !== 'boolean') return null
  if (!isCount(limit) || !isCount(used) || !isCount(remaining)) return null
  if (maxPerRequest !== undefined && !isCount(maxPerRequest)) return null
  return { feature, unit, limit, used, remaining, maxPerRequest, globalAvailable }
}

export function parseEntitlements(value: unknown): Entitlements | null {
  if (!isRecord(value) || typeof value.plan !== 'string' || !isRecord(value.usage)) return null
  const { period, timezone, resetAt, features } = value.usage
  if (typeof period !== 'string' || typeof timezone !== 'string' || typeof resetAt !== 'string') return null
  if (Number.isNaN(Date.parse(resetAt)) || !Array.isArray(features)) return null

  const parsed: FeatureAllowance[] = []
  for (const item of features) {
    const feature = parseFeature(item)
    if (!feature) return null
    parsed.push(feature)
  }
  return { plan: value.plan, usage: { period, timezone, resetAt, features: parsed } }
}
