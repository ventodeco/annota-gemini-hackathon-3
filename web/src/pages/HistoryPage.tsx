import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { PackageOpen, Trash2 } from 'lucide-react'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { useAnnotations, useDeleteAnnotation } from '@/hooks/useAnnotations'
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

interface AnnotationItem {
  id: number
  highlightedText: string
  nuanceSummary: string
  createdAt: string
}

export default function HistoryPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const scanIdParam = searchParams.get('scanId')
  const parsedScanId = scanIdParam ? Number.parseInt(scanIdParam, 10) : undefined
  const scanId = Number.isInteger(parsedScanId) && parsedScanId! > 0 ? parsedScanId : undefined
  const [page, setPage] = useState(1)
  const { data, isLoading, error } = useAnnotations(page, 20, scanId)
  const deleteAnnotation = useDeleteAnnotation()
  const [annotationToDelete, setAnnotationToDelete] = useState<AnnotationItem | null>(null)

  const handleAnnotationClick = (item: AnnotationItem) => {
    const detailPath = scanId ? `/annotations/${item.id}?scanId=${scanId}` : `/annotations/${item.id}`
    navigate(detailPath, { state: item })
  }

  const handleDeleteClick = (e: React.MouseEvent, item: AnnotationItem) => {
    e.stopPropagation()
    setAnnotationToDelete(item)
  }

  const handleDeleteConfirm = async () => {
    if (!annotationToDelete) return
    try {
      await deleteAnnotation.mutateAsync(annotationToDelete.id)
      setAnnotationToDelete(null)
      toast.success('Annotation removed', {
        description: 'The annotation has been removed from your history.',
      })
    } catch (err) {
      toast.error('Failed to remove annotation', {
        description: 'Please try again.',
      })
    }
  }

  const handleDeleteCancel = () => {
    setAnnotationToDelete(null)
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title={scanId ? `Annotations for OCR #${scanId}` : 'History'} />
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
          <div className="flex min-h-[60vh] flex-col items-center justify-center text-center text-gray-500">
            <PackageOpen className="mb-5 h-16 w-16 text-slate-400" aria-label="empty history" />
            <p className="max-w-[260px] text-base font-normal leading-relaxed text-slate-500">
              {scanId
                ? "Looks like this scan doesn't have saved annotations yet!"
                : "Looks like you haven't saved any annotations yet!"}
            </p>
          </div>
        )}

        {!isLoading && !error && data?.data.length !== undefined && data.data.length > 0 && (
          <div className="space-y-4">
            {data.data.map((item: AnnotationItem) => (
              <div
                key={item.id}
                className="flex items-center gap-3 bg-white rounded-lg p-4 shadow-sm"
              >
                <div
                  className="flex-1 min-w-0 cursor-pointer"
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
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={(e) => handleDeleteClick(e, item)}
                  aria-label="Delete annotation"
                  className="shrink-0 text-gray-500 hover:text-red-600"
                >
                  <Trash2 className="h-5 w-5" />
                </Button>
              </div>
            ))}
          </div>
        )}

        <AlertDialog
          open={!!annotationToDelete}
          onOpenChange={(open) => !open && handleDeleteCancel()}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Remove annotation?</AlertDialogTitle>
              <AlertDialogDescription>
                Remove this annotation? This action cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel onClick={handleDeleteCancel}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={handleDeleteConfirm}
                disabled={deleteAnnotation.isPending}
                className="bg-red-600 hover:bg-red-700"
              >
                {deleteAnnotation.isPending ? 'Removing...' : 'Remove'}
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
