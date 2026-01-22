import type { Annotation } from '@/lib/types'
import { BookmarkCard } from './BookmarkCard'

interface BookmarkListProps {
  annotations: Annotation[]
  onDelete: (id: number) => void
}

export function BookmarkList({ annotations, onDelete }: BookmarkListProps) {
  const sortedAnnotations = [...annotations].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  )

  return (
    <div className="space-y-3">
      {sortedAnnotations.map((annotation) => (
        <BookmarkCard
          key={annotation.id}
          annotation={annotation}
          onDelete={onDelete}
        />
      ))}
    </div>
  )
}
