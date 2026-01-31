import { useParams, useNavigate } from 'react-router-dom'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { formatDate } from '@/lib/api'
import { useAnnotationById } from '@/hooks/useAnnotationById'

export default function AnnotationDetailPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const annotationId = id ? parseInt(id, 10) : undefined

  const { data: annotation, isLoading, error } = useAnnotationById(annotationId)

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 pb-28">
        <Header title="Annotation" onBack={() => navigate('/history')} />
        <main className="pt-4 px-4">
          <div className="flex justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
          </div>
        </main>
        <BottomNavigation />
      </div>
    )
  }

  if (error || !annotation) {
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

        {/* Meaning */}
        <div className="bg-white rounded-lg p-4 shadow-sm">
          <h2 className="text-sm font-medium text-gray-500 mb-2">Meaning</h2>
          <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceData.meaning}</p>
        </div>

        {/* Usage Example */}
        {annotation.nuanceData.usageExample && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Usage Example</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceData.usageExample}</p>
          </div>
        )}

        {/* When to Use */}
        {annotation.nuanceData.usageTiming && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">When to Use</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceData.usageTiming}</p>
          </div>
        )}

        {/* Word Breakdown */}
        {annotation.nuanceData.wordBreakdown && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Word Breakdown</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceData.wordBreakdown}</p>
          </div>
        )}

        {/* Alternative Meaning */}
        {annotation.nuanceData.alternativeMeaning && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Alternative Meaning</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceData.alternativeMeaning}</p>
          </div>
        )}

        {/* Context */}
        {annotation.contextText && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Context</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.contextText}</p>
          </div>
        )}

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
