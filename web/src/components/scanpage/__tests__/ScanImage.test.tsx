import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import ScanImage from '../ScanImage'

describe('ScanImage', () => {
  it('should render image with correct src', () => {
    render(<ScanImage scanID="test-scan-id" />)

    const img = screen.getByRole('img')
    expect(img).toHaveAttribute('src', expect.stringContaining('/api/scans/test-scan-id/image'))
  })

  it('should use custom alt text', () => {
    render(<ScanImage scanID="test-id" alt="Custom alt" />)

    const img = screen.getByAltText('Custom alt')
    expect(img).toBeInTheDocument()
  })
})
