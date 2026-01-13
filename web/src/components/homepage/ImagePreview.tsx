interface ImagePreviewProps {
  src: string
  alt?: string
}

export default function ImagePreview({ src, alt = 'Preview' }: ImagePreviewProps) {
  return (
    <div className="mt-4">
      <img
        src={src}
        alt={alt}
        className="h-auto max-w-full rounded-lg border border-gray-200"
      />
    </div>
  )
}
