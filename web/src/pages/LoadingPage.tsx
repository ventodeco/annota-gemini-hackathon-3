import { useCallback, useEffect, useRef, type ReactElement } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Hourglass } from 'lucide-react'
import Header from '@/components/layout/Header'
import ScanFailureState from '@/components/scanpage/ScanFailureState'
import { useScan, isScanFailed, isScanOcrReady } from '@/hooks/useScan'
import { getScanFailureCopy, getScanReference } from '@/lib/scanStatus'
import { getErrorReference } from '@/lib/errorReference'
import type { Scan } from '@/lib/types'

const LOADING_POLL_INTERVAL_MS = 3000

export default function LoadingPage(): ReactElement {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const hasNavigatedRef = useRef(false)
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

  // Poll until the scan is ready or failed; the server always settles a scan
  // within its OCR deadline, so there is no client-side timeout.
  const { data: scan, error, refetch, isRefetching } = useScan(scanId, {
    pollIntervalMs: LOADING_POLL_INTERVAL_MS,
    retry: false,
  })

  useEffect(() => {
    if (!scanId) {
      navigate('/welcome', { replace: true })
    }
  }, [navigate, scanId])

  useEffect(() => {
    if (scan && isScanOcrReady(scan)) {
      navigateToScan(scan)
    }
  }, [navigateToScan, scan])

  const header = (
    <Header
      title="Scan Result"
      rightAction="bookmark"
      rightActionTo={scanHistoryPath}
    />
  )

  if (isScanFailed(scan)) {
    const copy = getScanFailureCopy(scan?.failureReason)
    return (
      <div className="min-h-screen bg-white flex flex-col pb-20">
        {header}
        <ScanFailureState
          title={copy.title}
          description={copy.description}
          actionLabel="Upload another page"
          onAction={() => navigate('/welcome')}
          reference={scanId ? getScanReference(scanId) : null}
        />
      </div>
    )
  }

  if (error && !isRefetching) {
    return (
      <div className="min-h-screen bg-white flex flex-col pb-20">
        {header}
        <ScanFailureState
          title="Couldn't check this scan"
          description="Your scan is saved. Check your connection and try again."
          actionLabel="Try again"
          onAction={() => void refetch()}
          secondaryActionLabel="Go back"
          onSecondaryAction={() => navigate('/welcome')}
          reference={getErrorReference(error)}
        />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      {header}
      <div className="flex-1 flex flex-col items-center justify-center p-6" role="status" aria-live="polite">
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
