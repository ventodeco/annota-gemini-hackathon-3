import { Button } from '@/components/ui/button'
import { useTextSelection } from '@/hooks/useTextSelection'
import { useAnnotation } from '@/hooks/useAnnotation'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface TextPreviewProps {
  text: string
  scanId: number
}

export default function TextPreview({ text, scanId }: TextPreviewProps) {
  const { selectedText, handleSelection, clearSelection } = useTextSelection()
  const annotationMutation = useAnnotation(scanId)

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
      { textToAnalyze: selectedText, context: text },
      {
        onSuccess: () => {
          clearSelection()
        },
      }
    )
  }

  return (
    <div className="space-y-4">
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-semibold mb-4">Extracted Text</h2>
        <div
          className="prose max-w-none"
          onMouseUp={handleTextSelect}
          onTouchEnd={handleTextSelect}
        >
          <p className="text-lg leading-relaxed text-gray-800 whitespace-pre-wrap">
            {text}
          </p>
        </div>
      </div>

      {selectedText && (
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold mb-2">Selected Text</h3>
          <p className="text-gray-700 mb-4">&quot;{selectedText}&quot;</p>
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

      {annotationMutation.isSuccess && (
        <Alert>
          <AlertDescription>
            Annotation created successfully!
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
