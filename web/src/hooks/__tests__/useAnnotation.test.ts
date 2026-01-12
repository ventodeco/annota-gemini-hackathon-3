import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useAnnotation } from '../useAnnotation'
import * as api from '@/lib/api'

vi.mock('@/lib/api')

describe('useAnnotation', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        mutations: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }

  it('should submit annotation successfully', async () => {
    const mockResponse = {
      id: 'ann-id',
      meaning: 'test meaning',
      usageExample: 'test example',
      whenToUse: 'test when',
      wordBreakdown: 'test breakdown',
      alternativeMeanings: 'test alt',
    }

    vi.mocked(api.annotate).mockResolvedValueOnce(mockResponse)

    const { result } = renderHook(() => useAnnotation('scan-id'), { wrapper })

    result.current.mutate({ selectedText: 'test text' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toEqual(mockResponse)
    expect(api.annotate).toHaveBeenCalledWith('scan-id', {
      selectedText: 'test text',
    })
  })

  it('should handle errors', async () => {
    const error = new Error('Failed to annotate')
    vi.mocked(api.annotate).mockRejectedValueOnce(error)

    const { result } = renderHook(() => useAnnotation('scan-id'), { wrapper })

    result.current.mutate({ selectedText: 'test text' })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(result.current.error).toEqual(error)
  })
})
