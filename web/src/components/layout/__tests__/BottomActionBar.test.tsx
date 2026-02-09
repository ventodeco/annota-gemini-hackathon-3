import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import BottomActionBar from '../BottomActionBar'

describe('BottomActionBar', () => {
  it('keeps bookmark enabled when explain is disabled', async () => {
    const user = userEvent.setup()
    const onBookmark = vi.fn()

    render(<BottomActionBar disabled onBookmark={onBookmark} />)

    const explainButton = screen.getByRole('button', { name: /explain this/i })
    const bookmarkButton = screen.getByRole('button', { name: /bookmark/i })

    expect(explainButton).toBeDisabled()
    expect(bookmarkButton).toBeEnabled()

    await user.click(bookmarkButton)
    expect(onBookmark).toHaveBeenCalledTimes(1)
  })
})
