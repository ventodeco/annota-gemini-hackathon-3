import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

interface ErrorAlertProps {
  title: string
  message: string
  onRetry?: () => void
  retryLabel?: string
}

export default function ErrorAlert({
  title,
  message,
  onRetry,
  retryLabel = 'Retry',
}: ErrorAlertProps) {
  return (
    <Alert variant="destructive" className="m-4">
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription className="mt-2">
        {message}
      </AlertDescription>
      {onRetry && (
        <div className="mt-4">
          <Button onClick={onRetry} variant="outline" size="sm">
            {retryLabel}
          </Button>
        </div>
      )}
    </Alert>
  )
}
