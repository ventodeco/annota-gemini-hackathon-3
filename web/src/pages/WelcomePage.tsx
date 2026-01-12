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
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <h1 className="font-roboto font-semibold text-[16px] leading-normal tracking-normal text-center text-gray-900 mb-4 align-middle">
          Welcome to ANNOTA
        </h1>
        <p className="font-roboto font-normal text-[16px] leading-normal tracking-normal text-center text-gray-700 mb-8 align-middle">
        You no longer need to worry about learning a new language!
        </p>
        <div className="w-full flex flex-col items-center gap-4">
          <Button
            onClick={handleTakePhoto}
            variant="default"
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <Camera className="w-5 h-5" />
            Take Photo
          </Button>
          <Button
            onClick={handleUploadGallery}
            variant="secondary"
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <ImageIcon className="w-5 h-5" />
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
