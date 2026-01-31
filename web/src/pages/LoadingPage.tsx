import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getScan } from '@/lib/api'

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

    let attempts = 0
    const maxAttempts = 30 // 30 seconds max wait

    const checkScan = async () => {
      try {
        const scan = await getScan(scanId)
        // Check if OCR is complete (fullText exists)
        if (scan.fullText && scan.fullText.length > 0) {
          navigate(`/scans/${id}`, { replace: true })
          return
        }

        attempts++
        if (attempts >= maxAttempts) {
          // Timeout - navigate anyway, page will show what's available
          navigate(`/scans/${id}`, { replace: true })
          return
        }

        // Poll again after 1 second
        setTimeout(checkScan, 1000)
      } catch {
        setStatus('error')
      }
    }

    checkScan()
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
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
        <p className="mt-4 text-center text-gray-600 text-sm">Processing your image...</p>
        <p className="mt-2 text-center text-gray-400 text-xs">This may take a few seconds</p>
      </div>
    </div>
  )
}
