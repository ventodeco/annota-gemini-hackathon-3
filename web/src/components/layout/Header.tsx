import { useNavigate } from 'react-router-dom'
import { ChevronLeft, Home } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface HeaderProps {
  title: string
  onBack?: () => void
}

export default function Header({ title, onBack }: HeaderProps) {
  const navigate = useNavigate()

  const handleBack = () => {
    if (onBack) {
      onBack()
    } else {
      navigate(-1)
    }
  }

  const handleHome = () => {
    navigate('/welcome')
  }

  return (
    <header className="sticky top-0 z-10 bg-white border-b border-gray-200">
      <div className="flex items-center justify-between px-4 py-3 max-w-md mx-auto">
        <Button
          variant="ghost"
          size="icon"
          onClick={handleBack}
          className="h-10 w-10 text-gray-900"
          aria-label="Go back"
        >
          <ChevronLeft className="w-6 h-6" />
        </Button>
        <h1 className="flex-1 text-center font-semibold text-gray-900">{title}</h1>
        <Button
          variant="ghost"
          size="icon"
          onClick={handleHome}
          className="h-10 w-10 text-gray-900"
          aria-label="Go home"
        >
          <Home className="w-6 h-6" />
        </Button>
      </div>
    </header>
  )
}
