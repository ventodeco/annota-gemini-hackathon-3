import { Button } from '@/components/ui/button'
import { Sparkles, Bookmark } from 'lucide-react'

interface BottomActionBarProps {
  disabled?: boolean
  onExplain?: () => void
  onBookmark?: () => void
}

export default function BottomActionBar({
  disabled = false,
  onExplain,
  onBookmark,
}: BottomActionBarProps) {
  return (
    <div className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200 safe-area-inset-bottom">
      <div className="flex items-center justify-between px-4 py-3 max-w-md mx-auto gap-4">
        <Button
          onClick={onExplain}
          disabled={disabled}
          className="flex-1 bg-gray-900 text-white hover:bg-gray-800 rounded-lg px-6 py-3 h-auto"
        >
          <Sparkles className="w-5 h-5 mr-2" />
          Explain this
        </Button>
        <button
          onClick={onBookmark}
          disabled={disabled}
          className="p-2 disabled:opacity-50"
          aria-label="Bookmark"
        >
          <Bookmark className="w-6 h-6 text-gray-900" />
        </button>
      </div>
    </div>
  )
}
