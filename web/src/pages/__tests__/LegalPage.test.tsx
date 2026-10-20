import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import LegalPage from '../LegalPage'

describe('LegalPage', () => {
  it('describes the configured processors without retention guarantees', () => {
    render(<LegalPage kind="privacy" />)

    expect(screen.getByRole('heading', { level: 1, name: 'Privacy Policy' })).toBeInTheDocument()
    expect(screen.getByText(/Sign-in is provided by Clerk/i)).toBeInTheDocument()
    const processing = screen.getByText(/UCloud AstraFlow \(ModelVerse\) or\s+OpenRouter/i)
    expect(processing).toHaveTextContent(/does not promise zero retention or a particular processing region/i)
    expect(screen.queryByText(/stores .*pronunciation requests/i)).not.toBeInTheDocument()
  })

  it('explains deletion, retained annotations and backups', () => {
    render(<LegalPage kind="privacy" />)

    expect(screen.getByRole('heading', { level: 2, name: 'Deleting your data' })).toBeInTheDocument()
    const deletion = screen.getByText(/Deleting a scan removes its image and text/i)
    expect(deletion).toHaveTextContent(/Annotations you saved are separate and stay until you\s+delete them or your account/i)
    expect(deletion).toHaveTextContent(/Deleting your account removes your files, records and\s+Clerk sign-in/i)
    expect(deletion).toHaveTextContent(/Backups made before your deletion/i)
  })

  it('renders terms with upload rights and paid-use notice', () => {
    render(<LegalPage kind="terms" />)

    expect(screen.getByRole('heading', { name: 'Terms of Service' })).toBeInTheDocument()
    expect(screen.getByText(/rights to upload/i)).toBeInTheDocument()
    expect(screen.getByText(/usage limits/i)).toBeInTheDocument()
  })
})
