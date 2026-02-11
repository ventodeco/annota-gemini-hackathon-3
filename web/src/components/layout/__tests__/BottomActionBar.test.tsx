import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import BottomActionBar from '../BottomActionBar'

describe('BottomActionBar', () => {
  it('renders explain button and disables it when disabled is true', () => {
    const onExplain = vi.fn()
    render(<BottomActionBar disabled onExplain={onExplain} />)

    const explainButton = screen.getByRole('button', { name: /explain this/i })
    expect(explainButton).toBeInTheDocument()
    expect(explainButton).toBeDisabled()
  })

  it('calls onExplain when explain button is clicked', async () => {
    const user = userEvent.setup()
    const onExplain = vi.fn()
    render(<BottomActionBar onExplain={onExplain} />)

    const explainButton = screen.getByRole('button', { name: /explain this/i })
    await user.click(explainButton)
    expect(onExplain).toHaveBeenCalledTimes(1)
  })
})
