import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest'
import { getToken } from '@clerk/react'
import { ApiError, createScan, deleteAccount, getScan, getUserProfile, analyzeText, getScanImageBlob, getScanImageUrl, getAnnotations, synthesizeSpeech, trackEvent } from '../api'
import { setUnauthorizedHandler } from '../authSession'

vi.mock('@clerk/react', () => ({
  getToken: vi.fn(),
}))

describe('API Client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    global.fetch = vi.fn() as Mock
    vi.mocked(getToken).mockResolvedValue(null)
    setUnauthorizedHandler(null)
  })

  describe('session token', () => {
    it('sends the current Clerk session token as a Bearer header only', async () => {
      vi.mocked(getToken).mockResolvedValue('clerk-session-jwt')
      vi.mocked(global.fetch).mockResolvedValueOnce(Response.json({ preferredLanguage: 'EN' }))

      await getUserProfile()

      const [, init] = vi.mocked(global.fetch).mock.calls[0]
      expect(init?.headers).toEqual(
        expect.objectContaining({ Authorization: 'Bearer clerk-session-jwt' }),
      )
      expect(init?.headers).not.toHaveProperty('x-token')
      expect(window.localStorage.getItem).not.toHaveBeenCalled()
    })

    it('omits Authorization when signed out', async () => {
      vi.mocked(global.fetch).mockResolvedValueOnce(Response.json({ preferredLanguage: 'EN' }))

      await getUserProfile()

      const [, init] = vi.mocked(global.fetch).mock.calls[0]
      expect(init?.headers).not.toHaveProperty('Authorization')
    })

    it('reports 401 responses to the unauthorized handler with the server message', async () => {
      const handler = vi.fn()
      setUnauthorizedHandler(handler)
      vi.mocked(global.fetch).mockResolvedValueOnce(
        Response.json({ error: 'Unauthorized', message: 'Unauthorized: account deleted' }, { status: 401 }),
      )

      await expect(getUserProfile()).rejects.toBeInstanceOf(ApiError)
      expect(handler).toHaveBeenCalledWith('Unauthorized: account deleted')
    })

    it('exposes status and parsed message on API errors without signing out', async () => {
      const handler = vi.fn()
      setUnauthorizedHandler(handler)
      vi.mocked(global.fetch).mockResolvedValueOnce(
        Response.json({ error: 'Conflict', message: 'legacy_google_link_required' }, { status: 409 }),
      )

      const error = await getUserProfile().catch((err: unknown) => err)
      expect(error).toBeInstanceOf(ApiError)
      if (!(error instanceof ApiError)) return
      expect(error.status).toBe(409)
      expect(error.serverMessage).toBe('legacy_google_link_required')
      expect(handler).not.toHaveBeenCalled()
    })
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
          headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
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

  describe('deleteAccount', () => {
    it('sends authenticated account deletion request', async () => {
      vi.mocked(getToken).mockResolvedValue('test-token')
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        status: 204,
      })

      await deleteAccount()

      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/users/me'),
        expect.objectContaining({
          method: 'DELETE',
          headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
        }),
      )
    })
  })

  describe('trackEvent', () => {
    it('sends authenticated analytics events', async () => {
      vi.mocked(getToken).mockResolvedValue('test-token')
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        status: 202,
      })

      await trackEvent('first_annotation', { source: 'pdf' })

      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/events'),
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
          body: JSON.stringify({ name: 'first_annotation', properties: { source: 'pdf' } }),
        }),
      )
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

  describe('synthesizeSpeech', () => {
    it('should synthesize speech as audio blob', async () => {
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        blob: async () => new Blob(['audio-bytes'], { type: 'audio/wav' }),
      })

      const result = await synthesizeSpeech({
        highlightedText: 'おはようございます',
        contextText: '丁寧な挨拶',
      })

      expect(result).toBeInstanceOf(Blob)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/ai/speech'),
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            highlightedText: 'おはようございます',
            contextText: '丁寧な挨拶',
          }),
        })
      )
    })

    it('should throw on synthesis error', async () => {
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: async () => 'Failed to synthesize speech',
      })

      await expect(
        synthesizeSpeech({
          highlightedText: 'テスト',
          contextText: '',
        }),
      ).rejects.toThrow('Failed to synthesize speech')
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

  describe('getScanImageBlob', () => {
    it('fetches private scan images with auth headers', async () => {
      vi.mocked(getToken).mockResolvedValue('test-token')
      const image = new Blob(['private image'], { type: 'image/jpeg' })
      ;(global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        blob: async () => image,
      })

      const result = await getScanImageBlob('/v1/scans/1/image')

      expect(result).toBe(image)
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/v1/scans/1/image'),
        expect.objectContaining({
          headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
        }),
      )
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
          headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
        }),
      )
    })
  })
})
