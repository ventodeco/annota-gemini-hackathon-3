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
        <CardDescription>Selected text: &quot;{annotation.selectedText}&quot;</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <h3 className="font-semibold mb-2">Meaning</h3>
          <p className="text-gray-700">{annotation.meaning}</p>
        </div>
        
        <div>
          <h3 className="font-semibold mb-2">Usage Example</h3>
          <p className="text-gray-700">{annotation.usageExample}</p>
        </div>
        
        <div>
          <h3 className="font-semibold mb-2">When to Use</h3>
          <p className="text-gray-700">{annotation.whenToUse}</p>
        </div>
        
        <div>
          <h3 className="font-semibold mb-2">Word Breakdown</h3>
          <p className="text-gray-700">{annotation.wordBreakdown}</p>
        </div>
        
        {annotation.alternativeMeanings && (
          <div>
            <h3 className="font-semibold mb-2">Alternative Meanings</h3>
            <p className="text-gray-700">{annotation.alternativeMeanings}</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
