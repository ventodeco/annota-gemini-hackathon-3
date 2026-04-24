import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { Annotation } from '@/lib/types'

interface AnnotationCardProps {
  annotation: Annotation
}

export default function AnnotationCard({ annotation }: AnnotationCardProps) {
  const nuance = annotation.nuance_data
  const translation = nuance.translation || nuance.meaning
  const explanation = nuance.contextualExplanation || ''
  const whenToUse = nuance.whenToUse || nuance.usageTiming
  const alternativeMeanings = nuance.alternativeMeanings || nuance.alternativeMeaning
  const pronunciation = nuance.pronunciation?.kana
    ? `${nuance.pronunciation.kana}${nuance.pronunciation.romaji ? ` (${nuance.pronunciation.romaji})` : ''}`
    : ''

  return (
    <Card className="mb-6">
      <CardHeader>
        <CardTitle>Annotation</CardTitle>
        <CardDescription>Selected text: &quot;{annotation.highlighted_text}&quot;</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <h3 className="font-semibold mb-2">Meaning</h3>
          <p className="text-gray-700">{translation}</p>
        </div>

        {explanation && explanation !== translation && (
          <div>
            <h3 className="font-semibold mb-2">Explanation</h3>
            <p className="text-gray-700">{explanation}</p>
          </div>
        )}

        {pronunciation && (
          <div>
            <h3 className="font-semibold mb-2">Pronunciation</h3>
            <p className="text-gray-700">{pronunciation}</p>
          </div>
        )}

        <div>
          <h3 className="font-semibold mb-2">Usage Example</h3>
          <p className="text-gray-700">{nuance.usageExample}</p>
        </div>

        <div>
          <h3 className="font-semibold mb-2">When to Use</h3>
          <p className="text-gray-700">{whenToUse}</p>
        </div>

        <div>
          <h3 className="font-semibold mb-2">Word Breakdown</h3>
          <p className="text-gray-700">{nuance.wordBreakdown}</p>
        </div>

        {alternativeMeanings && (
          <div>
            <h3 className="font-semibold mb-2">Alternative Meanings</h3>
            <p className="text-gray-700">{alternativeMeanings}</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
