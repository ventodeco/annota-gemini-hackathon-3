import { Button } from '@/components/ui/button'
import { Sparkles, Bookmark, Loader2 } from 'lucide-react'

interface BottomActionBarProps {
  disabled?: boolean
  isLoading?: boolean
  onExplain?: () => void
  onBookmark?: () => void
}

export default function BottomActionBar({
  disabled = false,
  isLoading = false,
  onExplain,
  onBookmark,
}: BottomActionBarProps) {
  return (
    <div className="
      fixed bottom-8 left-1/2 z-50 flex h-[72px] w-[348px] -translate-x-1/2
      items-center gap-6 rounded-[16px] border border-[#F1F5F9] bg-white px-8
      py-4
      shadow-[0_20px_25px_-5px_rgba(0,0,0,0.1),0_8px_10px_-6px_rgba(0,0,0,0.1)]
    ">
      <Button
        onClick={onExplain}
        disabled={disabled || isLoading}
        variant="default"
        className="flex-1"
      >
        {isLoading ? (
          <>
            <Loader2 className="size-5 animate-spin" />
            Loading...
          </>
        ) : (
          <>
            <Sparkles className="size-5" />
            Explain this
          </>
        )}
      </Button>

      <button
        onClick={onBookmark}
        disabled={disabled || isLoading}
        className="
          flex size-10 items-center justify-center rounded-[12px] border
          border-[#F1F5F9] bg-white text-[#0F172A] transition-colors
          disabled:opacity-50
        "
        aria-label="Bookmark"
      >
        <Bookmark className="size-5" />
      </button>
    </div>
  )
}
