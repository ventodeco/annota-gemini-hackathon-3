import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { MemoryRouter } from 'react-router-dom'
import WelcomePage from '../WelcomePage'

const navigateMock = vi.fn()
const createScanMock = vi.fn()
const uploadDocumentMock = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => navigateMock,
  }
})

vi.mock('@/lib/api', () => ({
  createScan: (...args: unknown[]) => createScanMock(...args),
  uploadDocument: (...args: unknown[]) => uploadDocumentMock(...args),
}))

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => ({
    user: { email: 'test@example.com' },
    logout: vi.fn(),
  }),
}))

describe('WelcomePage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
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

  it('navigates to loading route after successful upload', async () => {
    createScanMock.mockResolvedValueOnce({ scanId: 42, imageUrl: '/uploads/42.jpg' })

    renderPage()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File(['fake'], 'scan.jpg', { type: 'image/jpeg' })

    await userEvent.upload(input, file)

    await waitFor(() => {
      expect(createScanMock).toHaveBeenCalledTimes(1)
      expect(navigateMock).toHaveBeenCalledWith('/loading/42')
    })
  })

  it('shows inline error for non-image files', async () => {
    const user = userEvent.setup({ applyAccept: false })
    renderPage()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File(['fake'], 'doc.txt', { type: 'text/plain' })

    await user.upload(input, file)

    expect(screen.getByText('Please select an image file')).toBeInTheDocument()
    expect(createScanMock).not.toHaveBeenCalled()
  })

  it('renders Upload PDF button', () => {
    renderPage()
    expect(screen.getByText('Upload PDF')).toBeInTheDocument()
  })

  it('PDF file input accepts application/pdf only', () => {
    renderPage()
    const pdfInput = document.querySelector('input[accept="application/pdf"]') as HTMLInputElement
    expect(pdfInput).toBeInTheDocument()
  })

  it('navigates to document page after successful PDF upload', async () => {
    uploadDocumentMock.mockResolvedValueOnce({ documentId: 7, pageCount: 5, filename: 'test.pdf' })

    renderPage()
    const pdfInput = document.querySelector('input[accept="application/pdf"]') as HTMLInputElement
    const file = new File(['fake-pdf'], 'test.pdf', { type: 'application/pdf' })

    await userEvent.upload(pdfInput, file)

    await waitFor(() => {
      expect(uploadDocumentMock).toHaveBeenCalledTimes(1)
      expect(navigateMock).toHaveBeenCalledWith('/documents/7')
    })
  })
})
