import { Link } from 'react-router-dom'
import { Bookmark } from 'lucide-react'

export function EmptyHistory() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="
        mb-4 flex size-16 items-center justify-center rounded-full bg-gray-100
      ">
        <Bookmark className="size-8 text-gray-400" aria-label="bookmark icon" />
      </div>
      <h3 className="mb-2 text-lg font-medium text-gray-900">
        No saved annotations
      </h3>
      <p className="mb-6 max-w-xs text-sm text-gray-500">
        When you save annotations from your scans, they will appear here for easy access.
      </p>
      <Link
        to="/welcome"
        className="
          rounded-lg bg-[#0F172A] px-6 py-3 text-sm font-medium text-white
          transition-colors
          hover:bg-[#1E293B]
        "
      >
        Start Scanning
      </Link>
    </div>
  )
}
