import { useNavigate } from 'react-router-dom'
import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Camera, FileText, Image as ImageIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/contexts/useAuth'
import { createScan, trackEvent, uploadDocument } from '@/lib/api'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { AvatarMenu } from '@/components/layout/AvatarMenu'

export default function WelcomePage() {
  const navigate = useNavigate()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pdfInputRef = useRef<HTMLInputElement>(null)
  const { user } = useAuth()
  const [uploadError, setUploadError] = useState<string | null>(null)

  const uploadMutation = useMutation({
    mutationFn: createScan,
    onSuccess: (data) => {
      void trackEvent('reader_activation', { source: 'image' }).catch(() => undefined)
      navigate(`/loading/${data.scanId}`)
    },
    onError: (error: Error) => {
      setUploadError(error.message || 'Failed to upload image')
    },
  })

  const pdfUploadMutation = useMutation({
    mutationFn: uploadDocument,
    onSuccess: (data) => {
      void trackEvent('reader_activation', { source: 'pdf' }).catch(() => undefined)
      navigate(`/documents/${data.documentId}`)
    },
    onError: (error: Error) => {
      setUploadError(error.message || 'Failed to upload PDF')
    },
  })

  const isAnyPending = uploadMutation.isPending || pdfUploadMutation.isPending

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

  const handleUploadPDF = () => {
    pdfInputRef.current?.click()
  }

  const handlePDFFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (file.type !== 'application/pdf') {
      setUploadError('Please select a PDF file')
      return
    }

    setUploadError(null)
    pdfUploadMutation.mutate(file)
  }

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <div className="w-full flex justify-end absolute top-4 right-4">
          <AvatarMenu />
        </div>

        <h1 className="text-center text-gray-900 text-2xl font-semibold mb-2">
          Welcome to ANNOTA
        </h1>
        {user && (
          <p className="text-center text-gray-500 text-sm mb-6">
            Signed in as {user.email}
          </p>
        )}
        <p className="text-center text-gray-700 text-base mb-8">
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
            disabled={isAnyPending}
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <Camera className="w-5 h-5" />
            Take Photo
          </Button>
          <Button
            onClick={handleUploadGallery}
            variant="secondary"
            disabled={isAnyPending}
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <ImageIcon className="w-5 h-5" />
            {uploadMutation.isPending ? 'Uploading...' : 'Upload from Gallery'}
          </Button>
          <Button
            onClick={handleUploadPDF}
            variant="secondary"
            disabled={isAnyPending}
            className="w-[200px] min-h-[40px] h-auto rounded-full pt-[9.5px] pb-[9.5px] px-6 gap-2 text-[14px] font-medium font-roboto leading-none"
          >
            <FileText className="w-5 h-5" />
            {pdfUploadMutation.isPending ? 'Uploading...' : 'Upload PDF'}
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
      <input
        ref={pdfInputRef}
        type="file"
        accept="application/pdf"
        className="hidden"
        onChange={handlePDFFileChange}
      />
      <BottomNavigation />
    </div>
  )
}
