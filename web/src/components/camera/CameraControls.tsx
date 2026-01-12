import { Button } from '@/components/ui/button'
import { Camera, X, RotateCcw } from 'lucide-react'

interface CameraControlsProps {
  onCapture: () => void
  onClose: () => void
  onSwitch?: () => void
  isCapturing?: boolean
}

export default function CameraControls({
  onCapture,
  onClose,
  onSwitch,
  isCapturing = false,
}: CameraControlsProps) {
  return (
    <div className="absolute bottom-0 left-0 right-0 bg-black/50 backdrop-blur-sm safe-area-inset-bottom">
      <div className="flex items-center justify-between px-6 py-6 max-w-md mx-auto">
        <Button
          variant="ghost"
          size="icon"
          onClick={onClose}
          className="h-12 w-12 text-white hover:bg-white/20"
          aria-label="Close camera"
        >
          <X className="w-6 h-6" />
        </Button>
        <Button
          onClick={onCapture}
          disabled={isCapturing}
          className="h-16 w-16 rounded-full bg-white text-gray-900 hover:bg-gray-100 disabled:opacity-50"
          aria-label="Capture photo"
        >
          <Camera className="w-8 h-8" />
        </Button>
        {onSwitch && (
          <Button
            variant="ghost"
            size="icon"
            onClick={onSwitch}
            className="h-12 w-12 text-white hover:bg-white/20"
            aria-label="Switch camera"
          >
            <RotateCcw className="w-6 h-6" />
          </Button>
        )}
        {!onSwitch && <div className="w-12" />}
      </div>
    </div>
  )
}
