import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Hourglass } from 'lucide-react'
import BottomActionBar from '@/components/layout/BottomActionBar'
import { createMockScan } from '@/lib/mockData'

export default function LoadingPage() {
  const navigate = useNavigate()

  useEffect(() => {
    const pendingImageData = sessionStorage.getItem('pendingImage')
    if (!pendingImageData) {
      navigate('/welcome')
      return
    }

    try {
      const imageData = JSON.parse(pendingImageData)
      const mockDelay = 2500

      const timer = setTimeout(() => {
        const mockScan = createMockScan(imageData.blob, imageData.source)
        sessionStorage.removeItem('pendingImage')
        navigate(`/scans/${mockScan.scan.id}`)
      }, mockDelay)

      return () => clearTimeout(timer)
    } catch (error) {
      console.error('Error processing image:', error)
      navigate('/welcome')
    }
  }, [navigate])

  return (
    <div className="flex min-h-screen flex-col bg-white pb-20">
      <div className="flex flex-1 flex-col items-center justify-center p-6">
        <h1 className="mb-4 text-center text-2xl font-bold text-gray-900">
          Scanning in Progress..
        </h1>
        <p className="mb-8 text-center text-sm/relaxed text-gray-600">
          Please stay on the page while the scanning in progress
        </p>
        <Hourglass className="size-16 animate-pulse text-gray-400" />
      </div>
      <BottomActionBar disabled={true} />
    </div>
  )
}
