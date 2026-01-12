import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import LoadingSpinner from '../LoadingSpinner'

describe('LoadingSpinner', () => {
  it('should render spinner', () => {
    const { container } = render(<LoadingSpinner />)

    const spinner = container.querySelector('[class*="animate-spin"]')
    expect(spinner).toBeInTheDocument()
  })

  it('should display custom message', () => {
    render(<LoadingSpinner message="Custom loading..." />)

    expect(screen.getByText('Custom loading...')).toBeInTheDocument()
  })

  it('should display default message', () => {
    render(<LoadingSpinner />)

    expect(screen.getByText('Processing...')).toBeInTheDocument()
  })
})
