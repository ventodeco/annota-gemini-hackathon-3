import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import PageNavigator from '../PageNavigator'

describe('PageNavigator', () => {
  it('shows page indicator', () => {
    render(<PageNavigator currentPage={3} totalPages={10} onPrev={vi.fn()} onNext={vi.fn()} />)
    expect(screen.getByText('3 / 10')).toBeInTheDocument()
  })

  it('disables previous button on first page', () => {
    render(<PageNavigator currentPage={1} totalPages={10} onPrev={vi.fn()} onNext={vi.fn()} />)
    expect(screen.getByLabelText('Previous page')).toBeDisabled()
  })

  it('disables next button on last page', () => {
    render(<PageNavigator currentPage={10} totalPages={10} onPrev={vi.fn()} onNext={vi.fn()} />)
    expect(screen.getByLabelText('Next page')).toBeDisabled()
  })

  it('calls onPrev when previous clicked', async () => {
    const onPrev = vi.fn()
    render(<PageNavigator currentPage={5} totalPages={10} onPrev={onPrev} onNext={vi.fn()} />)
    await userEvent.click(screen.getByLabelText('Previous page'))
    expect(onPrev).toHaveBeenCalledTimes(1)
  })

  it('calls onNext when next clicked', async () => {
    const onNext = vi.fn()
    render(<PageNavigator currentPage={5} totalPages={10} onPrev={vi.fn()} onNext={onNext} />)
    await userEvent.click(screen.getByLabelText('Next page'))
    expect(onNext).toHaveBeenCalledTimes(1)
  })
})
