import { useNavigate } from 'react-router-dom'
import { Trash2 } from 'lucide-react'
import type { Annotation } from '@/lib/types'
import { toast } from 'sonner'

interface BookmarkCardProps {
  annotation: Annotation
  onDelete: (id: number) => void
}

export function BookmarkCard({ annotation, onDelete }: BookmarkCardProps) {
  const navigate = useNavigate()

  const handleCardClick = () => {
    if (annotation.scan_id) {
      navigate(`/scans/${annotation.scan_id}`)
    } else {
      toast.error('Scan not found', {
        description: 'The original scan for this annotation is no longer available.',
      })
    }
  }

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation()
    onDelete(annotation.id)
    toast.success('Annotation removed', {
      description: 'The annotation has been removed from your history.',
    })
  }

  const formatDate = (dateString: string) => {
    const date = new Date(dateString)
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  }

  const truncateText = (text: string, maxLength: number) => {
    if (text.length <= maxLength) return text
    return text.slice(0, maxLength) + '...'
  }

  return (
    <div
      className="bg-white rounded-xl p-4 shadow-sm border border-gray-100 active:opacity-70 transition-opacity cursor-pointer"
      onClick={handleCardClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          handleCardClick()
        }
      }}
    >
      <div className="flex justify-between items-start gap-3">
        <div className="flex-1 min-w-0">
          <p className="text-lg font-medium text-gray-900 mb-1 line-clamp-2">
            {truncateText(annotation.highlighted_text, 60)}
          </p>
          <p className="text-sm text-gray-600 line-clamp-2">
            {truncateText(annotation.nuance_data.meaning, 100)}
          </p>
          <p className="text-xs text-gray-400 mt-2">
            {formatDate(annotation.created_at)}
          </p>
        </div>
        <button
          onClick={handleDelete}
          className="shrink-0 p-2 text-gray-400 hover:text-red-500 active:bg-red-50 rounded-lg transition-colors"
          aria-label="Delete annotation"
        >
          <Trash2 className="w-5 h-5" />
        </button>
      </div>
    </div>
  )
}
