import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getToken } from '@clerk/react'
import { ApiError, getEntitlements } from '../api'
import { setUnauthorizedHandler } from '../authSession'
import { parseEntitlements, type FeatureAllowance } from '../entitlements'
import { getAllowanceState, getUsageErrorMessage } from '../usage'

vi.mock('@clerk/react', () => ({
  getToken: vi.fn(),
}))

const entitlementsBody = {
  plan: 'free',
  limits: { aiRequestsPerWindow: 60, aiWindowSeconds: 3600, maxUploadSize: 10485760 },
  usage: {
    period: 'daily',
    timezone: 'UTC',
    resetAt: '2026-10-21T00:00:00Z',
    features: [
      { feature: 'ocr', unit: 'page', limit: 10, used: 3, remaining: 7, globalAvailable: true },
      { feature: 'explanation', unit: 'request', limit: 50, used: 50, remaining: 0, globalAvailable: true },
      { feature: 'speech', unit: 'character', limit: 1500, used: 0, remaining: 1500, maxPerRequest: 250, globalAvailable: false },
    ],
  },
}

function allowance(overrides: Partial<FeatureAllowance>): FeatureAllowance {
  return { feature: 'ocr', unit: 'page', limit: 10, used: 0, remaining: 10, globalAvailable: true, ...overrides }
}

describe('parseEntitlements', () => {
  it('accepts the API shape', () => {
    const parsed = parseEntitlements(entitlementsBody)
    expect(parsed?.usage.features.map((f) => f.feature)).toEqual(['ocr', 'explanation', 'speech'])
    expect(parsed?.usage.features[2].maxPerRequest).toBe(250)
  })

  it.each([
    ['missing usage', { plan: 'free' }],
    ['unknown feature', { ...entitlementsBody, usage: { ...entitlementsBody.usage, features: [{ ...entitlementsBody.usage.features[0], feature: 'video' }] } }],
    ['negative count', { ...entitlementsBody, usage: { ...entitlementsBody.usage, features: [{ ...entitlementsBody.usage.features[0], remaining: -1 }] } }],
    ['invalid reset time', { ...entitlementsBody, usage: { ...entitlementsBody.usage, resetAt: 'soon' } }],
  ])('rejects %s', (_name, body) => {
    expect(parseEntitlements(body)).toBeNull()
  })
})

describe('getAllowanceState', () => {
  it.each([
    ['available', allowance({ used: 3, remaining: 7 })],
    ['low', allowance({ used: 8, remaining: 2 })],
    ['exhausted', allowance({ used: 10, remaining: 0 })],
    ['disabled', allowance({ limit: 0, remaining: 0 })],
    ['paused', allowance({ used: 5, remaining: 5, globalAvailable: false })],
  ])('derives %s', (state, value) => {
    expect(getAllowanceState(value)).toBe(state)
  })
})

describe('usage errors', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getToken).mockResolvedValue('session-token')
    setUnauthorizedHandler(null)
  })

  it('loads and validates entitlements', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce(Response.json(entitlementsBody))
    const result = await getEntitlements()
    expect(result.usage.features[0].remaining).toBe(7)

    global.fetch = vi.fn().mockResolvedValueOnce(Response.json({ plan: 'free' }))
    await expect(getEntitlements()).rejects.toThrow('Unexpected entitlements response')
  })

  it('keeps the session on a quota rejection and exposes its code', async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    global.fetch = vi.fn().mockResolvedValueOnce(
      Response.json(
        { error: 'Too Many Requests', message: 'quota', code: 'usage_quota_exceeded', feature: 'explanation', resetAt: '2026-10-21T00:00:00Z' },
        { status: 429 },
      ),
    )

    const error = await getEntitlements().catch((err: unknown) => err)
    expect(error).toBeInstanceOf(ApiError)
    if (!(error instanceof ApiError)) return
    expect(error.code).toBe('usage_quota_exceeded')
    expect(onUnauthorized).not.toHaveBeenCalled()
    expect(getUsageErrorMessage(error)).toMatch(/^You've used today's explanation allowance\. It resets at .+\.$/)
  })

  it.each([
    ['usage_budget_exhausted', { feature: 'ocr', resetAt: '2026-10-21T00:00:00Z' }, /beta capacity for image scan is used up/],
    ['usage_request_too_large', { feature: 'speech', max: 250 }, /Select up to 250 characters/],
    ['usage_unavailable', {}, /could not be checked/],
    ['rate_limited', {}, /Too many requests/],
    ['ai_provider_limited', {}, /AI service is busy/],
  ])('describes %s', (code, extra, expected) => {
    const error = new ApiError(429, JSON.stringify({ message: 'x', code, ...extra }))
    expect(getUsageErrorMessage(error)).toMatch(expected)
  })

  it('ignores unrelated errors', () => {
    expect(getUsageErrorMessage(new ApiError(500, 'boom'))).toBeNull()
    expect(getUsageErrorMessage(new Error('network'))).toBeNull()
  })
})
