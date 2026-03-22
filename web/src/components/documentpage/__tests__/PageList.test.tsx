import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import PageList from '../PageList'

describe('PageList', () => {
  it('renders correct number of page items', () => {
    render(<PageList pageCount={5} onSelectPage={vi.fn()} />)
    for (let i = 1; i <= 5; i++) {
      expect(screen.getByText(`Page ${i}`)).toBeInTheDocument()
    }
  })

  it('calls onSelectPage when page item clicked', async () => {
    const onSelectPage = vi.fn()
    render(<PageList pageCount={3} onSelectPage={onSelectPage} />)
    await userEvent.click(screen.getByText('Page 2'))
    expect(onSelectPage).toHaveBeenCalledWith(2)
  })
})
