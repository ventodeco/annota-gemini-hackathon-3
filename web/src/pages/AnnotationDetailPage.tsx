import { useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { ExternalLink, Trash2, Volume2 } from 'lucide-react'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { formatDate } from '@/lib/api'
import { useAnnotationById } from '@/hooks/useAnnotationById'
import { useDeleteAnnotation, useSynthesizeSpeech } from '@/hooks/useAnnotations'
import { useSpeechPlayback } from '@/hooks/useSpeechPlayback'
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
import { formatNuanceData } from '@/lib/annotationFormatting'
import type { AnnotationDetail } from '@/lib/types'

function getSourcePath(annotation: AnnotationDetail): string {
  if (annotation.documentId) {
    return `/documents/${annotation.documentId}?page=${annotation.pageNumber || 1}`
  }
  if (annotation.scanId) {
    return `/scans/${annotation.scanId}`
  }
  return ''
}

function getSpeechButtonCopy(isPlaying: boolean, isPending: boolean): string {
  if (isPlaying) {
    return 'Stop audio'
  }
  if (isPending) {
    return 'Loading audio...'
  }
  return 'Replay TTS'
}

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
  const synthesizeSpeech = useSynthesizeSpeech()
  const speech = useSpeechPlayback()

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 pb-28">
        <Header title="Annotation" />
        <main className="pt-4 px-4 max-w-md mx-auto" aria-busy="true">
          <div className="flex justify-center py-8" role="status" aria-label="Loading annotation">
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
        <main className="pt-4 px-4 max-w-md mx-auto">
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

  const formattedNuance = formatNuanceData(annotation.nuanceData)
  const sourcePath = getSourcePath(annotation)

  const handleReplaySpeech = async () => {
    if (speech.isPlaying || synthesizeSpeech.isPending) {
      speech.stop()
      return
    }
    try {
      const audio = await synthesizeSpeech.mutateAsync({
        highlightedText: annotation.highlightedText,
        contextText: annotation.contextText,
      })
      await speech.play(audio)
    } catch {
      speech.stop()
      toast.error('Failed to play audio')
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="Annotation" />
      <main className="pt-4 px-4 max-w-md mx-auto space-y-4">
        {/* Highlighted Text */}
        <div className="bg-white rounded-lg p-4 shadow-sm">
          <h2 className="text-sm font-medium text-gray-500 mb-2">Highlighted Text</h2>
          <p className="text-lg font-medium text-gray-900">{annotation.highlightedText}</p>
          <Button
            variant="outline"
            className="mt-4 w-full"
            onClick={handleReplaySpeech}
            disabled={synthesizeSpeech.isPending}
          >
            <Volume2 className="h-4 w-4 mr-2" />
            {getSpeechButtonCopy(speech.isPlaying, synthesizeSpeech.isPending)}
          </Button>
        </div>

        {annotation.sourceLabel && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Source</h2>
            <p className="text-base text-gray-700">{annotation.sourceLabel}</p>
            {sourcePath && (
              <Button
                variant="outline"
                className="mt-4 w-full"
                onClick={() => navigate(sourcePath)}
              >
                <ExternalLink className="h-4 w-4 mr-2" />
                Open source
              </Button>
            )}
          </div>
        )}

        {/* Meaning */}
        <div className="bg-white rounded-lg p-4 shadow-sm">
          <h2 className="text-sm font-medium text-gray-500 mb-2">Translation</h2>
          <p className="text-base text-gray-700 whitespace-pre-wrap">{formattedNuance.translation}</p>
        </div>

        <div className="bg-white rounded-lg p-4 shadow-sm">
          <h2 className="text-sm font-medium text-gray-500 mb-2">Explanation</h2>
          <p className="text-base text-gray-700 whitespace-pre-wrap">{formattedNuance.explanation}</p>
        </div>

        {formattedNuance.pronunciation && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Pronunciation</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{formattedNuance.pronunciation}</p>
          </div>
        )}

        {/* Usage Example */}
        {annotation.nuanceData.usageExample && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Usage Example</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{annotation.nuanceData.usageExample}</p>
          </div>
        )}

        {/* When to Use */}
        {formattedNuance.whenToUse && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">When to Use</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{formattedNuance.whenToUse}</p>
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
        {formattedNuance.alternativeMeanings && (
          <div className="bg-white rounded-lg p-4 shadow-sm">
            <h2 className="text-sm font-medium text-gray-500 mb-2">Alternative Meanings</h2>
            <p className="text-base text-gray-700 whitespace-pre-wrap">{formattedNuance.alternativeMeanings}</p>
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
                  } catch {
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
