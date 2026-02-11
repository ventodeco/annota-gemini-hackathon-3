import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Trash2 } from 'lucide-react'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { useScans, useDeleteScan } from '@/hooks/useScans'
import { formatDate } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { toast } from 'sonner'

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
  const deleteScan = useDeleteScan()
  const [scanToDelete, setScanToDelete] = useState<ScanHistoryItem | null>(null)

  const handleDeleteClick = (e: React.MouseEvent, scan: ScanHistoryItem) => {
    e.stopPropagation()
    setScanToDelete(scan)
  }

  const handleDeleteConfirm = async () => {
    if (!scanToDelete) return
    try {
      await deleteScan.mutateAsync(scanToDelete.id)
      setScanToDelete(null)
      toast.success('Scan removed', {
        description: 'The scan has been removed from your history.',
      })
    } catch (err) {
      toast.error('Failed to remove scan', {
        description: 'Please try again.',
      })
    }
  }

  const handleDeleteCancel = () => {
    setScanToDelete(null)
  }

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
              <div
                key={scan.id}
                className="flex items-center gap-3 bg-white rounded-lg p-4 shadow-sm"
              >
                <button
                  onClick={() => navigate(`/scans/${scan.id}`)}
                  aria-label={`Open OCR #${scan.id}`}
                  className="flex-1 text-left min-w-0"
                >
                  <p className="font-medium text-gray-900">{`OCR #${scan.id}`}</p>
                  <p className="text-sm text-gray-500 mt-1">
                    Language: {scan.detectedLanguage || 'Unknown'}
                  </p>
                  <p className="text-xs text-gray-400 mt-2">
                    {formatDate(scan.createdAt)}
                  </p>
                </button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={(e) => handleDeleteClick(e, scan)}
                  aria-label="Delete scan"
                  className="shrink-0 text-gray-500 hover:text-red-600"
                >
                  <Trash2 className="h-5 w-5" />
                </Button>
              </div>
            ))}
          </div>
        )}

        <AlertDialog open={!!scanToDelete} onOpenChange={(open) => !open && handleDeleteCancel()}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Remove scan?</AlertDialogTitle>
              <AlertDialogDescription>
                Remove this scan and its image? Annotations will remain in History.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel onClick={handleDeleteCancel}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={handleDeleteConfirm}
                disabled={deleteScan.isPending}
                className="bg-red-600 hover:bg-red-700"
              >
                {deleteScan.isPending ? 'Removing...' : 'Remove'}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

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
