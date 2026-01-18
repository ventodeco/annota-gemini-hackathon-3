import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { BookmarkList } from '../BookmarkList'
import type { Annotation } from '@/lib/types'

const mockAnnotations: Annotation[] = [
  {
    id: 'ann-1',
    scanID: 'scan-1',
    ocrResultID: 'ocr-1',
    selectedText: 'テスト',
    meaning: 'Test',
    usageExample: 'テストです',
    whenToUse: 'Testing',
    wordBreakdown: 'Test',
    alternativeMeanings: 'Exam',
    model: 'gemini-1.5-flash',
    promptVersion: '1.0',
    createdAt: '2026-01-10T10:00:00.000Z',
    context: 'Test context',
  },
  {
    id: 'ann-2',
    scanID: 'scan-2',
    ocrResultID: 'ocr-2',
    selectedText: '取消し',
    meaning: 'Cancellation',
    usageExample: '取消しです',
    whenToUse: 'Cancelling',
    wordBreakdown: 'Cancel',
    alternativeMeanings: 'Revocation',
    model: 'gemini-1.5-flash',
    promptVersion: '1.0',
    createdAt: '2026-01-12T10:00:00.000Z',
    context: 'Cancel context',
  },
]

describe('BookmarkList', () => {
  const onDelete = vi.fn()

  it('should render all annotations sorted by date (newest first)', () => {
    render(
      <BrowserRouter>
        <BookmarkList annotations={mockAnnotations} onDelete={onDelete} />
      </BrowserRouter>
    )
    const cards = screen.getAllByText(/取消し|テスト/)
    expect(cards).toHaveLength(2)
  })

  it('should pass onDelete callback to each card', () => {
    render(
      <BrowserRouter>
        <BookmarkList annotations={mockAnnotations} onDelete={onDelete} />
      </BrowserRouter>
    )
    const deleteButtons = screen.getAllByRole('button', { name: /delete annotation/i })
    expect(deleteButtons).toHaveLength(2)
  })
})
