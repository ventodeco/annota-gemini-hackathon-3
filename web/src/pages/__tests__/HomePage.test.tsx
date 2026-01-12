import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import HomePage from '../HomePage'

describe('HomePage', () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  it('should render upload form', () => {
    const { getAllByText } = render(<HomePage />, { wrapper })

    const uploadTexts = getAllByText('Upload Image')
    expect(uploadTexts.length).toBeGreaterThan(0)
    expect(uploadTexts[0]).toBeInTheDocument()
  })
})
