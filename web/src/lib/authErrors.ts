export function isAuthError(error: unknown): boolean {
  if (error instanceof Error) {
    const message = error.message.toLowerCase()
    return (
      message.includes('unauthorized') ||
      message.includes('invalid token') ||
      message.includes('missing token') ||
      message.includes('token') ||
      message.includes('401')
    )
  }
  return false
}

export function getAuthErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    const message = error.message.toLowerCase()

    if (message.includes('unauthorized') || message.includes('missing token')) {
      return 'Your session has expired. Please log in again.'
    }

    if (message.includes('invalid token')) {
      return 'Invalid authentication token. Please log in again.'
    }

    if (message.includes('forbidden') || message.includes('access denied')) {
      return 'You do not have permission to access this resource.'
    }

    return error.message
  }

  return 'An unexpected error occurred. Please try again.'
}

export function handleAuthRedirect(): void {
  localStorage.removeItem('auth_token')
  window.location.href = '/login'
}
