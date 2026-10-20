import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter } from 'react-router-dom'
import { ApiError } from '@/lib/api'
import WelcomePage from '../WelcomePage'

const uploadDocumentMock = vi.fn()

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return {
    ...actual,
    uploadDocument: (...args: unknown[]) => uploadDocumentMock(...args),
  }
})

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

  it('states which PDFs are supported next to the upload button', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /upload pdf/i })).toHaveAccessibleDescription(
      'PDFs need selectable text. For scanned PDFs, take a photo of each page instead.',
    )
  })

  it('shows the server message, not the raw response, when a PDF is rejected', async () => {
    uploadDocumentMock.mockRejectedValueOnce(
      new ApiError(422, JSON.stringify({ error: 'Unprocessable Entity', message: "This file couldn't be read as a PDF. Try another file." })),
    )
    renderPage()
    const input = document.querySelector('input[type="file"][accept="application/pdf"]')
    if (!(input instanceof HTMLInputElement)) throw new Error('missing PDF input')
    fireEvent.change(input, { target: { files: [new File(['%PDF'], 'bad.pdf', { type: 'application/pdf' })] } })

    expect(await screen.findByRole('alert')).toHaveTextContent("This file couldn't be read as a PDF. Try another file.")
    expect(screen.queryByText(/Unprocessable Entity/)).not.toBeInTheDocument()
    expect(input.value).toBe('')
  })

  it('has PDF file input', () => {
    renderPage()
    const input = document.querySelector('input[type="file"][accept="application/pdf"]') as HTMLInputElement
    expect(input).toBeInTheDocument()
  })
})