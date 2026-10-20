import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthNotConfigured } from '../ClerkRoot'

describe('AuthNotConfigured', () => {
  it('explains the missing publishable key instead of rendering a blank page', () => {
    render(<AuthNotConfigured />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Sign-in is not configured' })).toBeInTheDocument()
    expect(screen.getByText('VITE_CLERK_PUBLISHABLE_KEY')).toBeInTheDocument()
  })
})
