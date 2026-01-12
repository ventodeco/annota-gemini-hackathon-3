import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import HomePage from '@/pages/HomePage'
import * as api from '@/lib/api'

vi.mock('@/lib/api')

describe('Integration Tests', () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  it.skip('should handle complete upload flow', async () => {
    const mockCreateResponse = {
      scanID: 'test-scan-id',
      status: 'uploaded',
      createdAt: '2024-01-01T00:00:00Z',
    }

    vi.mocked(api.createScan).mockResolvedValueOnce(mockCreateResponse)

    render(<HomePage />, { wrapper })

    const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement

    if (input) {
      const user = userEvent.setup()
      await user.upload(input, file)

      const button = screen.getByRole('button', { name: /upload/i })
      await user.click(button)

      await waitFor(() => {
        expect(api.createScan).toHaveBeenCalled()
        const calls = vi.mocked(api.createScan).mock.calls
        expect(calls.length).toBeGreaterThan(0)
        expect(calls[0][0]).toBeInstanceOf(File)
      })
    }
  })
})
