import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { createScan } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function UploadForm() {
  const navigate = useNavigate()
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)

  const uploadMutation = useMutation({
    mutationFn: createScan,
    onSuccess: (data) => {
      navigate(`/scans/${data.scanID}`)
    },
  })

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!file.type.startsWith('image/')) {
      alert('Please select an image file')
      return
    }

    setSelectedFile(file)

    const reader = new FileReader()
    reader.onloadend = () => {
      setPreview(reader.result as string)
    }
    reader.readAsDataURL(file)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedFile) {
      alert('Please select an image file')
      return
    }

    uploadMutation.mutate(selectedFile)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <Input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={handleFileChange}
          disabled={uploadMutation.isPending}
          className="w-full"
        />
      </div>

      {preview && (
        <div className="mt-4">
          <img
            src={preview}
            alt="Preview"
            className="h-auto max-w-full rounded-lg border border-gray-200"
          />
        </div>
      )}

      {uploadMutation.isError && (
        <Alert variant="destructive">
          <AlertDescription>
            {uploadMutation.error instanceof Error
              ? uploadMutation.error.message
              : 'Failed to upload image'}
          </AlertDescription>
        </Alert>
      )}

      <Button
        type="submit"
        disabled={!selectedFile || uploadMutation.isPending}
        className="w-full"
      >
        {uploadMutation.isPending ? 'Uploading...' : 'Upload Image'}
      </Button>
    </form>
  )
}
