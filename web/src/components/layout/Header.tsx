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
    <header className="sticky top-0 z-10 border-b border-gray-200 bg-white">
      <div className="
        mx-auto flex max-w-md items-center justify-between px-4 py-3
      ">
        <Button
          variant="ghost"
          size="icon"
          onClick={handleBack}
          className="size-10 text-gray-900"
          aria-label="Go back"
        >
          <ChevronLeft className="size-6" />
        </Button>
        <h1 className="flex-1 text-center font-semibold text-gray-900">{title}</h1>
        <Button
          variant="ghost"
          size="icon"
          onClick={handleHome}
          className="size-10 text-gray-900"
          aria-label="Go home"
        >
          <Home className="size-6" />
        </Button>
      </div>
    </header>
  )
}
