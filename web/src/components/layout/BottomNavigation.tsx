import { Home, Newspaper } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import logo from '@/assets/logo.svg'

export default function BottomNavigation() {
  const location = useLocation()
  const isHome = location.pathname === '/welcome' || location.pathname === '/'
  const isHistory = location.pathname === '/scans-history'

  return (
    <nav className="fixed bottom-8 left-1/2 -translate-x-1/2 w-[348px] h-[72px] bg-white border border-[#F1F5F9] rounded-[16px] shadow-[0_20px_25px_-5px_rgba(0,0,0,0.1),0_8px_10px_-6px_rgba(0,0,0,0.1)] px-8 py-4 flex items-center gap-6 z-50">
      <div className="shrink-0">
        <img src={logo} alt="ANNOTA" className="w-[130px] h-[40px] p-2" />
      </div>
      
      <div className="w-px h-8 bg-[#F1F5F9]" />

      <div className="flex items-center gap-6 flex-1 justify-end">
        <Link 
          to="/welcome"
          className={`w-10 h-10 rounded-[12px] flex items-center justify-center transition-colors ${
            isHome ? 'bg-[#0F172A] text-white' : 'bg-white text-[#0F172A]'
          }`}
        >
          <Home className="w-5 h-5" />
        </Link>
        <Link 
          to="/scans-history"
          className={`w-10 h-10 rounded-[12px] flex items-center justify-center transition-colors ${
            isHistory ? 'bg-[#0F172A] text-white' : 'bg-white text-[#0F172A]'
          }`}
        >
          <Newspaper className="w-5 h-5" />
        </Link>
      </div>
    </nav>
  )
}
