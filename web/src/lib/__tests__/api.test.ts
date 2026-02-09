import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest'
import { createScan, getScan, analyzeText, getScanImageUrl, getAnnotations } from '../api'

describe('API Client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    global.fetch = vi.fn() as Mock
  })

  describe('createScan', () => {
    it('should create a scan successfully', async () => {
      const mockResponse = {
        scanID: 'test-scan-id',
        status: 'uploaded',
        createdAt: '2024-01-01T00:00:00Z',
      }

      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' })
      const result = await createScan(file)

      expect(result).toEqual(mockResponse)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/scans'),
        expect.objectContaining({
          method: 'POST',
          body: expect.any(FormData),
        })
      )
    })

    it('should handle errors', async () => {
      ;(global.fetch as Mock).mockResolvedValueOnce({
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
        scan: { id: 1, status: 'ocr_done' },
        ocrResult: { id: 'ocr-id', rawText: 'test text' },
        status: 'ocr_done',
      }

      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await getScan(1)

      expect(result).toEqual(mockResponse)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/scans/1'),
        expect.objectContaining({
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
        })
      )
    })

    it('should handle 404 errors', async () => {
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: async () => 'Scan not found',
      })

      await expect(getScan(999)).rejects.toThrow('Scan not found')
    })
  })

  describe('analyzeText', () => {
    it('should analyze text successfully', async () => {
      const mockResponse = {
        meaning: 'test meaning',
        usageExample: 'test example',
        whenToUse: 'test when',
        wordBreakdown: 'test breakdown',
        alternativeMeanings: 'test alt',
      }

      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await analyzeText({ textToAnalyze: 'test', context: 'test context' })

      expect(result).toEqual(mockResponse)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/ai/analyze'),
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ textToAnalyze: 'test', context: 'test context' }),
        })
      )
    })

    it('should handle errors', async () => {
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: async () => 'Invalid selection',
      })

      await expect(analyzeText({ textToAnalyze: '', context: 'test' })).rejects.toThrow(
        'Invalid selection'
      )
    })
  })

  describe('getScanImageUrl', () => {
    it('should return correct image URL for relative path', () => {
      const url = getScanImageUrl('/v1/scans/test-id/image')
      expect(url).toContain('/v1/scans/test-id/image')
    })

    it('should return original URL for absolute URLs', () => {
      const url = getScanImageUrl('http://example.com/image.jpg')
      expect(url).toBe('http://example.com/image.jpg')
    })
  })

  describe('getAnnotations', () => {
    it('includes scanId in query string when provided', async () => {
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [], meta: { currentPage: 1, pageSize: 20 } }),
      })

      await getAnnotations(1, 20, 5)

      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/annotations?page=1&size=20&scanId=5'),
        expect.objectContaining({
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    })
  })
})
