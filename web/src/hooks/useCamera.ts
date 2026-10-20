import { useState, useEffect, useRef, useCallback } from 'react'

type FacingMode = 'user' | 'environment'

interface UseCameraReturn {
  stream: MediaStream | null
  error: string | null
  isSupported: boolean
  facingMode: FacingMode
  videoRef: React.RefObject<HTMLVideoElement | null>
  startCamera: () => Promise<void>
  stopCamera: () => void
  switchCamera: () => Promise<void>
  capturePhoto: () => Promise<Blob | null>
}

function stopTracks(mediaStream: MediaStream) {
  mediaStream.getTracks().forEach((track) => track.stop())
}

export function useCamera(): UseCameraReturn {
  const [stream, setStream] = useState<MediaStream | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [facingMode, setFacingMode] = useState<FacingMode>('environment')
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  // Incremented whenever a pending camera request must be abandoned (new request, stop, unmount).
  const requestIdRef = useRef(0)

  const isSupported = typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia

  const releaseStream = useCallback(() => {
    requestIdRef.current += 1
    if (streamRef.current) {
      stopTracks(streamRef.current)
      streamRef.current = null
    }
  }, [])

  const stopCamera = useCallback(() => {
    releaseStream()
    setStream(null)
  }, [releaseStream])

  const openCamera = useCallback(async (mode: FacingMode) => {
    if (!isSupported) {
      setError('Camera is not supported on this device')
      return
    }

    stopCamera()
    const requestId = requestIdRef.current

    try {
      const mediaStream = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode: mode,
          width: { ideal: 1920 },
          height: { ideal: 1080 },
        },
        audio: false,
      })

      if (requestId !== requestIdRef.current) {
        stopTracks(mediaStream)
        return
      }

      streamRef.current = mediaStream
      setStream(mediaStream)
      setError(null)

      if (videoRef.current) {
        videoRef.current.srcObject = mediaStream
        videoRef.current.play().catch((err) => {
          console.error('Error playing video:', err)
        })
      }
    } catch (err) {
      if (requestId !== requestIdRef.current) return
      const errorMessage =
        err instanceof Error
          ? err.message
          : 'Failed to access camera. Please check permissions.'
      setError(errorMessage)
      setStream(null)
    }
  }, [isSupported, stopCamera])

  const startCamera = useCallback(() => openCamera(facingMode), [openCamera, facingMode])

  const switchCamera = useCallback(async () => {
    const newFacingMode = facingMode === 'user' ? 'environment' : 'user'
    setFacingMode(newFacingMode)
    await openCamera(newFacingMode)
  }, [facingMode, openCamera])

  const capturePhoto = useCallback(async (): Promise<Blob | null> => {
    const video = videoRef.current
    if (!video || !stream) {
      return null
    }

    if (!canvasRef.current) {
      canvasRef.current = document.createElement('canvas')
    }

    const canvas = canvasRef.current
    // Resolution can change after switching cameras, so size the canvas for every capture.
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    const ctx = canvas.getContext('2d')

    if (!ctx) {
      return null
    }

    ctx.drawImage(video, 0, 0, canvas.width, canvas.height)

    return new Promise((resolve) => {
      canvas.toBlob(
        (blob) => {
          resolve(blob)
        },
        'image/jpeg',
        0.9
      )
    })
  }, [stream])

  useEffect(() => releaseStream, [releaseStream])

  return {
    stream,
    error,
    isSupported,
    facingMode,
    videoRef,
    startCamera,
    stopCamera,
    switchCamera,
    capturePhoto,
  }
}
