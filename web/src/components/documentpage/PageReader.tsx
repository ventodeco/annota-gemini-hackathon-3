import PageNavigator from './PageNavigator'

interface PageReaderProps {
  text: string
  currentPage: number
  totalPages: number
  onPrev: () => void
  onNext: () => void
  onTextSelect: () => void
  isLoading: boolean
}

export default function PageReader({ text, currentPage, totalPages, onPrev, onNext, onTextSelect, isLoading }: PageReaderProps) {
  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div role="status" className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
      </div>
    )
  }

  return (
    <div className="flex flex-col flex-1">
      <div className="flex-1 overflow-y-auto">
        <div className="p-6">
          <p className="text-base leading-relaxed text-gray-900 whitespace-pre-wrap"
            onMouseUp={onTextSelect} onTouchEnd={onTextSelect}>
            {text}
          </p>
        </div>
      </div>
      <PageNavigator currentPage={currentPage} totalPages={totalPages} onPrev={onPrev} onNext={onNext} />
    </div>
  )
}
