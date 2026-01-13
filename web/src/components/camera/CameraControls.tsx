import { Button } from '@/components/ui/button'
import { Camera, X, RotateCcw, Check } from 'lucide-react'

interface CameraControlsProps {
  onCapture: () => void
  onClose: () => void
  onSwitch?: () => void
  isCapturing?: boolean
  isPreview?: boolean
  onRetake?: () => void
  onConfirm?: () => void
}

export default function CameraControls({
  onCapture,
  onClose,
  onSwitch,
  isCapturing = false,
  isPreview = false,
  onRetake,
  onConfirm,
}: CameraControlsProps) {
  if (isPreview) {
    return (
      <div className="
        safe-area-inset-bottom absolute inset-x-0 bottom-0 bg-black/50
        backdrop-blur-sm
      ">
        <div className="mx-auto flex max-w-md items-center justify-between p-6">
          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
            className="
              size-12 text-white
              hover:bg-white/20
            "
            aria-label="Close camera"
          >
            <X className="size-6" />
          </Button>
          <Button
            onClick={onRetake}
            className="
              size-16 rounded-full bg-white text-gray-900
              hover:bg-gray-100
            "
            aria-label="Retake photo"
          >
            <RotateCcw className="size-8" />
          </Button>
          <Button
            onClick={onConfirm}
            className="
              size-16 rounded-full bg-green-500 text-white
              hover:bg-green-600
            "
            aria-label="Confirm photo"
          >
            <Check className="size-8" />
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="
      safe-area-inset-bottom absolute inset-x-0 bottom-0 bg-black/50
      backdrop-blur-sm
    ">
      <div className="mx-auto flex max-w-md items-center justify-between p-6">
        <Button
          variant="ghost"
          size="icon"
          onClick={onClose}
          className="
            size-12 text-white
            hover:bg-white/20
          "
          aria-label="Close camera"
        >
          <X className="size-6" />
        </Button>
        <Button
          onClick={onCapture}
          disabled={isCapturing}
          className="
            size-16 rounded-full bg-white text-gray-900
            hover:bg-gray-100
            disabled:opacity-50
          "
          aria-label="Capture photo"
        >
          <Camera className="size-8" />
        </Button>
        {onSwitch && (
          <Button
            variant="ghost"
            size="icon"
            onClick={onSwitch}
            className="
              size-12 text-white
              hover:bg-white/20
            "
            aria-label="Switch camera"
          >
            <RotateCcw className="size-6" />
          </Button>
        )}
        {!onSwitch && <div className="w-12" />}
      </div>
    </div>
  )
}
