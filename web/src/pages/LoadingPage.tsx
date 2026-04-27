import { useCallback, useEffect, useRef } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Hourglass } from 'lucide-react'
import Header from '@/components/layout/Header'
import { useScan, isScanFailed, isScanOcrReady } from '@/hooks/useScan'
import type { Scan } from '@/lib/types'

const LOADING_POLL_INTERVAL_MS = 3000

export default function LoadingPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const hasNavigatedRef = useRef(false)
  const latestScanRef = useRef<Scan | undefined>(undefined)
  const scanIdParsed = id ? Number.parseInt(id, 10) : NaN
  const scanId = Number.isInteger(scanIdParsed) && scanIdParsed > 0 ? scanIdParsed : undefined
  const scanHistoryPath = scanId ? `/history?scanId=${scanId}` : '/history'

  const navigateToScan = useCallback((scanData?: Scan) => {
    if (!id || hasNavigatedRef.current) return
    hasNavigatedRef.current = true
    navigate(`/scans/${id}`, {
      replace: true,
      state: scanData ? { preloadedScan: scanData } : undefined,
    })
  }, [id, navigate])

  const { data: scan, error } = useScan(scanId, {
    pollIntervalMs: LOADING_POLL_INTERVAL_MS,
    retry: false,
  })

  useEffect(() => {
    if (!id || !Number.isInteger(scanIdParsed) || scanIdParsed <= 0) {
      navigate('/welcome', { replace: true })
    }
  }, [id, navigate, scanIdParsed])

  useEffect(() => {
    if (!id || !Number.isInteger(scanIdParsed) || scanIdParsed <= 0) {
      return
    }
    const timeoutId = setTimeout(() => {
      const currentScan = latestScanRef.current
      if (currentScan && isScanOcrReady(currentScan)) return
      navigateToScan()
    }, 30000)
    return () => clearTimeout(timeoutId)
  }, [id, scanIdParsed, navigateToScan])

  useEffect(() => {
    latestScanRef.current = scan
  }, [scan])

  useEffect(() => {
    if (scan && isScanOcrReady(scan)) {
      navigateToScan(scan)
    }
  }, [navigateToScan, scan])

  const status: 'processing' | 'error' = error || isScanFailed(scan) ? 'error' : 'processing'

  if (status === 'error') {
    return (
      <div className="min-h-screen bg-white flex flex-col pb-20">
        <Header
          title="Scan Result"
          rightAction="bookmark"
          rightActionTo={scanHistoryPath}
        />
        <div className="flex-1 flex flex-col items-center justify-center p-6">
          <p className="text-center text-red-600 text-sm">
            {scan?.failureReason ? `Scan failed: ${scan.failureReason}` : 'Failed to load scan'}
          </p>
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
      <Header
        title="Scan Result"
        rightAction="bookmark"
        rightActionTo={scanHistoryPath}
      />
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <h1 className="text-center text-gray-900 font-semibold text-lg leading-6">
          Scanning in Progress..
        </h1>
        <p className="mt-6 max-w-xs text-center text-gray-900 text-base leading-6">
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
