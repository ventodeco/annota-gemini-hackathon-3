import { getScanImageUrl } from '@/lib/api'

interface ScanImageProps {
  imageUrl: string
  alt?: string
}

export default function ScanImage({ imageUrl, alt = 'Scanned image' }: ScanImageProps) {
  return (
    <div className="mb-6">
      <img
        src={getScanImageUrl(imageUrl)}
        alt={alt}
        className="w-full h-auto rounded-lg border border-gray-200"
      />
    </div>
  )
}
