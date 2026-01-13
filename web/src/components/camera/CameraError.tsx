import { AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

interface CameraErrorProps {
  error: string
  onRetry?: () => void
  onClose?: () => void
}

export default function CameraError({ error, onRetry, onClose }: CameraErrorProps) {
  return (
    <div className="
      flex min-h-screen items-center justify-center bg-gray-900 p-6
    ">
      <Card className="w-full max-w-md">
        <CardHeader>
          <div className="flex items-center gap-2">
            <AlertCircle className="text-destructive size-6" />
            <CardTitle>Camera Error</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <CardDescription>{error}</CardDescription>
          <div className="flex gap-2">
            {onRetry && (
              <Button onClick={onRetry} className="flex-1">
                Try Again
              </Button>
            )}
            {onClose && (
              <Button onClick={onClose} variant="outline" className="flex-1">
                Close
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
