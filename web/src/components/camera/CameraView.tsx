import { useEffect } from 'react'

interface CameraViewProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  stream: MediaStream | null
}

export default function CameraView({ videoRef, stream }: CameraViewProps) {
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
      className="size-full object-cover"
      style={{ transform: 'scaleX(-1)' }}
    />
  )
}
