import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter } from 'react-router-dom'
import { ApiError } from '@/lib/api'
import { FIRST_USE_GUIDE_STORAGE_KEY } from '@/components/homepage/FirstUseGuide'
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
  it('uploads a private copy of the bundled sample through the normal PDF upload', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Blob(['%PDF-sample']), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    uploadDocumentMock.mockReturnValueOnce(new Promise(() => undefined))
    try {
      renderPage()
      fireEvent.click(screen.getByRole('button', { name: /try the sample/i }))

      await waitFor(() => expect(uploadDocumentMock).toHaveBeenCalledTimes(1))
      expect(fetchMock).toHaveBeenCalledWith('/samples/annota-sample.pdf')
      const uploaded: unknown = uploadDocumentMock.mock.calls[0][0]
      if (!(uploaded instanceof File)) throw new Error('sample was not uploaded as a File')
      expect(uploaded.name).toBe('annota-sample.pdf')
      expect(uploaded.type).toBe('application/pdf')
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('reports a sample that cannot be downloaded without uploading anything', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')))
    uploadDocumentMock.mockClear()
    try {
      renderPage()
      fireEvent.click(screen.getByRole('button', { name: /try the sample/i }))

      expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the sample.')
      expect(uploadDocumentMock).not.toHaveBeenCalled()
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('shows a skippable first-use guide and remembers when it is hidden', () => {
    vi.mocked(localStorage.getItem).mockReturnValue(null)
    renderPage()

    const guide = screen.getByRole('region', { name: 'How ANNOTA works' })
    expect(guide).toHaveTextContent(/Explain this/)
    fireEvent.click(screen.getByRole('button', { name: 'Hide guide' }))

    expect(localStorage.setItem).toHaveBeenCalledWith(FIRST_USE_GUIDE_STORAGE_KEY, 'hidden')
    expect(screen.queryByRole('region', { name: 'How ANNOTA works' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show how ANNOTA works' })).toHaveFocus()
  })

  it('keeps the guide hidden for returning users who skipped it', () => {
    vi.mocked(localStorage.getItem).mockImplementation((key) => (key === FIRST_USE_GUIDE_STORAGE_KEY ? 'hidden' : null))
    renderPage()

    expect(screen.queryByRole('region', { name: 'How ANNOTA works' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Show how ANNOTA works' }))
    expect(screen.getByRole('region', { name: 'How ANNOTA works' })).toBeInTheDocument()
    expect(localStorage.removeItem).toHaveBeenCalledWith(FIRST_USE_GUIDE_STORAGE_KEY)
  })
})
