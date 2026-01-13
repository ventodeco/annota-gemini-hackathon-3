import { Spinner } from '@/components/ui/spinner'

interface LoadingSpinnerProps {
  message?: string
}

export default function LoadingSpinner({ message = 'Processing...' }: LoadingSpinnerProps) {
  return (
    <div className="flex flex-col items-center justify-center p-8">
      <Spinner className="mb-4 size-8" />
      <p className="text-gray-600">{message}</p>
    </div>
  )
}
