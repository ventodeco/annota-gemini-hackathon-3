/**
 * The support reference (server request ID) carried by a failed API call, or
 * null when there is none, e.g. a network failure or an offline device.
 */
export function getErrorReference(error: unknown): string | null {
  if (!(error instanceof Error) || !('requestId' in error)) return null
  return typeof error.requestId === 'string' && error.requestId !== '' ? error.requestId : null
}
