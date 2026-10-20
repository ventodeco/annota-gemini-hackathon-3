import { toast } from 'sonner'
import { getErrorReference } from './errorReference'

/** Shows a failed operation, with its support reference when the server sent one. */
export function showErrorToast(message: string, error?: unknown): void {
  const reference = getErrorReference(error)
  toast.error(message, reference ? { description: `Reference: ${reference}` } : undefined)
}
