import { Button } from '@/components/ui/button'
import { Sparkles, Bookmark, Loader2 } from 'lucide-react'

interface BottomActionBarProps {
  disabled?: boolean
  bookmarkDisabled?: boolean
  isLoading?: boolean
  onExplain?: () => void
  onBookmark?: () => void
}

export default function BottomActionBar({
  disabled = false,
  bookmarkDisabled = false,
  isLoading = false,
  onExplain,
  onBookmark,
}: BottomActionBarProps) {
  return (
    <div className="fixed bottom-8 left-1/2 -translate-x-1/2 w-[348px] h-[72px] bg-white border border-[#F1F5F9] rounded-[16px] shadow-[0_20px_25px_-5px_rgba(0,0,0,0.1),0_8px_10px_-6px_rgba(0,0,0,0.1)] px-8 py-4 flex items-center gap-6 z-50">
      <Button
        onClick={onExplain}
        disabled={disabled || isLoading}
        variant="default"
        className="flex-1"
      >
        {isLoading ? (
          <>
            <Loader2 className="w-5 h-5 animate-spin" />
            Loading...
          </>
        ) : (
          <>
            <Sparkles className="w-5 h-5" />
            Explain this
          </>
        )}
      </Button>

      <button
        onClick={onBookmark}
        disabled={bookmarkDisabled}
        className="w-10 h-10 rounded-[12px] flex items-center justify-center bg-white text-[#0F172A] border border-[#F1F5F9] disabled:opacity-50 transition-colors"
        aria-label="Bookmark"
      >
        <Bookmark className="w-5 h-5" />
      </button>
    </div>
  )
}
