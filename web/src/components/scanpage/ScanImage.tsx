import { getScanImageURL } from '@/lib/api'

interface ScanImageProps {
  scanID: string
  alt?: string
}

export default function ScanImage({ scanID, alt = 'Scanned image' }: ScanImageProps) {
  return (
    <div className="mb-6">
      <img
        src={getScanImageURL(scanID)}
        alt={alt}
        className="h-auto w-full rounded-lg border border-gray-200"
      />
    </div>
  )
}
