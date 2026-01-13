import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { BookmarkList } from '@/components/history/BookmarkList'
import { EmptyHistory } from '@/components/history/EmptyHistory'
import { useBookmarks } from '@/hooks/useBookmarks'

export default function HistoryPage() {
  const { savedAnnotations, removeAnnotation } = useBookmarks()

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="History" />
      <main className="px-4 pt-4">
        {savedAnnotations.length > 0 ? (
          <BookmarkList
            annotations={savedAnnotations}
            onDelete={removeAnnotation}
          />
        ) : (
          <EmptyHistory />
        )}
      </main>
      <BottomNavigation />
    </div>
  )
}
