import { vi } from 'vitest'

export const mockApiClient = {
  createScan: vi.fn(),
  getScan: vi.fn(),
  annotate: vi.fn(),
}
