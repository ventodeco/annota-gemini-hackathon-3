import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import IntroPage from '../IntroPage'
import { AuthPageLayout } from '../LoginPage'

const authState = { isSignedIn: false }

vi.mock('@/contexts/useAuth', () => ({
  useAuth: () => authState,
}))

function renderIntro(entry = '/') {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/" element={<IntroPage />} />
        <Route path="/welcome" element={<p>Start screen</p>} />
        <Route path="/documents/:id" element={<p>Reader</p>} />
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

  it('keeps a sign-in destination through the About detour', () => {
    const destination = encodeURIComponent('/documents/123?page=2')
    render(
      <MemoryRouter initialEntries={[`/login?redirect_url=${destination}`]}>
        <AuthPageLayout>
          <p>Sign-in form</p>
        </AuthPageLayout>
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: 'About ANNOTA' })).toHaveAttribute('href', `/?redirect_url=${destination}`)
  })

  it('passes the destination on to sign-in and sign-up, and ignores unsafe ones', () => {
    const destination = encodeURIComponent('/documents/123?page=2')
    const { unmount } = renderIntro(`/?redirect_url=${destination}`)
    expect(screen.getByRole('link', { name: 'Create an account' })).toHaveAttribute('href', `/sign-up?redirect_url=${destination}`)
    for (const link of screen.getAllByRole('link', { name: 'Sign in' })) {
      expect(link).toHaveAttribute('href', `/login?redirect_url=${destination}`)
    }
    unmount()

    renderIntro(`/?redirect_url=${encodeURIComponent('https://evil.example/')}`)
    expect(screen.getByRole('link', { name: 'Create an account' })).toHaveAttribute('href', '/sign-up')
  })

  it('sends signed-in visitors to their preserved destination', () => {
    authState.isSignedIn = true
    renderIntro(`/?redirect_url=${encodeURIComponent('/documents/123?page=2')}`)

    expect(screen.getByText('Reader')).toBeInTheDocument()
  })
})
