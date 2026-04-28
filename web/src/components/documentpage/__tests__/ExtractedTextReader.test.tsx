import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import ExtractedTextReader from '../ExtractedTextReader'

const mockOnTextSelect = vi.fn()
const mockOnPageChange = vi.fn()

function renderReader(options: {
  pageText?: string
  pageNumber?: number
  totalPages?: number
  isLoading?: boolean
} = {}) {
  return render(
    <ExtractedTextReader
      pageText={options.pageText ?? 'お母さん、ちょっと来て！'}
      pageNumber={options.pageNumber ?? 2}
      totalPages={4}
      isLoading={options.isLoading ?? false}
      onTextSelect={mockOnTextSelect}
      onPageChange={mockOnPageChange}
    />
  )
}

describe('ExtractedTextReader', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.getSelection()?.removeAllRanges()
  })

  it('shows a loading state while extracted text is loading', () => {
    renderReader({ isLoading: true })

    expect(screen.getByRole('status', { name: /loading reader text/i })).toBeInTheDocument()
  })

  it('shows an empty state when the page has no extracted text', () => {
    renderReader({ pageText: '   ' })

    expect(screen.getByText(/No selectable text was found on this page/i)).toBeInTheDocument()
    expect(screen.getByText(/Try PDF View or upload a text-based PDF/i)).toBeInTheDocument()
  })

  it('reports selected text from the reader content on mouse up', () => {
    renderReader({ pageText: '一行目\n二行目' })

    const readerText = screen.getByTestId('extracted-text-content')
    const selectedTextNode = readerText.firstChild
    const selection = window.getSelection()
    if (!selectedTextNode || !selection) {
      throw new Error('Expected selection APIs to be available')
    }
    const range = document.createRange()
    range.selectNodeContents(selectedTextNode)
    selection.removeAllRanges()
    selection.addRange(range)

    fireEvent.mouseUp(readerText)

    expect(mockOnTextSelect).toHaveBeenCalledWith('一行目\n二行目')
  })

  it('reports selected text from the reader content on document selection change', () => {
    renderReader({ pageText: '選択できる文章' })

    const readerText = screen.getByTestId('extracted-text-content')
    const selectedTextNode = readerText.firstChild
    const selection = window.getSelection()
    if (!selectedTextNode || !selection) {
      throw new Error('Expected selection APIs to be available')
    }
    const range = document.createRange()
    range.selectNodeContents(selectedTextNode)
    selection.removeAllRanges()
    selection.addRange(range)

    fireEvent(document, new Event('selectionchange'))

    expect(mockOnTextSelect).toHaveBeenCalledWith('選択できる文章')
  })

  it('disables previous and next controls at page boundaries', () => {
    const { rerender } = render(
      <ExtractedTextReader
        pageText="先頭ページ"
        pageNumber={1}
        totalPages={2}
        isLoading={false}
        onTextSelect={mockOnTextSelect}
        onPageChange={mockOnPageChange}
      />
    )

    expect(screen.getByRole('button', { name: /previous page/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /next page/i })).not.toBeDisabled()

    rerender(
      <ExtractedTextReader
        pageText="最終ページ"
        pageNumber={2}
        totalPages={2}
        isLoading={false}
        onTextSelect={mockOnTextSelect}
        onPageChange={mockOnPageChange}
      />
    )

    expect(screen.getByRole('button', { name: /previous page/i })).not.toBeDisabled()
    expect(screen.getByRole('button', { name: /next page/i })).toBeDisabled()
  })

  it('navigates pages through navigation page-change events', () => {
    renderReader({ pageNumber: 2, totalPages: 4 })

    fireEvent.click(screen.getByRole('button', { name: /previous page/i }))
    fireEvent.click(screen.getByRole('button', { name: /next page/i }))

    expect(mockOnPageChange).toHaveBeenNthCalledWith(1, 1, { source: 'navigation' })
    expect(mockOnPageChange).toHaveBeenNthCalledWith(2, 3, { source: 'navigation' })
  })
})
