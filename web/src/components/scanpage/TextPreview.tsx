import { Button } from '@/components/ui/button'
import { useTextSelection } from '@/hooks/useTextSelection'
import { useAnnotation } from '@/hooks/useAnnotation'
import { Alert, AlertDescription } from '@/components/ui/alert'
import AnnotationCard from './AnnotationCard'

interface TextPreviewProps {
  text: string
  scanID: string
}

export default function TextPreview({ text, scanID }: TextPreviewProps) {
  const { selectedText, handleSelection, clearSelection } = useTextSelection()
  const annotationMutation = useAnnotation(scanID)

  const handleTextSelect = () => {
    const selection = window.getSelection()
    if (!selection || selection.toString().trim() === '') {
      return
    }

    const result = handleSelection(selection.toString())
    if (!result.valid) {
      alert(result.error)
    }
  }

  const handleAnnotate = async () => {
    if (!selectedText) return

    annotationMutation.mutate(
      { selectedText },
      {
        onSuccess: () => {
          clearSelection()
        },
      }
    )
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-xl font-semibold">Extracted Text</h2>
        <div
          className="prose max-w-none"
          onMouseUp={handleTextSelect}
          onTouchEnd={handleTextSelect}
        >
          <p className="text-lg/relaxed whitespace-pre-wrap text-gray-800">
            {text}
          </p>
        </div>
      </div>

      {selectedText && (
        <div className="rounded-lg bg-white p-6 shadow-sm">
          <h3 className="mb-2 text-lg font-semibold">Selected Text</h3>
          <p className="mb-4 text-gray-700">&quot;{selectedText}&quot;</p>
          <Button
            onClick={handleAnnotate}
            disabled={annotationMutation.isPending}
            className="w-full"
          >
            {annotationMutation.isPending ? 'Annotating...' : 'Annotate Selected Text'}
          </Button>
        </div>
      )}

      {annotationMutation.isError && (
        <Alert variant="destructive">
          <AlertDescription>
            {annotationMutation.error instanceof Error
              ? annotationMutation.error.message
              : 'Failed to create annotation'}
          </AlertDescription>
        </Alert>
      )}

      {annotationMutation.isSuccess && annotationMutation.data && (
        <AnnotationCard annotation={annotationMutation.data} />
      )}
    </div>
  )
}
