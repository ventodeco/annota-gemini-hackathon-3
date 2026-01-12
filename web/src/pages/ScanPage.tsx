import { useParams } from 'react-router-dom'
import { useEffect, useState } from 'react'
import Header from '@/components/layout/Header'
import BottomActionBar from '@/components/layout/BottomActionBar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { getMockScan, getMockImageUrl } from '@/lib/mockData'
import type { GetScanResponse } from '@/lib/types'
import { useToast } from '@/hooks/use-toast'

export default function ScanPage() {
  const { id } = useParams<{ id: string }>()
  const [scanData, setScanData] = useState<GetScanResponse | null>(null)
  const [imageUrl, setImageUrl] = useState<string | null>(null)
  const { toast } = useToast()

  useEffect(() => {
    if (id) {
      const mockScan = getMockScan(id)
      if (mockScan) {
        setScanData(mockScan)
        const url = getMockImageUrl(id)
        if (url) {
          setImageUrl(url)
        }
      }
    }
  }, [id])

  const handleExplain = () => {
    toast({
      title: "Text saved",
      description: "Saved text to Highlight and will receive an explanation",
      duration: 3000,
    })
  }

  if (!scanData || !scanData.ocrResult) {
    return (
      <div className="min-h-screen bg-white flex flex-col">
        <Header title="Scan Result" />
        <div className="flex-1 flex items-center justify-center p-6">
          <p className="text-gray-600">Scan not found</p>
        </div>
      </div>
    )
  }

  const { ocrResult } = scanData

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <Header title="Scan Result" />
      <ScrollArea className="flex-1">
        <div className="p-6">
          {imageUrl && (
            <img
              src={imageUrl}
              alt="Scanned document"
              className="w-full mb-6 rounded-lg"
            />
          )}
          <p className="text-base leading-relaxed text-gray-900 whitespace-pre-wrap">
            {ocrResult.rawText}
          </p>
        </div>
      </ScrollArea>
      <BottomActionBar onExplain={handleExplain} />
    </div>
  )
}
