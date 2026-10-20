import { describe, it, expect, vi, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider, focusManager } from '@tanstack/react-query'
import React from 'react'
import { useEntitlements } from '../useEntitlements'
import * as api from '@/lib/api'
import type { Entitlements } from '@/lib/entitlements'

function entitlements(remaining: number, resetAt: string): Entitlements {
  return {
    plan: 'free',
    usage: {
      period: 'daily',
      timezone: 'UTC',
      resetAt,
      features: [{ feature: 'ocr', unit: 'page', limit: 20, used: 20 - remaining, remaining, globalAvailable: true }],
    },
  }
}

describe('useEntitlements', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    focusManager.setFocused(undefined)
  })

  it('refreshes a stale allowance when a hidden tab regains focus after the reset', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-20T23:59:00Z'))
    const fetchEntitlements = vi
      .spyOn(api, 'getEntitlements')
      .mockResolvedValueOnce(entitlements(0, '2026-10-21T00:00:00Z'))
      .mockResolvedValueOnce(entitlements(20, '2026-10-22T00:00:00Z'))
    // Mirror App.tsx, which disables focus refetching globally.
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } })
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(QueryClientProvider, { client: queryClient }, children)

    const { result } = renderHook(() => useEntitlements(), { wrapper })
    await waitFor(() => expect(result.current.data?.usage.features[0].remaining).toBe(0))

    act(() => focusManager.setFocused(false))
    vi.setSystemTime(new Date('2026-10-21T08:00:00Z'))
    act(() => focusManager.setFocused(true))

    await waitFor(() => expect(result.current.data?.usage.features[0].remaining).toBe(20))
    expect(fetchEntitlements).toHaveBeenCalledTimes(2)
  })
})
