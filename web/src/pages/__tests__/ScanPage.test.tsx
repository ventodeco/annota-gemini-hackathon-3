import { describe, it, expect, vi } from 'vitest'
import { render, waitFor } from '@testing-library/react'
import React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ScanPage from '../ScanPage'
import * as api from '@/lib/api'

vi.mock('@/lib/api')
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useParams: () => ({ id: 'test-scan-id' }),
  }
})

describe('ScanPage', () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  it('should show loading state initially', () => {
    vi.mocked(api.getScan).mockImplementation(
      () => new Promise(() => {})
    )

    const { getByText } = render(<ScanPage />, { wrapper })

    expect(getByText(/Loading scan/i)).toBeInTheDocument()
  })

  it.skip('should display OCR result when ready', async () => {
    const mockData = {
      scan: { 
        id: 'test-id', 
        sessionID: 'session-1',
        status: 'ocr_done',
        source: 'test.jpg',
        createdAt: '2024-01-01T00:00:00Z',
      },
      ocrResult: { 
        id: 'ocr-id', 
        scanID: 'test-id',
        rawText: 'test text',
        model: 'gemini-flash',
        promptVersion: '1.0',
        createdAt: '2024-01-01T00:00:00Z',
      },
      status: 'ocr_done',
    }

    vi.mocked(api.getScan).mockResolvedValue(mockData)

    const { getByText, findByText } = render(<ScanPage />, { wrapper })

    await findByText('Extracted Text')
    await findByText('test text')
  })
})
