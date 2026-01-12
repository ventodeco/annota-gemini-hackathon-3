import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createScan, getScan, annotate, getScanImageURL } from '../api'

describe('API Client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    global.fetch = vi.fn()
  })

  describe('createScan', () => {
    it('should create a scan successfully', async () => {
      const mockResponse = {
        scanID: 'test-scan-id',
        status: 'uploaded',
        createdAt: '2024-01-01T00:00:00Z',
      }

      ;(global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' })
      const result = await createScan(file)

      expect(result).toEqual(mockResponse)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/scans'),
        expect.objectContaining({
          method: 'POST',
          body: expect.any(FormData),
        })
      )
    })

    it('should handle errors', async () => {
      ;(global.fetch as any).mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: async () => 'Invalid image type',
      })

      const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' })
      await expect(createScan(file)).rejects.toThrow('Invalid image type')
    })
  })

  describe('getScan', () => {
    it('should get scan data successfully', async () => {
      const mockResponse = {
        scan: { id: 'test-id', status: 'ocr_done' },
        ocrResult: { id: 'ocr-id', rawText: 'test text' },
        status: 'ocr_done',
      }

      ;(global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await getScan('test-id')

      expect(result).toEqual(mockResponse)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/scans/test-id')
      )
    })

    it('should handle 404 errors', async () => {
      ;(global.fetch as any).mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: async () => 'Scan not found',
      })

      await expect(getScan('invalid-id')).rejects.toThrow('Scan not found')
    })
  })

  describe('annotate', () => {
    it('should create annotation successfully', async () => {
      const mockResponse = {
        id: 'ann-id',
        meaning: 'test meaning',
        usageExample: 'test example',
        whenToUse: 'test when',
        wordBreakdown: 'test breakdown',
        alternativeMeanings: 'test alt',
      }

      ;(global.fetch as any).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await annotate('scan-id', { selectedText: 'test' })

      expect(result).toEqual(mockResponse)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/scans/scan-id/annotate'),
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ selectedText: 'test' }),
        })
      )
    })

    it('should handle errors', async () => {
      ;(global.fetch as any).mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: async () => 'Invalid selection',
      })

      await expect(annotate('scan-id', { selectedText: '' })).rejects.toThrow(
        'Invalid selection'
      )
    })
  })

  describe('getScanImageURL', () => {
    it('should return correct image URL', () => {
      const url = getScanImageURL('test-id')
      expect(url).toContain('/api/scans/test-id/image')
    })
  })
})
