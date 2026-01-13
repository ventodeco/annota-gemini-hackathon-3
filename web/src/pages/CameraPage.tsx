import { useEffect, useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCamera } from '@/hooks/useCamera'
import CameraView from '@/components/camera/CameraView'
import CameraControls from '@/components/camera/CameraControls'
import CameraError from '@/components/camera/CameraError'

export default function CameraPage() {
  const navigate = useNavigate()
  const { stream, error, isSupported, videoRef, startCamera, stopCamera, switchCamera, capturePhoto } = useCamera()
  const [isCapturing, setIsCapturing] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const capturedBlobRef = useRef<Blob | null>(null)

  useEffect(() => {
    if (isSupported && !previewUrl) {
      startCamera()
    }
    return () => {
      stopCamera()
    }
  }, [isSupported, startCamera, stopCamera, previewUrl])

  useEffect(() => {
    return () => {
      if (previewUrl) {
        URL.revokeObjectURL(previewUrl)
      }
    }
  }, [previewUrl])

  const handleCapture = async () => {
    if (isCapturing) return

    setIsCapturing(true)
    const blob = await capturePhoto()

    if (blob) {
      const imageUrl = URL.createObjectURL(blob)
      capturedBlobRef.current = blob
      setPreviewUrl(imageUrl)
      stopCamera()
    }
    setIsCapturing(false)
  }

  const handleRetake = async () => {
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl)
      setPreviewUrl(null)
      capturedBlobRef.current = null
    }
    await startCamera()
  }

  const handleConfirm = async () => {
    if (!capturedBlobRef.current) return

    const blob = capturedBlobRef.current
    const reader = new FileReader()
    reader.onloadend = () => {
      const base64data = reader.result as string
      sessionStorage.setItem('pendingImage', JSON.stringify({
        blob: base64data,
        type: 'image/jpeg',
        source: 'camera',
      }))
      if (previewUrl) {
        URL.revokeObjectURL(previewUrl)
      }
      navigate('/loading')
    }
    reader.readAsDataURL(blob)
  }

  const handleClose = async () => {
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl)
      setPreviewUrl(null)
      capturedBlobRef.current = null
      await startCamera()
    } else {
      stopCamera()
      navigate('/welcome')
    }
  }

  if (!isSupported) {
    return (
      <CameraError
        error="Camera is not supported on this device. Please use the upload option instead."
        onClose={handleClose}
      />
    )
  }

  if (error) {
    return (
      <CameraError
        error={error}
        onRetry={startCamera}
        onClose={handleClose}
      />
    )
  }

  return (
    <div className="fixed inset-0 z-50 bg-black">
      <div className="absolute inset-0">
        {previewUrl ? (
          <img
            src={previewUrl}
            alt="Preview"
            className="size-full object-cover"
            style={{ transform: 'scaleX(-1)' }}
          />
        ) : (
          <CameraView videoRef={videoRef} stream={stream} />
        )}
      </div>
      <CameraControls
        onCapture={handleCapture}
        onClose={handleClose}
        onSwitch={previewUrl ? undefined : switchCamera}
        isCapturing={isCapturing}
        isPreview={!!previewUrl}
        onRetake={handleRetake}
        onConfirm={handleConfirm}
      />
    </div>
  )
}
