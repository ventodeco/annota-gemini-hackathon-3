import { lazy, type ComponentType, type LazyExoticComponent } from 'react'

/** A route chunk that failed to download, for example offline or after a deploy replaced its hash. */
export class ChunkLoadError extends Error {
  constructor(cause: unknown) {
    super('Failed to load page code', { cause })
    this.name = 'ChunkLoadError'
  }
}

/**
 * React.lazy caches a rejected import, and a redeploy can remove the old chunk, so retrying in
 * place can't recover. Failures are tagged so the error boundary's retry reloads the page.
 */
export function lazyRoute(load: () => Promise<{ default: ComponentType }>): LazyExoticComponent<ComponentType> {
  return lazy(() =>
    load().catch((error: unknown) => {
      throw new ChunkLoadError(error)
    }),
  )
}

export function reloadPage(): void {
  window.location.reload()
}
