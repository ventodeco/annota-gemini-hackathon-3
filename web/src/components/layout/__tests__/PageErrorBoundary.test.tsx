import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Link, Route, Routes, useParams } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { PageErrorBoundary } from '@/components/layout/PageErrorBoundary'

function ScanStub() {
  const { id } = useParams<{ id: string }>()
  if (id === '1') {
    throw new Error('scan 1 failed to render')
  }
  return <p>Scan {id}</p>
}

describe('PageErrorBoundary', () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleErrorSpy.mockRestore()
  })

  it('resets when navigating to another param of the same route', async () => {
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/scans/1']}>
        <Link to="/scans/2">Next scan</Link>
        <Routes>
          <Route
            path="/scans/:id"
            element={
              <PageErrorBoundary>
                <ScanStub />
              </PageErrorBoundary>
            }
          />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong')

    await user.click(screen.getByRole('link', { name: 'Next scan' }))

    expect(screen.getByText('Scan 2')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
