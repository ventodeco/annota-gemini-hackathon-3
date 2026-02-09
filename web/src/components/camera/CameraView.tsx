import { useEffect } from 'react'

interface CameraViewProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  stream: MediaStream | null
  facingMode: 'user' | 'environment'
}

export default function CameraView({ videoRef, stream, facingMode }: CameraViewProps) {
  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream
      videoRef.current.play().catch((err) => {
        console.error('Error playing video:', err)
      })
    }
  }, [videoRef, stream])

  return (
    <video
      ref={videoRef}
      autoPlay
      playsInline
      muted
      className="w-full h-full object-cover"
      style={facingMode === 'user' ? { transform: 'scaleX(-1)' } : undefined}
    />
  )
}
