import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PageReader from '../PageReader'

describe('PageReader', () => {
  it('renders page text content', () => {
    render(
      <PageReader text="This is page content" currentPage={1} totalPages={5}
        onPrev={vi.fn()} onNext={vi.fn()} onTextSelect={vi.fn()} isLoading={false} />
    )
    expect(screen.getByText('This is page content')).toBeInTheDocument()
  })

  it('shows loading state', () => {
    render(
      <PageReader text="" currentPage={1} totalPages={5}
        onPrev={vi.fn()} onNext={vi.fn()} onTextSelect={vi.fn()} isLoading={true} />
    )
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('shows page indicator', () => {
    render(
      <PageReader text="content" currentPage={3} totalPages={10}
        onPrev={vi.fn()} onNext={vi.fn()} onTextSelect={vi.fn()} isLoading={false} />
    )
    expect(screen.getByText('3 / 10')).toBeInTheDocument()
  })
})
