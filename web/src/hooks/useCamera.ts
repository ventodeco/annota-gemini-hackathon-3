import { useState, useEffect, useRef, useCallback } from 'react'

interface UseCameraReturn {
  stream: MediaStream | null
  error: string | null
  isSupported: boolean
  facingMode: 'user' | 'environment'
  videoRef: React.RefObject<HTMLVideoElement | null>
  startCamera: () => Promise<void>
  stopCamera: () => void
  switchCamera: () => Promise<void>
  capturePhoto: () => Promise<Blob | null>
}

export function useCamera(): UseCameraReturn {
  const [stream, setStream] = useState<MediaStream | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [facingMode, setFacingMode] = useState<'user' | 'environment'>('environment')
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)

  const isSupported = typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia

  useEffect(() => {
    streamRef.current = stream
  }, [stream])

  const stopCamera = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop())
      streamRef.current = null
      setStream(null)
    }
  }, [])

  const startCamera = useCallback(async () => {
    if (!isSupported) {
      setError('Camera is not supported on this device')
      return
    }

    try {
      stopCamera()

      const mediaStream = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode,
          width: { ideal: 1920 },
          height: { ideal: 1080 },
        },
        audio: false,
      })

      setStream(mediaStream)
      setError(null)

      if (videoRef.current) {
        videoRef.current.srcObject = mediaStream
        videoRef.current.play().catch((err) => {
          console.error('Error playing video:', err)
        })
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error
          ? err.message
          : 'Failed to access camera. Please check permissions.'
      setError(errorMessage)
      setStream(null)
    }
  }, [isSupported, facingMode, stopCamera])

  const switchCamera = useCallback(async () => {
    const newFacingMode = facingMode === 'user' ? 'environment' : 'user'
    setFacingMode(newFacingMode)
    await startCamera()
  }, [facingMode, startCamera])

  const capturePhoto = useCallback(async (): Promise<Blob | null> => {
    if (!videoRef.current || !stream) {
      return null
    }

    if (!canvasRef.current) {
      const canvas = document.createElement('canvas')
      canvas.width = videoRef.current.videoWidth
      canvas.height = videoRef.current.videoHeight
      canvasRef.current = canvas
    }

    const canvas = canvasRef.current
    const ctx = canvas.getContext('2d')

    if (!ctx) {
      return null
    }

    ctx.drawImage(videoRef.current, 0, 0, canvas.width, canvas.height)

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

  useEffect(() => {
    return () => {
      if (streamRef.current) {
        streamRef.current.getTracks().forEach((track) => track.stop())
        streamRef.current = null
      }
    }
  }, [])

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
