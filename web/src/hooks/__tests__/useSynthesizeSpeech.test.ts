import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useSynthesizeSpeech } from '../useAnnotations'
import * as api from '@/lib/api'

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }
}

describe('useSynthesizeSpeech', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('replays cached audio without spending speech allowance again', async () => {
    const queryClient = new QueryClient()
    const audio = new Blob(['audio'], { type: 'audio/mpeg' })
    const synth = vi.spyOn(api, 'synthesizeSpeech').mockResolvedValue(audio)
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useSynthesizeSpeech(), { wrapper: createWrapper(queryClient) })

    await act(() => result.current.mutateAsync({ highlightedText: '本', contextText: 'ctx' }))
    const replay = await act(() => result.current.mutateAsync({ highlightedText: '本', contextText: 'ctx' }))
    await act(() => result.current.mutateAsync({ highlightedText: '本', contextText: 'other' }))

    expect(replay).toBe(audio)
    expect(synth).toHaveBeenCalledTimes(2)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['entitlements'] })
  })

  it('does not cache failed synthesis', async () => {
    const queryClient = new QueryClient()
    const synth = vi
      .spyOn(api, 'synthesizeSpeech')
      .mockRejectedValueOnce(new Error('busy'))
      .mockResolvedValueOnce(new Blob(['audio']))
    const { result } = renderHook(() => useSynthesizeSpeech(), { wrapper: createWrapper(queryClient) })

    await act(async () => {
      await expect(result.current.mutateAsync({ highlightedText: '本' })).rejects.toThrow('busy')
    })
    await act(() => result.current.mutateAsync({ highlightedText: '本' }))
    expect(synth).toHaveBeenCalledTimes(2)
  })
})
