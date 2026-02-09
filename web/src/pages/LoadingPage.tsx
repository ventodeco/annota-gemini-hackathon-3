import { useCallback, useEffect, useRef } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Hourglass } from 'lucide-react'
import { getScan } from '@/lib/api'
import { isScanOcrReady } from '@/hooks/useScan'
import type { Scan } from '@/lib/types'

export default function LoadingPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const hasNavigatedRef = useRef(false)
  const latestScanRef = useRef<Scan | undefined>(undefined)
  const scanId = id ? parseInt(id, 10) : NaN

  const navigateToScan = useCallback((scanData?: Scan) => {
    if (!id || hasNavigatedRef.current) return
    hasNavigatedRef.current = true
    navigate(`/scans/${id}`, {
      replace: true,
      state: scanData ? { preloadedScan: scanData } : undefined,
    })
  }, [id, navigate])

  const { data: scan, error } = useQuery({
    queryKey: ['scan', scanId],
    queryFn: () => getScan(scanId),
    enabled: Number.isInteger(scanId) && scanId > 0,
    retry: false,
    refetchInterval: (query) => {
      const currentScan = query.state.data as Scan | undefined
      if (!currentScan) return false
      if (isScanOcrReady(currentScan)) return false
      return 1000
    },
  })

  useEffect(() => {
    if (!id || !Number.isInteger(scanId) || scanId <= 0) {
      navigate('/welcome', { replace: true })
    }
  }, [id, navigate, scanId])

  useEffect(() => {
    if (!id || !Number.isInteger(scanId) || scanId <= 0) {
      return
    }
    const timeoutId = setTimeout(() => {
      const currentScan = latestScanRef.current
      if (currentScan && isScanOcrReady(currentScan)) return
      navigateToScan()
    }, 30000)
    return () => clearTimeout(timeoutId)
  }, [id, scanId, navigateToScan])

  useEffect(() => {
    latestScanRef.current = scan
  }, [scan])

  useEffect(() => {
    if (scan && isScanOcrReady(scan)) {
      navigateToScan(scan)
    }
  }, [navigateToScan, scan])

  const status: 'processing' | 'error' = error ? 'error' : 'processing'

  if (status === 'error') {
    return (
      <div className="min-h-screen bg-white flex flex-col pb-20">
        <div className="flex-1 flex flex-col items-center justify-center p-6">
          <p className="text-center text-red-600 text-sm">Failed to load scan</p>
          <button
            onClick={() => navigate('/welcome')}
            className="mt-4 px-4 py-2 bg-gray-900 text-white rounded-full text-sm"
          >
            Go back
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <h1
          className="text-center text-gray-900"
          style={{
            fontWeight: 600,
            fontStyle: 'normal',
            fontSize: 'var(--paragraph-regular-font-size, 16px)',
            lineHeight: '24px',
          }}
        >
          Scanning in Progress..
        </h1>
        <p
          className="mt-6 max-w-[320px] text-center text-gray-900"
          style={{
            fontSize: '16px',
            fontWeight: 400,
            lineHeight: '24px',
          }}
        >
          Processing your image and checking OCR status.
          <br />
          Please stay on this page while
          <br />
          scanning is in progress.
        </p>
        <Hourglass className="mt-8 text-slate-500" size={32} strokeWidth={1.6} />
      </div>
    </div>
  )
}
