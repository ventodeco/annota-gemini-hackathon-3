import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import UploadForm from '../UploadForm'
import * as api from '@/lib/api'

vi.mock('@/lib/api')
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => vi.fn(),
  }
})

describe('UploadForm', () => {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  it('should render file input', () => {
    render(<UploadForm />, { wrapper })

    const input = document.querySelector('input[type="file"]')
    expect(input).toBeInTheDocument()
  })

  it('should disable submit button when no file selected', () => {
    render(<UploadForm />, { wrapper })

    const button = screen.getByRole('button', { name: /upload/i })
    expect(button).toBeDisabled()
  })

  it('should enable submit button when file is selected', async () => {
    const user = userEvent.setup()
    render(<UploadForm />, { wrapper })

    const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement

    if (input) {
      await user.upload(input, file)

      const button = screen.getByRole('button', { name: /upload/i })
      await waitFor(() => {
        expect(button).not.toBeDisabled()
      })
    }
  })
})
