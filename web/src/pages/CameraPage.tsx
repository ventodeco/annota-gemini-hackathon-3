import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCamera } from '@/hooks/useCamera'
import CameraView from '@/components/camera/CameraView'
import CameraControls from '@/components/camera/CameraControls'
import CameraError from '@/components/camera/CameraError'

export default function CameraPage() {
  const navigate = useNavigate()
  const { stream, error, isSupported, videoRef, startCamera, stopCamera, switchCamera, capturePhoto } = useCamera()
  const [isCapturing, setIsCapturing] = useState(false)

  useEffect(() => {
    if (isSupported) {
      startCamera()
    }
    return () => {
      stopCamera()
    }
  }, [isSupported, startCamera, stopCamera])

  const handleCapture = async () => {
    if (isCapturing) return

    setIsCapturing(true)
    const blob = await capturePhoto()

    if (blob) {
      const imageUrl = URL.createObjectURL(blob)
      sessionStorage.setItem('pendingImage', JSON.stringify({
        blob: imageUrl,
        type: 'image/jpeg',
        source: 'camera',
      }))
      stopCamera()
      navigate('/loading')
    } else {
      setIsCapturing(false)
    }
  }

  const handleClose = () => {
    stopCamera()
    navigate('/welcome')
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
    <div className="fixed inset-0 bg-black z-50">
      <div className="absolute inset-0">
        <CameraView videoRef={videoRef} stream={stream} />
      </div>
      <CameraControls
        onCapture={handleCapture}
        onClose={handleClose}
        onSwitch={switchCamera}
        isCapturing={isCapturing}
      />
    </div>
  )
}
