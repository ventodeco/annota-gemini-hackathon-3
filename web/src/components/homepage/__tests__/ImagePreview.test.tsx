import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import ImagePreview from '../ImagePreview'

describe('ImagePreview', () => {
  it('should render image with src', () => {
    render(<ImagePreview src="test-image.jpg" />)

    const img = screen.getByRole('img')
    expect(img).toHaveAttribute('src', 'test-image.jpg')
  })

  it('should use custom alt text', () => {
    render(<ImagePreview src="test.jpg" alt="Custom preview" />)

    const img = screen.getByAltText('Custom preview')
    expect(img).toBeInTheDocument()
  })
})
