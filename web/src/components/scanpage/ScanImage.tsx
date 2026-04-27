import { useEffect, useState } from 'react'
import { getScanImageBlob } from '@/lib/api'

interface ScanImageProps {
  imageUrl?: string
  alt?: string
}

export default function ScanImage({ imageUrl, alt = 'Scanned image' }: ScanImageProps) {
  const [blobUrl, setBlobUrl] = useState<string>('')

  useEffect(() => {
    if (!imageUrl) {
      setBlobUrl('')
      return
    }

    let objectUrl = ''
    let isActive = true

    getScanImageBlob(imageUrl)
      .then((blob) => {
        if (!isActive) return
        objectUrl = URL.createObjectURL(blob)
        setBlobUrl(objectUrl)
      })
      .catch(() => {
        if (isActive) setBlobUrl('')
      })

    return () => {
      isActive = false
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl)
      }
    }
  }, [imageUrl])

  if (!imageUrl) {
    return null
  }

  if (!blobUrl) {
    return null
  }

  return (
    <div className="mb-6">
      <img
        src={blobUrl}
        alt={alt}
        className="w-full h-auto rounded-lg border border-gray-200"
      />
    </div>
  )
}
