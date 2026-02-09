import { X } from 'lucide-react'

interface DrawerHeaderProps {
  onClose: () => void
  onCollapse?: () => void
  version?: number
  maxVersions?: number
}

export function DrawerHeader({
  onClose,
  onCollapse,
  version = 1,
  maxVersions = 2,
}: DrawerHeaderProps) {
  const handleTitleClick = () => {
    if (onCollapse) {
      onCollapse()
    }
  }

  return (
    <div className="flex items-center justify-between h-6 gap-2">
      <div 
        className="flex items-center gap-2 cursor-pointer"
        onClick={handleTitleClick}
      >
        <svg
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M12 21V7"
            stroke="#F97316"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M16 12L18 14L22 10"
            stroke="#F97316"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M22 6V4C22 3.73478 21.8946 3.48043 21.7071 3.29289C21.5196 3.10536 21.2652 3 21 3H16C14.9391 3 13.9217 3.42143 13.1716 4.17157C12.4214 4.92172 12 5.93913 12 7C12 5.93913 11.5786 4.92172 10.8284 4.17157C10.0783 3.42143 9.06087 3 8 3H3C2.73478 3 2.48043 3.10536 2.29289 3.29289C2.10536 3.48043 2 3.73478 2 4V17C2 17.2652 2.10536 17.5196 2.29289 17.7071C2.48043 17.8946 2.73478 18 3 18H9C9.79565 18 10.5587 18.3161 11.1213 18.8787C11.6839 19.4413 12 20.2044 12 21C12 20.2044 12.3161 19.4413 12.8787 18.8787C13.4413 18.3161 14.2044 18 15 18H21C21.2652 18 21.5196 17.8946 21.7071 17.7071C21.8946 17.5196 22 17.2652 22 17V15.7"
            stroke="#F97316"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <span className="font-semibold text-lg leading-6 text-gray-900">Annotation</span>
        <span className="font-normal text-base leading-6 text-gray-700">
          {`Version ${version}/${maxVersions}`}
        </span>
      </div>
      <button
        onClick={onClose}
        className="w-6 h-6 flex items-center justify-center rounded-full hover:bg-gray-100 transition-colors"
        aria-label="Close annotation"
      >
        <X className="w-5 h-5 text-gray-500" />
      </button>
    </div>
  )
}
