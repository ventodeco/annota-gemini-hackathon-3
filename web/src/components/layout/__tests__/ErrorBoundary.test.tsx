import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ErrorBoundary } from '@/components/layout/ErrorBoundary'

function ThrowingChild() {
  throw new Error('render failed')
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
})
