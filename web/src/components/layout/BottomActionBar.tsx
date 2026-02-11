import { Button } from '@/components/ui/button'
import { Sparkles, Loader2 } from 'lucide-react'

interface BottomActionBarProps {
  disabled?: boolean
  isLoading?: boolean
  onExplain?: () => void
}

export default function BottomActionBar({
  disabled = false,
  isLoading = false,
  onExplain,
}: BottomActionBarProps) {
  return (
    <div className="fixed bottom-8 left-1/2 -translate-x-1/2 w-[348px] h-[72px] bg-white border border-[#F1F5F9] rounded-[16px] shadow-[0_20px_25px_-5px_rgba(0,0,0,0.1),0_8px_10px_-6px_rgba(0,0,0,0.1)] px-8 py-4 flex items-center z-50">
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
    </div>
  )
}
