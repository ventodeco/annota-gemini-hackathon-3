import { Button } from '@/components/ui/button'

interface PageListProps {
  pageCount: number
  onSelectPage: (pageNumber: number) => void
}

export default function PageList({ pageCount, onSelectPage }: PageListProps) {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: pageCount }, (_, i) => i + 1).map((pageNum) => (
        <Button
          key={pageNum}
          variant="ghost"
          className="w-full justify-start text-left p-4 text-base"
          onClick={() => onSelectPage(pageNum)}
        >
          Page {pageNum}
        </Button>
      ))}
    </div>
  )
}
