import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter } from 'react-router-dom'
import WelcomePage from '../WelcomePage'

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => ({
    user: { email: 'test@example.com' },
  }),
}))

describe('WelcomePage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  function renderPage() {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })

    return render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <WelcomePage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
  }

  it('renders welcome message', () => {
    renderPage()
    expect(screen.getByText('Welcome to ANNOTA')).toBeInTheDocument()
  })

  it('shows user email when authenticated', () => {
    renderPage()
    expect(screen.getByText('Signed in as test@example.com')).toBeInTheDocument()
  })

  it('renders Take Photo button', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /take photo/i })).toBeInTheDocument()
  })

  it('renders Upload from Gallery button', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /upload from gallery/i })).toBeInTheDocument()
  })

  it('renders Upload PDF button', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /upload pdf/i })).toBeInTheDocument()
  })

  it('has image file input', () => {
    renderPage()
    const input = document.querySelector('input[type="file"][accept="image/*"]') as HTMLInputElement
    expect(input).toBeInTheDocument()
  })

  it('has PDF file input', () => {
    renderPage()
    const input = document.querySelector('input[type="file"][accept="application/pdf"]') as HTMLInputElement
    expect(input).toBeInTheDocument()
  })
})