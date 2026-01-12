import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useScan } from '../useScan'
import * as api from '@/lib/api'

vi.mock('@/lib/api')

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

  const wrapper = ({ children }: { children: React.ReactNode }) => {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }

  it('should fetch scan data', async () => {
    const mockData = {
      scan: { id: 'test-id', status: 'ocr_done' },
      ocrResult: { id: 'ocr-id', rawText: 'test' },
      status: 'ocr_done',
    }

    vi.mocked(api.getScan).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useScan('test-id'), { wrapper })

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
      scan: { id: 'test-id', status: 'uploaded' },
      ocrResult: null,
      status: 'uploaded',
    }

    vi.mocked(api.getScan).mockResolvedValue(mockData)

    const { result } = renderHook(() => useScan('test-id'), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    await waitFor(
      () => expect(api.getScan).toHaveBeenCalledTimes(2),
      { timeout: 3000 }
    )
  })

  it('should stop polling when status is ocr_done', async () => {
    const mockData = {
      scan: { id: 'test-id', status: 'ocr_done' },
      ocrResult: { id: 'ocr-id', rawText: 'test' },
      status: 'ocr_done',
    }

    vi.mocked(api.getScan).mockResolvedValue(mockData)

    const { result } = renderHook(() => useScan('test-id'), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    await new Promise((resolve) => setTimeout(resolve, 2500))

    expect(api.getScan).toHaveBeenCalledTimes(1)
  })
})
