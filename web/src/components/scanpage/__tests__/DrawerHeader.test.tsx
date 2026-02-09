import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DrawerHeader } from '../DrawerHeader'

describe('DrawerHeader', () => {
  it('renders version text next to annotation title', () => {
    render(
      <DrawerHeader
        onClose={vi.fn()}
        version={1}
        maxVersions={2}
      />,
    )

    expect(screen.getByText('Annotation')).toBeInTheDocument()
    expect(screen.getByText('Version 1/2')).toBeInTheDocument()
  })

  it('calls collapse when title region is clicked', async () => {
    const user = userEvent.setup()
    const onCollapse = vi.fn()

    render(
      <DrawerHeader
        onClose={vi.fn()}
        onCollapse={onCollapse}
        version={2}
        maxVersions={2}
      />,
    )

    await user.click(screen.getByText('Annotation'))

    expect(onCollapse).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Version 2/2')).toBeInTheDocument()
  })
})
