import { Home, Newspaper } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import logo from '@/assets/logo.svg'

export default function BottomNavigation() {
  const location = useLocation()
  const isHome = location.pathname === '/welcome' || location.pathname === '/'
  const isHistory = location.pathname === '/history'

  return (
    <nav className="
      fixed bottom-8 left-1/2 z-50 flex h-[72px] w-[348px] -translate-x-1/2
      items-center gap-6 rounded-[16px] border border-[#F1F5F9] bg-white px-8
      py-4
      shadow-[0_20px_25px_-5px_rgba(0,0,0,0.1),0_8px_10px_-6px_rgba(0,0,0,0.1)]
    ">
      <div className="shrink-0">
        <img src={logo} alt="ANNOTA" className="h-[40px] w-[130px] p-2" />
      </div>
      
      <div className="h-8 w-px bg-[#F1F5F9]" />

      <div className="flex flex-1 items-center justify-end gap-6">
        <Link 
          to="/welcome"
          className={`
            flex size-10 items-center justify-center rounded-[12px]
            transition-colors
            ${
            isHome ? 'bg-[#0F172A] text-white' : 'bg-white text-[#0F172A]'
          }
          `}
        >
          <Home className="size-5" />
        </Link>
        <Link 
          to="/history"
          className={`
            flex size-10 items-center justify-center rounded-[12px]
            transition-colors
            ${
            isHistory ? 'bg-[#0F172A] text-white' : 'bg-white text-[#0F172A]'
          }
          `}
        >
          <Newspaper className="size-5" />
        </Link>
      </div>
    </nav>
  )
}
