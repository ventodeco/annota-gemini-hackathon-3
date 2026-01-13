import { useNavigate } from 'react-router-dom'
import { useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Camera, Image as ImageIcon } from 'lucide-react'
import BottomNavigation from '@/components/layout/BottomNavigation'

export default function WelcomePage() {
  const navigate = useNavigate()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleTakePhoto = () => {
    navigate('/camera')
  }

  const handleUploadGallery = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      if (file.type.startsWith('image/')) {
        const blobUrl = URL.createObjectURL(file)
        sessionStorage.setItem('pendingImage', JSON.stringify({
          blob: blobUrl,
          type: file.type,
          source: 'upload',
        }))
        navigate('/loading')
      } else {
        alert('Please select an image file')
      }
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-white pb-20">
      <div className="flex flex-1 flex-col items-center justify-center p-6">
        <h1 className="
          mb-4 text-center align-middle text-[16px] leading-normal font-semibold
          tracking-normal text-gray-900
        ">
          Welcome to ANNOTA
        </h1>
        <p className="
          mb-8 text-center align-middle text-[16px] leading-normal font-normal
          tracking-normal text-gray-700
        ">
        You no longer need to worry about learning a new language!
        </p>
        <div className="flex w-full flex-col items-center gap-4">
          <Button
            onClick={handleTakePhoto}
            variant="default"
            className="
              h-auto min-h-[40px] w-[200px] gap-2 rounded-full px-6 py-[9.5px]
              text-[14px] leading-none font-medium
            "
          >
            <Camera className="size-5" />
            Take Photo
          </Button>
          <Button
            onClick={handleUploadGallery}
            variant="secondary"
            className="
              h-auto min-h-[40px] w-[200px] gap-2 rounded-full px-6 py-[9.5px]
              text-[14px] leading-none font-medium
            "
          >
            <ImageIcon className="size-5" />
            Upload for Gallery
          </Button>
        </div>
      </div>
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleFileChange}
      />
      <BottomNavigation />
    </div>
  )
}
