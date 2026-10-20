import type { QueryClient } from '@tanstack/react-query'

// Runtime cache populated by the service worker for API responses (see vite.config.ts).
const API_CACHE_NAME = 'api-cache'
// Pre-Clerk builds stored a locally minted token under this key.
const LEGACY_TOKEN_KEY = 'auth_token'

/** Server message for a session whose ANNOTA account was deleted. */
export const ACCOUNT_DELETED_MESSAGE = 'Unauthorized: account deleted'

type UnauthorizedHandler = (message: string) => void

let unauthorizedHandler: UnauthorizedHandler | null = null

/** Registers the handler that ends the Clerk session after an API 401. */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler
}

export function notifyUnauthorized(message: string): void {
  unauthorizedHandler?.(message)
}

/**
 * Removes user-scoped data held in memory and browser caches so the next
 * account (or a signed-out visitor) cannot see it.
 */
export async function clearUserScopedCaches(queryClient: QueryClient): Promise<void> {
  queryClient.clear()
  try {
    window.localStorage.removeItem(LEGACY_TOKEN_KEY)
  } catch {
    // Storage can be unavailable (private mode); nothing to clear then.
  }
  if (typeof caches !== 'undefined') {
    await caches.delete(API_CACHE_NAME).catch(() => false)
  }
}

const AUTH_PATH_PREFIXES = ['/login', '/sign-up']

/**
 * Returns a same-origin in-app path from a redirect_url value, or null when
 * it is missing, cross-origin, or points back into the sign-in flow.
 */
export function safeRedirectPath(value: string | null): string | null {
  if (!value) return null
  let target: URL
  try {
    target = new URL(value, window.location.origin)
  } catch {
    return null
  }
  if (target.origin !== window.location.origin) return null
  if (AUTH_PATH_PREFIXES.some((prefix) => target.pathname === prefix || target.pathname.startsWith(`${prefix}/`))) {
    return null
  }
  return `${target.pathname}${target.search}${target.hash}`
}
