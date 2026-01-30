import { useNavigate } from 'react-router-dom'
import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Camera, Image as ImageIcon, LogOut } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/contexts/useAuth'
import { createScan } from '@/lib/api'
import BottomNavigation from '@/components/layout/BottomNavigation'

export default function WelcomePage() {
  const navigate = useNavigate()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { user, logout } = useAuth()
  const [uploadError, setUploadError] = useState<string | null>(null)

  const uploadMutation = useMutation({
    mutationFn: createScan,
    onSuccess: (data) => {
      navigate(`/scans/${data.scanId}`)
    },
    onError: (error: Error) => {
      setUploadError(error.message || 'Failed to upload image')
    },
  })

  const handleTakePhoto = () => {
    navigate('/camera')
  }

  const handleUploadGallery = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!file.type.startsWith('image/')) {
      setUploadError('Please select an image file')
      return
    }

    setUploadError(null)
    uploadMutation.mutate(file)
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <div className="w-full flex justify-end absolute top-4 right-4">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleLogout}
            className="text-gray-500"
          >
            <LogOut className="w-4 h-4 mr-2" />
            Logout
          </Button>
        </div>

        <h1 className="font-roboto font-semibold text-[16px] leading-normal tracking-normal text-center text-gray-900 mb-2 align-middle">
          Welcome to ANNOTA
        </h1>
        {user && (
          <p className="font-roboto font-normal text-[14px] leading-normal tracking-normal text-center text-gray-500 mb-6 align-middle">
            Signed in as {user.email}
          </p>
        )}
        <p className="font-roboto font-normal text-[16px] leading-normal tracking-normal text-center text-gray-700 mb-8 align-middle">
          You no longer need to worry about learning a new language!
        </p>

        {uploadError && (
          <div className="mb-4 p-3 bg-red-50 text-red-600 text-sm rounded-lg">
            {uploadError}
          </div>
        )}

        <div className="w-full flex flex-col items-center gap-4">
          <Button
            onClick={handleTakePhoto}
            variant="default"
            disabled={uploadMutation.isPending}
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <Camera className="w-5 h-5" />
            Take Photo
          </Button>
          <Button
            onClick={handleUploadGallery}
            variant="secondary"
            disabled={uploadMutation.isPending}
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <ImageIcon className="w-5 h-5" />
            {uploadMutation.isPending ? 'Uploading...' : 'Upload from Gallery'}
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
