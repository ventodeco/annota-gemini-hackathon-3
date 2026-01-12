import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ErrorAlert from '../ErrorAlert'

describe('ErrorAlert', () => {
  it('should render error message', () => {
    render(
      <ErrorAlert title="Error Title" message="Error message content" />
    )

    expect(screen.getByText('Error Title')).toBeInTheDocument()
    expect(screen.getByText('Error message content')).toBeInTheDocument()
  })

  it('should call onRetry when retry button is clicked', async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()

    render(
      <ErrorAlert
        title="Error"
        message="Test error"
        onRetry={onRetry}
        retryLabel="Try Again"
      />
    )

    const button = screen.getByRole('button', { name: /try again/i })
    await user.click(button)

    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('should not show retry button when onRetry is not provided', () => {
    render(<ErrorAlert title="Error" message="Test error" />)

    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
