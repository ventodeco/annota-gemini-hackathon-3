import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ErrorBoundary } from '@/components/layout/ErrorBoundary'
import { ChunkLoadError, reloadPage } from '@/lib/lazyRoute'

vi.mock('@/lib/lazyRoute', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/lazyRoute')>()),
  reloadPage: vi.fn(),
}))

function ChunkFailureChild(): never {
  throw new ChunkLoadError(new TypeError('Failed to fetch dynamically imported module'))
}

function ThrowingChild() {
  throw new Error('render failed')
}

function createFlakyChild() {
  const state = { shouldThrow: true }
  function FlakyChild() {
    if (state.shouldThrow) {
      throw new Error('transient render failure')
    }
    return <p>Recovered content</p>
  }
  return { state, FlakyChild }
}

describe('ErrorBoundary', () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleErrorSpy.mockRestore()
  })

  it('renders a retry fallback when a child throws during render', () => {
    render(
      <ErrorBoundary>
        <ThrowingChild />
      </ErrorBoundary>,
    )

    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('moves focus to the fallback heading and offers a way home', () => {
    render(
      <ErrorBoundary>
        <ThrowingChild />
      </ErrorBoundary>,
    )

    expect(screen.getByRole('heading', { name: 'Something went wrong' })).toHaveFocus()
    expect(screen.getByRole('link', { name: /go to home/i })).toHaveAttribute('href', '/')
  })

  it('reloads the page when a route chunk failed to load', async () => {
    const user = userEvent.setup()
    const onReset = vi.fn()

    render(
      <ErrorBoundary onReset={onReset}>
        <ChunkFailureChild />
      </ErrorBoundary>,
    )

    await user.click(screen.getByRole('button', { name: /try again/i }))

    expect(reloadPage).toHaveBeenCalledTimes(1)
    expect(onReset).not.toHaveBeenCalled()
  })

  it('runs the reset callback from the retry action', async () => {
    const user = userEvent.setup()
    const onReset = vi.fn()

    render(
      <ErrorBoundary onReset={onReset}>
        <ThrowingChild />
      </ErrorBoundary>,
    )

    await user.click(screen.getByRole('button', { name: /try again/i }))

    expect(onReset).toHaveBeenCalledTimes(1)
  })

  it('renders the children again after retry when the failure was transient', async () => {
    const user = userEvent.setup()
    const { state, FlakyChild } = createFlakyChild()

    render(
      <ErrorBoundary>
        <FlakyChild />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()

    state.shouldThrow = false
    await user.click(screen.getByRole('button', { name: /try again/i }))

    expect(screen.getByText('Recovered content')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('clears the fallback when the reset key changes', () => {
    const { state, FlakyChild } = createFlakyChild()

    const { rerender } = render(
      <ErrorBoundary resetKey="/scans/1">
        <FlakyChild />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()

    state.shouldThrow = false
    rerender(
      <ErrorBoundary resetKey="/scans/2">
        <FlakyChild />
      </ErrorBoundary>,
    )

    expect(screen.getByText('Recovered content')).toBeInTheDocument()
  })

  it('renders a custom fallback that can reset the boundary', async () => {
    const user = userEvent.setup()
    const { state, FlakyChild } = createFlakyChild()

    render(
      <ErrorBoundary
        fallback={(reset) => (
          <button type="button" onClick={reset}>
            Reload panel
          </button>
        )}
      >
        <FlakyChild />
      </ErrorBoundary>,
    )
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument()

    state.shouldThrow = false
    await user.click(screen.getByRole('button', { name: 'Reload panel' }))

    expect(screen.getByText('Recovered content')).toBeInTheDocument()
  })
})
