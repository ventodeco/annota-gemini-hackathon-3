import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import IntroPage from '../IntroPage'

const authState = { isSignedIn: false }

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => authState,
}))

function renderIntro() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<IntroPage />} />
        <Route path="/welcome" element={<p>Start screen</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('IntroPage', () => {
  beforeEach(() => {
    authState.isSignedIn = false
  })

  it('explains ANNOTA, supported files and limits to signed-out visitors', () => {
    renderIntro()

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/Read real Japanese/)
    expect(screen.getByText('Public beta')).toBeInTheDocument()
    const limits = screen.getByRole('region', { name: 'What works in the beta' })
    expect(limits).toHaveTextContent(/Text-based PDFs up to 10 MB/)
    expect(limits).toHaveTextContent(/JPEG, PNG or WebP up to 10 MB/)
    expect(limits).toHaveTextContent(/image-only PDFs are not supported yet/)
    expect(limits).toHaveTextContent(/Explanations are written in English/)
    expect(screen.getByRole('region', { name: 'Accounts and privacy' })).toHaveTextContent(/Clerk, with Google or email/)
  })

  it('links to sign-in, sign-up, legal pages and the read-only sample', () => {
    renderIntro()

    expect(screen.getByRole('link', { name: 'Create an account' })).toHaveAttribute('href', '/sign-up')
    for (const link of screen.getAllByRole('link', { name: 'Sign in' })) {
      expect(link).toHaveAttribute('href', '/login')
    }
    expect(screen.getAllByRole('link', { name: /Privacy/ })[0]).toHaveAttribute('href', '/privacy')
    expect(screen.getAllByRole('link', { name: /Terms/ })[0]).toHaveAttribute('href', '/terms')
    expect(screen.getByRole('link', { name: 'View the sample PDF' })).toHaveAttribute('href', '/samples/annota-sample.pdf')
  })

  it('sends signed-in visitors to the start screen', () => {
    authState.isSignedIn = true
    renderIntro()

    expect(screen.getByText('Start screen')).toBeInTheDocument()
  })
})
