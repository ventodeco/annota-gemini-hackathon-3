import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import TextPreview from '../TextPreview'

vi.mock('@/lib/api')

describe('TextPreview', () => {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  it('should render text', () => {
    render(<TextPreview text="Test text content" scanID="test-id" />, {
      wrapper,
    })

    expect(screen.getByText('Test text content')).toBeInTheDocument()
  })

  it.skip('should show annotation form when text is selected', async () => {
    vi.mocked(api.annotate).mockResolvedValue({
      id: 'ann-id',
      scanID: 'test-id',
      ocrResultID: 'ocr-id',
      selectedText: 'Test text',
      meaning: 'test meaning',
      usageExample: 'test example',
      whenToUse: 'test when',
      wordBreakdown: 'test breakdown',
      alternativeMeanings: 'test alt',
      model: 'gemini-flash',
      promptVersion: '1.0',
      createdAt: '2024-01-01T00:00:00Z',
    })

    render(<TextPreview text="Test text content" scanID="test-id" />, {
      wrapper,
    })

    const textElement = screen.getByText('Test text content')
    
    const range = document.createRange()
    range.selectNodeContents(textElement)
    const selection = window.getSelection()
    selection?.removeAllRanges()
    selection?.addRange(range)

    const proseDiv = textElement.parentElement
    if (proseDiv) {
      const mouseEvent = new MouseEvent('mouseup', { bubbles: true })
      proseDiv.dispatchEvent(mouseEvent)
    }

    await waitFor(() => {
      expect(screen.queryByText(/Selected Text/i)).toBeInTheDocument()
    }, { timeout: 2000 })
  })
})
