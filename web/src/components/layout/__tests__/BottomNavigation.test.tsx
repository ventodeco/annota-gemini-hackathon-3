import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import BottomNavigation from '../BottomNavigation'

describe('BottomNavigation', () => {
  it('links the newspaper icon to OCR history route', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/welcome']}>
        <BottomNavigation />
      </MemoryRouter>,
    )

    const historyLink = container.querySelector('a[href="/scans-history"]')
    expect(historyLink).toBeInTheDocument()
  })

  it('marks OCR history as active when current route is /scans-history', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/scans-history']}>
        <BottomNavigation />
      </MemoryRouter>,
    )

    const historyLink = container.querySelector('a[href="/scans-history"]')
    expect(historyLink?.className).toContain('bg-[#0F172A]')
  })
})
