import type { QueryClient } from '@tanstack/react-query'

// Earlier service workers cached API responses here, keyed by URL only. The
// current service worker never writes it; any copy left behind is purged.
const API_CACHE_NAME = 'api-cache'
// Earlier builds kept account data in localStorage under these keys: a locally
// minted token and saved annotations. Neither is scoped to an account.
const LEGACY_STORAGE_KEYS = ['auth_token', 'savedAnnotations']

/** Server message for a session whose ANNOTA account was deleted. */
export const ACCOUNT_DELETED_MESSAGE = 'Unauthorized: account deleted'

export interface UnauthorizedEvent {
  /** Server error message (e.g. ACCOUNT_DELETED_MESSAGE). */
  message: string
  /** Session token the failed request was sent with, if any. */
  token: string | null
}

type UnauthorizedHandler = (event: UnauthorizedEvent) => void

let unauthorizedHandler: UnauthorizedHandler | null = null

/** Registers the handler that ends the Clerk session after an API 401. */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler
}

export function notifyUnauthorized(event: UnauthorizedEvent): void {
  unauthorizedHandler?.(event)
}

export interface TokenSession {
  /** Clerk user ID (`sub`). */
  userId: string
  /** Clerk session ID (`sid`), when present. */
  sessionId: string | null
}

function isTokenPayload(value: unknown): value is { sub: string; sid?: unknown } {
  return typeof value === 'object' && value !== null && 'sub' in value && typeof value.sub === 'string'
}

/**
 * Reads the Clerk user and session IDs from a session token without verifying
 * it. Only used to match a response to the session that sent it; the API
 * performs the actual verification.
 */
export function tokenSession(token: string): TokenSession | null {
  const payload = token.split('.')[1]
  if (!payload) return null
  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/')
    const parsed: unknown = JSON.parse(atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, '=')))
    if (!isTokenPayload(parsed)) return null
    return { userId: parsed.sub, sessionId: typeof parsed.sid === 'string' ? parsed.sid : null }
  } catch {
    return null
  }
}

/**
 * Removes account data that earlier builds stored in the browser without
 * scoping it to an account. Safe to call at any time.
 */
export async function purgeLegacyBrowserData(): Promise<void> {
  for (const key of LEGACY_STORAGE_KEYS) {
    try {
      window.localStorage.removeItem(key)
    } catch {
      // Storage can be unavailable (private mode); nothing to clear then.
    }
  }
  if (typeof caches !== 'undefined') {
    await caches.delete(API_CACHE_NAME).catch(() => false)
  }
}

/**
 * Removes user-scoped data held in memory and browser caches so the next
 * account (or a signed-out visitor) cannot see it.
 */
export async function clearUserScopedCaches(queryClient: QueryClient): Promise<void> {
  queryClient.clear()
  await purgeLegacyBrowserData()
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
