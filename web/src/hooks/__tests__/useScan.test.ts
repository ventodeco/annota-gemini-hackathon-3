import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useScan, isScanOcrReady } from '../useScan'
import * as api from '@/lib/api'

describe('useScan', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }

  it('should fetch scan data', async () => {
    const mockData = {
      id: 123,
      imageUrl: '/uploads/123.jpg',
      detectedLanguage: 'JP',
      fullText: 'test full text',
      createdAt: '2026-02-09T00:00:00Z',
    }

    vi.spyOn(api, 'getScan').mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useScan(123), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toEqual(mockData)
  })

  it('should not fetch when scanID is undefined', () => {
    const { result } = renderHook(() => useScan(undefined), { wrapper })

    expect(result.current.isLoading).toBe(false)
    expect(api.getScan).not.toHaveBeenCalled()
  })

  it('should poll when status is uploaded', async () => {
    const mockData = {
      id: 123,
      imageUrl: '/uploads/123.jpg',
      detectedLanguage: 'JP',
      fullText: '',
      createdAt: '2026-02-09T00:00:00Z',
    }

    vi.spyOn(api, 'getScan').mockResolvedValue(mockData)

    const { result } = renderHook(() => useScan(123, { pollIntervalMs: 100 }), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    await waitFor(() => expect(api.getScan).toHaveBeenCalledTimes(3), { timeout: 2000 })
  })

  it('should stop polling when status is ocr_done', async () => {
    const mockData = {
      id: 123,
      imageUrl: '/uploads/123.jpg',
      detectedLanguage: 'JP',
      fullText: 'test full text',
      createdAt: '2026-02-09T00:00:00Z',
    }

    vi.spyOn(api, 'getScan').mockResolvedValue(mockData)

    const { result } = renderHook(() => useScan(123, { pollIntervalMs: 100 }), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 400))
    })

    expect(api.getScan).toHaveBeenCalledTimes(1)
  })

  it('should not fetch when hook is disabled', () => {
    renderHook(() => useScan(123, { enabled: false }), { wrapper })
    expect(api.getScan).not.toHaveBeenCalled()
  })
})

describe('isScanOcrReady', () => {
  it('returns false for missing fullText', () => {
    expect(isScanOcrReady(undefined)).toBe(false)
    expect(isScanOcrReady({ fullText: '' })).toBe(false)
    expect(isScanOcrReady({ fullText: '   ' })).toBe(false)
  })

  it('returns true for non-empty fullText', () => {
    expect(isScanOcrReady({ fullText: 'OCR complete' })).toBe(true)
  })
})
