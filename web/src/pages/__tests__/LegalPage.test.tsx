import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import LegalPage from '../LegalPage'

describe('LegalPage', () => {
  it('renders privacy policy AI processing and deletion guidance', () => {
    render(<LegalPage kind="privacy" />)

    expect(screen.getByRole('heading', { name: 'Privacy Policy' })).toBeInTheDocument()
    expect(screen.getByText(/Google Gemini/i)).toBeInTheDocument()
    expect(screen.getByText(/delete your account/i)).toBeInTheDocument()
  })

  it('renders terms with upload rights and paid-use notice', () => {
    render(<LegalPage kind="terms" />)

    expect(screen.getByRole('heading', { name: 'Terms of Service' })).toBeInTheDocument()
    expect(screen.getByText(/rights to upload/i)).toBeInTheDocument()
    expect(screen.getByText(/usage limits/i)).toBeInTheDocument()
  })
})
