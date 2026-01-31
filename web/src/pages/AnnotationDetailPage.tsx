import { useLocation, useNavigate } from 'react-router-dom'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { formatDate } from '@/lib/api'

interface AnnotationState {
  highlightedText: string
  nuanceSummary: string
  createdAt: string
}

export default function AnnotationDetailPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const annotation = location.state as AnnotationState | null

  if (!annotation) {
    return (
      <div className="min-h-screen bg-gray-50 pb-28">
        <Header title="Annotation" onBack={() => navigate('/history')} />
        <main className="pt-4 px-4">
          <div className="text-center py-8 text-gray-500">
            Annotation not found. Please go back to history.
          </div>
          <button
            onClick={() => navigate('/history')}
            className="w-full mt-4 px-4 py-3 bg-gray-900 text-white rounded-lg font-medium"
          >
            Go to History
          </button>
        </main>
        <BottomNavigation />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="Annotation" onBack={() => navigate('/history')} />
      <main className="pt-4 px-4 space-y-4">
        {/* Highlighted Text */}
        <div className="bg-white rounded-lg p-4 shadow-sm">
          <h2 className="text-sm font-medium text-gray-500 mb-2">Highlighted Text</h2>
          <p className="text-lg font-medium text-gray-900">{annotation.highlightedText}</p>
        </div>

        {/* Explanation */}
        <div className="bg-white rounded-lg p-4 shadow-sm">
          <h2 className="text-sm font-medium text-gray-500 mb-2">Explanation</h2>
          <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceSummary}</p>
        </div>

        {/* Metadata */}
        <div className="bg-white rounded-lg p-4 shadow-sm">
          <p className="text-sm text-gray-400">
            Created: {formatDate(annotation.createdAt)}
          </p>
        </div>
      </main>
      <BottomNavigation />
    </div>
  )
}
