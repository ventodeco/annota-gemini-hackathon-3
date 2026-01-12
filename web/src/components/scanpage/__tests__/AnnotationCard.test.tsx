import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnnotationCard from '../AnnotationCard'
import type { Annotation } from '@/lib/types'

describe('AnnotationCard', () => {
  const mockAnnotation: Annotation = {
    id: 'ann-1',
    scanID: 'scan-1',
    ocrResultID: 'ocr-1',
    selectedText: 'テスト',
    meaning: 'Test meaning',
    usageExample: 'Usage example',
    whenToUse: 'When to use',
    wordBreakdown: 'Word breakdown',
    alternativeMeanings: 'Alternative meanings',
    model: 'gemini-2.5-flash',
    promptVersion: '1.0',
    createdAt: '2024-01-01T00:00:00Z',
  }

  it('should render annotation fields', () => {
    render(<AnnotationCard annotation={mockAnnotation} />)

    expect(screen.getByText('Annotation')).toBeInTheDocument()
    expect(screen.getByText(/テスト/)).toBeInTheDocument()
    expect(screen.getByText('Test meaning')).toBeInTheDocument()
    expect(screen.getByText('Usage example')).toBeInTheDocument()
  })

  it('should render all required fields', () => {
    render(<AnnotationCard annotation={mockAnnotation} />)

    expect(screen.getByText('Meaning')).toBeInTheDocument()
    expect(screen.getByText('Usage Example')).toBeInTheDocument()
    expect(screen.getByText('When to Use')).toBeInTheDocument()
    expect(screen.getByText('Word Breakdown')).toBeInTheDocument()
    expect(screen.getByText('Alternative Meanings')).toBeInTheDocument()
  })
})
