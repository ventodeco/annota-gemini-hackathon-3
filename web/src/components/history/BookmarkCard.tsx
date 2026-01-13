import { useNavigate } from 'react-router-dom'
import { Trash2 } from 'lucide-react'
import type { Annotation } from '@/lib/types'
import { getMockScan } from '@/lib/mockData'
import { toast } from 'sonner'

interface BookmarkCardProps {
  annotation: Annotation
  onDelete: (id: string) => void
}

export function BookmarkCard({ annotation, onDelete }: BookmarkCardProps) {
  const navigate = useNavigate()

  const handleCardClick = () => {
    const scan = getMockScan(annotation.scanID)
    if (scan) {
      navigate(`/scans/${annotation.scanID}`)
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
      className="
        cursor-pointer rounded-xl border border-gray-100 bg-white p-4 shadow-sm
        transition-opacity
        active:opacity-70
      "
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
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="mb-1 line-clamp-2 text-lg font-medium text-gray-900">
            {truncateText(annotation.selectedText, 60)}
          </p>
          <p className="line-clamp-2 text-sm text-gray-600">
            {truncateText(annotation.meaning, 100)}
          </p>
          <p className="mt-2 text-xs text-gray-400">
            {formatDate(annotation.createdAt)}
          </p>
        </div>
        <button
          onClick={handleDelete}
          className="
            shrink-0 rounded-lg p-2 text-gray-400 transition-colors
            hover:text-red-500
            active:bg-red-50
          "
          aria-label="Delete annotation"
        >
          <Trash2 className="size-5" />
        </button>
      </div>
    </div>
  )
}
