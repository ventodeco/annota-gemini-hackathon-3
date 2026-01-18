import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ScanPage from '../ScanPage'

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useParams: () => ({ id: 'test-scan-id' }),
  }
})

vi.mock('@/lib/mockData', async () => {
  const actual = await vi.importActual('@/lib/mockData')
  return {
    ...actual,
    getMockScan: vi.fn(() => null),
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

  it('should show scan not found when mock data returns null', () => {
    const { getByText } = render(<ScanPage />, { wrapper })
    expect(getByText('Scan not found')).toBeInTheDocument()
  })
})
