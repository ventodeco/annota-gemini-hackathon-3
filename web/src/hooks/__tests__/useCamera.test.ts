import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useCamera } from '../useCamera'

function createFakeStream() {
  const track = { stop: vi.fn() }
  return { track, stream: { getTracks: () => [track] } }
}

function deferred<T>() {
  let resolve: (value: T) => void = () => {}
  let reject: (reason: unknown) => void = () => {}
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const originalMediaDevices = Object.getOwnPropertyDescriptor(navigator, 'mediaDevices')

describe('useCamera', () => {
  let getUserMedia: ReturnType<typeof vi.fn>

  beforeEach(() => {
    getUserMedia = vi.fn()
    Object.defineProperty(navigator, 'mediaDevices', {
      value: { getUserMedia },
      configurable: true,
    })
  })

  afterEach(() => {
    if (originalMediaDevices) {
      Object.defineProperty(navigator, 'mediaDevices', originalMediaDevices)
    } else {
      Reflect.deleteProperty(navigator, 'mediaDevices')
    }
  })

  it('turns the camera off when the user leaves while the permission prompt is open', async () => {
    const pending = deferred<unknown>()
    getUserMedia.mockReturnValue(pending.promise)
    const { result, unmount } = renderHook(() => useCamera())

    let start: Promise<void> = Promise.resolve()
    act(() => {
      start = result.current.startCamera()
    })
    unmount()

    const late = createFakeStream()
    await act(async () => {
      pending.resolve(late.stream)
      await start
    })

    expect(late.track.stop).toHaveBeenCalled()
  })

  it('switches to the other camera and releases the previous one', async () => {
    const back = createFakeStream()
    const front = createFakeStream()
    getUserMedia.mockResolvedValueOnce(back.stream).mockResolvedValueOnce(front.stream)
    const { result } = renderHook(() => useCamera())

    await act(() => result.current.startCamera())
    expect(result.current.stream).toBe(back.stream)

    await act(() => result.current.switchCamera())

    expect(back.track.stop).toHaveBeenCalled()
    expect(getUserMedia).toHaveBeenLastCalledWith(
      expect.objectContaining({ video: expect.objectContaining({ facingMode: 'user' }) })
    )
    expect(result.current.facingMode).toBe('user')
    expect(result.current.stream).toBe(front.stream)
  })

  it('shows the permission error and clears it after a successful retry', async () => {
    const granted = createFakeStream()
    getUserMedia
      .mockRejectedValueOnce(new Error('Permission denied'))
      .mockResolvedValueOnce(granted.stream)
    const { result } = renderHook(() => useCamera())

    await act(() => result.current.startCamera())
    expect(result.current.error).toBe('Permission denied')
    expect(result.current.stream).toBeNull()

    await act(() => result.current.startCamera())
    expect(result.current.error).toBeNull()
    expect(result.current.stream).toBe(granted.stream)
  })

  describe('capturePhoto', () => {
    const drawImage = vi.fn()
    const originalGetContext = Object.getOwnPropertyDescriptor(HTMLCanvasElement.prototype, 'getContext')
    const originalToBlob = Object.getOwnPropertyDescriptor(HTMLCanvasElement.prototype, 'toBlob')

    beforeEach(() => {
      drawImage.mockClear()
      Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
        value: () => ({ drawImage }),
        configurable: true,
      })
      Object.defineProperty(HTMLCanvasElement.prototype, 'toBlob', {
        value: (callback: (blob: Blob | null) => void) => callback(new Blob(['jpeg'])),
        configurable: true,
      })
    })

    afterEach(() => {
      if (originalGetContext) Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', originalGetContext)
      if (originalToBlob) Object.defineProperty(HTMLCanvasElement.prototype, 'toBlob', originalToBlob)
    })

    it('captures at the current video resolution after the camera changes', async () => {
      getUserMedia.mockResolvedValue(createFakeStream().stream)
      const video = document.createElement('video')
      Object.defineProperty(video, 'play', { value: () => Promise.resolve() })
      const size = { width: 1920, height: 1080 }
      Object.defineProperty(video, 'videoWidth', { get: () => size.width })
      Object.defineProperty(video, 'videoHeight', { get: () => size.height })

      const { result } = renderHook(() => useCamera())
      result.current.videoRef.current = video
      await act(() => result.current.startCamera())

      await expect(result.current.capturePhoto()).resolves.toBeInstanceOf(Blob)
      expect(drawImage).toHaveBeenLastCalledWith(video, 0, 0, 1920, 1080)

      size.width = 1280
      size.height = 720
      await result.current.capturePhoto()
      expect(drawImage).toHaveBeenLastCalledWith(video, 0, 0, 1280, 720)
    })
  })
})
