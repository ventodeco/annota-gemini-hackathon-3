import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { useScans } from '@/hooks/useScans'
import { formatDate } from '@/lib/api'

interface ScanHistoryItem {
  id: number
  imageUrl: string
  detectedLanguage?: string
  createdAt: string
}

export default function OCRHistoryPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const { data, isLoading, error } = useScans(page, 20)

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="OCR History" />
      <main className="pt-4 px-4">
        {isLoading && (
          <div className="flex justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
          </div>
        )}

        {error && (
          <div className="text-center py-8 text-red-600">
            Failed to load OCR history. Please try again.
          </div>
        )}

        {!isLoading && !error && data?.data.length === 0 && (
          <div className="text-center py-8">
            <p className="text-gray-500 mb-4">No OCR scans yet.</p>
            <Link
              to="/welcome"
              className="inline-flex items-center justify-center px-4 py-2 bg-gray-900 text-white rounded-lg text-sm font-medium"
            >
              Start scanning
            </Link>
          </div>
        )}

        {!isLoading && !error && data?.data.length !== undefined && data.data.length > 0 && (
          <div className="space-y-4">
            {data.data.map((scan: ScanHistoryItem) => (
              <button
                key={scan.id}
                onClick={() => navigate(`/scans/${scan.id}`)}
                aria-label={`Open OCR #${scan.id}`}
                className="w-full text-left bg-white rounded-lg p-4 shadow-sm"
              >
                <p className="font-medium text-gray-900">{`OCR #${scan.id}`}</p>
                <p className="text-sm text-gray-500 mt-1">
                  Language: {scan.detectedLanguage || 'Unknown'}
                </p>
                <p className="text-xs text-gray-400 mt-2">
                  {formatDate(scan.createdAt)}
                </p>
              </button>
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
