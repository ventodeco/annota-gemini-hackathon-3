import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Hourglass } from 'lucide-react'
import { getScan } from '@/lib/api'
import type { Scan } from '@/lib/types'

export default function LoadingPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const [status, setStatus] = useState<'processing' | 'error'>('processing')

  useEffect(() => {
    if (!id) {
      navigate('/welcome', { replace: true })
      return
    }

    const scanId = parseInt(id, 10)
    if (isNaN(scanId)) {
      navigate('/welcome', { replace: true })
      return
    }

    let isActive = true
    let hasNavigated = false
    let attempts = 0
    const maxAttempts = 30 // 30 seconds max wait
    let timer: ReturnType<typeof setTimeout> | null = null

    const navigateToScan = (scanData?: Scan) => {
      if (!isActive || hasNavigated) return
      hasNavigated = true
      navigate(`/scans/${id}`, {
        replace: true,
        state: scanData ? { preloadedScan: scanData } : undefined,
      })
    }

    const checkScan = async () => {
      if (!isActive || hasNavigated) return

      try {
        const scan = await getScan(scanId)
        if (!isActive || hasNavigated) return

        // Check if OCR is complete (fullText exists)
        if (scan.fullText && scan.fullText.length > 0) {
          navigateToScan(scan)
          return
        }

        attempts++
        if (attempts >= maxAttempts) {
          // Timeout - navigate anyway, page will show what's available
          navigateToScan()
          return
        }

        // Poll again after 1 second
        timer = setTimeout(() => {
          void checkScan()
        }, 1000)
      } catch {
        if (!isActive) return
        setStatus('error')
      }
    }

    void checkScan()

    return () => {
      isActive = false
      if (timer) {
        clearTimeout(timer)
      }
    }
  }, [id, navigate])

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
          Please stay on the page while
          <br />
          the scanning in progess
        </p>
        <Hourglass className="mt-8 text-slate-500" size={32} strokeWidth={1.6} />
      </div>
    </div>
  )
}
