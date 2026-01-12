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
        <h1 className="text-3xl font-bold text-gray-900 text-center mb-4">
          Welcome to ANNOTA
        </h1>
        <p className="text-center text-gray-700 mb-8 text-base leading-relaxed">
          You no longer need to worry about learning a new language!
        </p>
        <div className="w-full max-w-sm flex flex-col gap-4">
          <Button
            onClick={handleTakePhoto}
            className="w-full bg-gray-900 text-white hover:bg-gray-800 rounded-full px-6 py-4 h-auto text-base font-medium"
          >
            <Camera className="w-5 h-5 mr-2" />
            Take Photo
          </Button>
          <Button
            onClick={handleUploadGallery}
            variant="outline"
            className="w-full border-2 border-gray-900 text-gray-900 hover:bg-gray-50 rounded-full px-6 py-4 h-auto text-base font-medium"
          >
            <ImageIcon className="w-5 h-5 mr-2" />
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
