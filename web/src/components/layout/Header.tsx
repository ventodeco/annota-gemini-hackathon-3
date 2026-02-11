import { useNavigate } from 'react-router-dom'
import { Bookmark, ChevronLeft, Home } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface HeaderProps {
  title: string
  onBack?: () => void
  rightAction?: 'home' | 'bookmark'
  rightActionTo?: string
  backFallbackTo?: string
  rightSlot?: React.ReactNode
}

export default function Header({
  title,
  onBack,
  rightAction = 'home',
  rightActionTo,
  backFallbackTo = '/welcome',
  rightSlot,
}: HeaderProps) {
  const navigate = useNavigate()

  const handleBack = () => {
    if (onBack) {
      onBack()
      return
    }

    const historyState = window.history.state as { idx?: number } | null
    const canGoBack =
      typeof historyState?.idx === 'number' ? historyState.idx > 0 : window.history.length > 1

    if (canGoBack) {
      navigate(-1)
      return
    }

    navigate(backFallbackTo, { replace: true })
  }

  const handleRightAction = () => {
    if (rightAction === 'bookmark') {
      navigate(rightActionTo ?? '/history')
      return
    }

    navigate(rightActionTo ?? '/welcome')
  }

  return (
    <header className="sticky top-0 z-10 bg-white border-b border-gray-200">
      <div className="flex items-center justify-between px-4 py-3 max-w-md mx-auto">
        <Button
          variant="ghost"
          size="icon"
          onClick={handleBack}
          className="h-10 w-10 rounded-full border border-gray-200 text-gray-900"
          aria-label="Go back"
        >
          <ChevronLeft className="w-6 h-6" />
        </Button>
        <h1 className="flex-1 text-center font-semibold text-gray-900">{title}</h1>
        {rightSlot != null ? rightSlot : (
          <Button
            variant="ghost"
            size="icon"
            onClick={handleRightAction}
            className="h-10 w-10 text-gray-900"
            aria-label={rightAction === 'bookmark' ? 'Go to bookmarks' : 'Go home'}
          >
            {rightAction === 'bookmark' ? (
              <Bookmark className="w-6 h-6" />
            ) : (
              <Home className="w-6 h-6" />
            )}
          </Button>
        )}
      </div>
    </header>
  )
}
