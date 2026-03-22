import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface PageNavigatorProps {
  currentPage: number
  totalPages: number
  onPrev: () => void
  onNext: () => void
}

export default function PageNavigator({ currentPage, totalPages, onPrev, onNext }: PageNavigatorProps) {
  return (
    <div className="flex items-center justify-between px-4 py-3 border-t bg-white">
      <Button variant="ghost" size="sm" onClick={onPrev} disabled={currentPage <= 1} aria-label="Previous page">
        <ChevronLeft className="w-5 h-5" />
      </Button>
      <span className="text-sm font-medium text-gray-700">{currentPage} / {totalPages}</span>
      <Button variant="ghost" size="sm" onClick={onNext} disabled={currentPage >= totalPages} aria-label="Next page">
        <ChevronRight className="w-5 h-5" />
      </Button>
    </div>
  )
}
