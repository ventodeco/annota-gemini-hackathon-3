import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { useAnnotations } from '@/hooks/useAnnotations'
import { formatDate } from '@/lib/api'

interface AnnotationItem {
  id: number
  highlightedText: string
  nuanceSummary: string
  createdAt: string
}

export default function HistoryPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const { data, isLoading, error } = useAnnotations(page, 20)

  const handleAnnotationClick = (item: AnnotationItem) => {
    navigate(`/annotations/${item.id}`, { state: item })
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="History" />
      <main className="pt-4 px-4">
        {isLoading && (
          <div className="flex justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
          </div>
        )}

        {error && (
          <div className="text-center py-8 text-red-600">
            Failed to load history. Please try again.
          </div>
        )}

        {!isLoading && !error && data?.data.length === 0 && (
          <div className="text-center py-8 text-gray-500">
            No annotations yet. Start scanning to create some!
          </div>
        )}

        {!isLoading && !error && data?.data.length !== undefined && data.data.length > 0 && (
          <div className="space-y-4">
            {data.data.map((item: AnnotationItem) => (
              <div
                key={item.id}
                className="bg-white rounded-lg p-4 shadow-sm cursor-pointer"
                onClick={() => handleAnnotationClick(item)}
              >
                <p className="font-medium text-gray-900">{item.highlightedText}</p>
                <p className="text-sm text-gray-500 mt-1 line-clamp-2">
                  {item.nuanceSummary}
                </p>
                <p className="text-xs text-gray-400 mt-2">
                  {formatDate(item.createdAt)}
                </p>
              </div>
            ))}
          </div>
        )}

        {data?.meta && (
          <div className="flex justify-center gap-4 mt-6">
            {data.meta.previousPage && (
              <button
                onClick={() => setPage(data.meta.previousPage!)}
                className="px-4 py-2 bg-white rounded-lg shadow-sm text-sm font-medium"
              >
                Previous
              </button>
            )}
            <span className="px-4 py-2 text-sm text-gray-500">
              Page {data.meta.currentPage}
            </span>
            {data.meta.nextPage && (
              <button
                onClick={() => setPage(data.meta.nextPage!)}
                className="px-4 py-2 bg-white rounded-lg shadow-sm text-sm font-medium"
              >
                Next
              </button>
            )}
          </div>
        )}
      </main>
      <BottomNavigation />
    </div>
  )
}
