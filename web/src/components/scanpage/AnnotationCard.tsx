import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Annotation } from '@/lib/types'

interface AnnotationCardProps {
  annotation: Annotation
}

export default function AnnotationCard({ annotation }: AnnotationCardProps) {
  return (
    <Card className="mb-6">
      <CardHeader>
        <CardTitle>Annotation</CardTitle>
        <CardDescription>Selected text: &quot;{annotation.highlighted_text}&quot;</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <h3 className="font-semibold mb-2">Meaning</h3>
          <p className="text-gray-700">{annotation.nuance_data.meaning}</p>
        </div>
        
        <div>
          <h3 className="font-semibold mb-2">Usage Example</h3>
          <p className="text-gray-700">{annotation.nuance_data.usageExample}</p>
        </div>
        
        <div>
          <h3 className="font-semibold mb-2">When to Use</h3>
          <p className="text-gray-700">{annotation.nuance_data.usageTiming}</p>
        </div>
        
        <div>
          <h3 className="font-semibold mb-2">Word Breakdown</h3>
          <p className="text-gray-700">{annotation.nuance_data.wordBreakdown}</p>
        </div>
        
        {annotation.nuance_data.alternativeMeaning && (
          <div>
            <h3 className="font-semibold mb-2">Alternative Meanings</h3>
            <p className="text-gray-700">{annotation.nuance_data.alternativeMeaning}</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
