import { Home, FileText } from 'lucide-react'

export default function BottomNavigation() {
  return (
    <nav className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200 safe-area-inset-bottom">
      <div className="flex items-center justify-around px-4 py-3 max-w-md mx-auto">
        <div className="flex-1 flex justify-center">
          <span className="text-lg font-bold text-gray-900">ANNOTA</span>
        </div>
        <div className="flex-1 flex justify-center">
          <Home className="w-6 h-6 text-gray-900 fill-current" />
        </div>
        <div className="flex-1 flex justify-center">
          <FileText className="w-6 h-6 text-gray-900" />
        </div>
      </div>
    </nav>
  )
}
