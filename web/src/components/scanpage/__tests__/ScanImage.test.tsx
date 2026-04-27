import { beforeEach, describe, it, expect, vi, type Mock } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import ScanImage from '../ScanImage'
import { getScanImageBlob } from '@/lib/api'

vi.mock('@/lib/api', () => ({
  getScanImageBlob: vi.fn(),
}))

describe('ScanImage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    URL.createObjectURL = vi.fn(() => 'blob:private-image')
    URL.revokeObjectURL = vi.fn()
    ;(getScanImageBlob as Mock).mockResolvedValue(new Blob(['image'], { type: 'image/jpeg' }))
  })

  it('should fetch private image and render blob src', async () => {
    render(<ScanImage imageUrl="/v1/scans/test-scan-id/image" />)

    const img = await screen.findByRole('img')
    expect(getScanImageBlob).toHaveBeenCalledWith('/v1/scans/test-scan-id/image')
    expect(img).toHaveAttribute('src', 'blob:private-image')
  })

  it('should use custom alt text', async () => {
    render(<ScanImage imageUrl="/v1/scans/test-id/image" alt="Custom alt" />)

    const img = await screen.findByAltText('Custom alt')
    expect(img).toBeInTheDocument()
  })

  it('should return null when imageUrl is undefined', () => {
    const { container } = render(<ScanImage imageUrl={undefined} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('should hide the previous blob when imageUrl is removed', async () => {
    const { container, rerender } = render(<ScanImage imageUrl="/v1/scans/test-id/image" />)
    await screen.findByRole('img')

    rerender(<ScanImage imageUrl={undefined} />)

    expect(container).toBeEmptyDOMElement()
  })

  it('should revoke blob URLs on cleanup', async () => {
    const { unmount } = render(<ScanImage imageUrl="/v1/scans/test-id/image" />)
    await waitFor(() => expect(URL.createObjectURL).toHaveBeenCalled())

    unmount()

    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:private-image')
  })
})
