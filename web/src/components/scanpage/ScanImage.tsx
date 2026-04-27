import { useEffect, useState } from 'react'
import { getScanImageBlob } from '@/lib/api'

interface ScanImageProps {
  imageUrl?: string
  alt?: string
}

interface BlobImageState {
  sourceUrl: string
  blobUrl: string
}

export default function ScanImage({ imageUrl, alt = 'Scanned image' }: ScanImageProps) {
  const [blobImage, setBlobImage] = useState<BlobImageState | null>(null)

  useEffect(() => {
    if (!imageUrl) {
      return
    }

    let objectUrl = ''
    let isActive = true

    getScanImageBlob(imageUrl)
      .then((blob) => {
        if (!isActive) return
        objectUrl = URL.createObjectURL(blob)
        setBlobImage({ sourceUrl: imageUrl, blobUrl: objectUrl })
      })
      .catch(() => {
        if (isActive) setBlobImage(null)
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

  if (!blobImage || blobImage.sourceUrl !== imageUrl) {
    return null
  }

  return (
    <div className="mb-6">
      <img
        src={blobImage.blobUrl}
        alt={alt}
        className="w-full h-auto rounded-lg border border-gray-200"
      />
    </div>
  )
}
