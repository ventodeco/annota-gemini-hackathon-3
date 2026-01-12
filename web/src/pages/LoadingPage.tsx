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
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <h1 className="text-2xl font-bold text-gray-900 text-center mb-4">
          Scanning in Progress..
        </h1>
        <p className="text-center text-gray-600 mb-8 text-sm leading-relaxed">
          Please stay on the page while the scanning in progress
        </p>
        <Hourglass className="w-16 h-16 text-gray-400 animate-pulse" />
      </div>
      <BottomActionBar disabled={true} />
    </div>
  )
}
