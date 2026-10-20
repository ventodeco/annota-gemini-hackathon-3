import { QueryClient } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearUserScopedCaches, purgeLegacyBrowserData } from '../authSession'

const cachesDelete = vi.fn()

describe('purgeLegacyBrowserData', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    cachesDelete.mockResolvedValue(true)
    vi.stubGlobal('caches', { delete: cachesDelete })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('removes the unscoped API cache and legacy account keys', async () => {
    await purgeLegacyBrowserData()

    expect(cachesDelete).toHaveBeenCalledWith('api-cache')
    expect(window.localStorage.removeItem).toHaveBeenCalledWith('auth_token')
    expect(window.localStorage.removeItem).toHaveBeenCalledWith('savedAnnotations')
  })

  it('still purges the cache when localStorage is unavailable', async () => {
    vi.mocked(window.localStorage.removeItem).mockImplementation(() => {
      throw new DOMException('denied', 'SecurityError')
    })

    await expect(purgeLegacyBrowserData()).resolves.toBeUndefined()
    expect(cachesDelete).toHaveBeenCalledWith('api-cache')
    vi.mocked(window.localStorage.removeItem).mockReset()
  })

  it('works where Cache Storage does not exist', async () => {
    vi.unstubAllGlobals()
    vi.stubGlobal('caches', undefined)

    await expect(purgeLegacyBrowserData()).resolves.toBeUndefined()
    expect(window.localStorage.removeItem).toHaveBeenCalledWith('savedAnnotations')
  })

  it('clears query data together with legacy browser data', async () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(['documents'], ['private document'])

    await clearUserScopedCaches(queryClient)

    expect(queryClient.getQueryData(['documents'])).toBeUndefined()
    expect(cachesDelete).toHaveBeenCalledWith('api-cache')
    expect(window.localStorage.removeItem).toHaveBeenCalledWith('savedAnnotations')
  })
})
