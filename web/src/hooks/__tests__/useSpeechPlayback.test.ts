import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useSpeechPlayback } from '../useSpeechPlayback'

describe('useSpeechPlayback', () => {
  let playMock: ReturnType<typeof vi.fn>
  let pauseMock: ReturnType<typeof vi.fn>
  let revokeObjectURLMock: ReturnType<typeof vi.fn>
  let createObjectURLMock: ReturnType<typeof vi.fn>
  let originalAudio: typeof Audio
  let originalURL: typeof URL

  beforeEach(() => {
    vi.clearAllMocks()
    playMock = vi.fn().mockResolvedValue(undefined)
    pauseMock = vi.fn()
    revokeObjectURLMock = vi.fn()
    createObjectURLMock = vi.fn(() => 'blob:audio-url')

    originalAudio = globalThis.Audio
    originalURL = globalThis.URL

    class MockAudio {
      src = ''
      onended: (() => void) | null = null
      play = playMock
      pause = pauseMock
    }

    globalThis.Audio = MockAudio as unknown as typeof Audio
    globalThis.URL = {
      ...originalURL,
      createObjectURL: createObjectURLMock,
      revokeObjectURL: revokeObjectURLMock,
    } as unknown as typeof URL
  })

  afterEach(() => {
    globalThis.Audio = originalAudio
    globalThis.URL = originalURL
  })

  it('starts with isPlaying=false', () => {
    const { result } = renderHook(() => useSpeechPlayback())
    expect(result.current.isPlaying).toBe(false)
  })

  it('plays audio from blob', async () => {
    const { result } = renderHook(() => useSpeechPlayback())

    const blob = new Blob(['audio-data'], { type: 'audio/wav' })

    await act(async () => {
      await result.current.play(blob)
    })

    expect(createObjectURLMock).toHaveBeenCalledWith(blob)
    expect(playMock).toHaveBeenCalled()
    expect(result.current.isPlaying).toBe(true)
  })

  it('stops playback and cleans up resources', async () => {
    const { result } = renderHook(() => useSpeechPlayback())

    const blob = new Blob(['audio-data'], { type: 'audio/wav' })
    await act(async () => {
      await result.current.play(blob)
    })

    act(() => {
      result.current.stop()
    })

    expect(pauseMock).toHaveBeenCalled()
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:audio-url')
    expect(result.current.isPlaying).toBe(false)
  })

  it('cleans up on unmount', async () => {
    const { result, unmount } = renderHook(() => useSpeechPlayback())

    const blob = new Blob(['audio-data'], { type: 'audio/wav' })
    await act(async () => {
      await result.current.play(blob)
    })

    unmount()

    expect(pauseMock).toHaveBeenCalled()
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:audio-url')
  })

  it('sets isPlaying=false when audio ends naturally', async () => {
    let capturedOnEnded: (() => void) | null = null

    class MockAudioWithEndCapture {
      src = ''
      private _onended: (() => void) | null = null
      get onended() { return this._onended }
      set onended(fn: (() => void) | null) {
        this._onended = fn
        capturedOnEnded = fn
      }
      play = playMock
      pause = pauseMock
    }

    globalThis.Audio = MockAudioWithEndCapture as unknown as typeof Audio

    const { result } = renderHook(() => useSpeechPlayback())
    const blob = new Blob(['audio-data'], { type: 'audio/wav' })

    await act(async () => {
      await result.current.play(blob)
    })

    expect(result.current.isPlaying).toBe(true)

    act(() => {
      capturedOnEnded?.()
    })

    expect(result.current.isPlaying).toBe(false)
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:audio-url')
  })
})
