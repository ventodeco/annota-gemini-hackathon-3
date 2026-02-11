import { useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { Trash2 } from 'lucide-react'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { formatDate } from '@/lib/api'
import { useAnnotationById } from '@/hooks/useAnnotationById'
import { useDeleteAnnotation } from '@/hooks/useAnnotations'
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

export default function AnnotationDetailPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { id } = useParams<{ id: string }>()
  const annotationId = id ? parseInt(id, 10) : undefined
  const scanIdParam = searchParams.get('scanId')
  const historyPath = scanIdParam ? `/history?scanId=${scanIdParam}` : '/history'
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  const { data: annotation, isLoading, error } = useAnnotationById(annotationId)
  const deleteAnnotation = useDeleteAnnotation()

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 pb-28">
        <Header title="Annotation" />
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
        <Header title="Annotation" />
        <main className="pt-4 px-4">
          <div className="text-center py-8 text-gray-500">
            Annotation not found. Please go back to history.
          </div>
          <button
            onClick={() => navigate(historyPath)}
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
      <Header title="Annotation" />
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

        <Button
          variant="outline"
          className="w-full border-red-200 text-red-600 hover:bg-red-50 hover:text-red-700"
          onClick={() => setShowDeleteDialog(true)}
        >
          <Trash2 className="h-4 w-4 mr-2" />
          Remove annotation
        </Button>

        <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Remove annotation?</AlertDialogTitle>
              <AlertDialogDescription>
                Remove this annotation? This action cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={async () => {
                  if (!annotation) return
                  try {
                    await deleteAnnotation.mutateAsync(annotation.id)
                    setShowDeleteDialog(false)
                    toast.success('Annotation removed')
                    navigate(historyPath)
                  } catch (err) {
                    toast.error('Failed to remove annotation')
                  }
                }}
                disabled={deleteAnnotation.isPending}
                className="bg-red-600 hover:bg-red-700"
              >
                {deleteAnnotation.isPending ? 'Removing...' : 'Remove'}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </main>
      <BottomNavigation />
    </div>
  )
}
