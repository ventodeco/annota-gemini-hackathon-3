import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { formatNuanceData } from '@/lib/annotationFormatting'
import type { Annotation } from '@/lib/types'

interface AnnotationCardProps {
  annotation: Annotation
}

export default function AnnotationCard({ annotation }: AnnotationCardProps) {
  const nuance = annotation.nuance_data
  const formattedNuance = formatNuanceData(nuance)

  return (
    <Card className="mb-6">
      <CardHeader>
        <CardTitle>Annotation</CardTitle>
        <CardDescription>Selected text: &quot;{annotation.highlighted_text}&quot;</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <h3 className="font-semibold mb-2">Meaning</h3>
          <p className="text-gray-700">{formattedNuance.translation}</p>
        </div>

        {formattedNuance.explanation !== formattedNuance.translation && (
          <div>
            <h3 className="font-semibold mb-2">Explanation</h3>
            <p className="text-gray-700">{formattedNuance.explanation}</p>
          </div>
        )}

        {formattedNuance.pronunciation && (
          <div>
            <h3 className="font-semibold mb-2">Pronunciation</h3>
            <p className="text-gray-700">{formattedNuance.pronunciation}</p>
          </div>
        )}

        <div>
          <h3 className="font-semibold mb-2">Usage Example</h3>
          <p className="text-gray-700">{nuance.usageExample}</p>
        </div>

        <div>
          <h3 className="font-semibold mb-2">When to Use</h3>
          <p className="text-gray-700">{formattedNuance.whenToUse}</p>
        </div>

        <div>
          <h3 className="font-semibold mb-2">Word Breakdown</h3>
          <p className="text-gray-700">{nuance.wordBreakdown}</p>
        </div>

        {formattedNuance.alternativeMeanings && (
          <div>
            <h3 className="font-semibold mb-2">Alternative Meanings</h3>
            <p className="text-gray-700">{formattedNuance.alternativeMeanings}</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
