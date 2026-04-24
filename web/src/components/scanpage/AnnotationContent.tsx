import { useEffect, useRef } from 'react'
import type { Annotation } from '@/lib/types'
import { HighlightedTextSection } from './HighlightedTextSection'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { NuanceData } from '@/lib/types'

interface AnnotationContentProps {
  annotation: Annotation
  drawerState?: 'collapsed' | 'expanded' | 'closed'
}

export function AnnotationContent({ annotation, drawerState }: AnnotationContentProps) {
  const scrollAreaRef = useRef<HTMLDivElement>(null)
  const renderWordBreakdown = (breakdown: string) => {
    const items = breakdown.split('\n').filter((line) => line.trim())
    return (
      <ul className="list-disc list-inside space-y-2">
        {items.map((item, index) => (
          <li key={index} className="text-base text-gray-900 leading-relaxed">
            {item.replace(/^•\s*/, '')}
          </li>
        ))}
      </ul>
    )
  }

  const renderSection = (title: string, content: string, isBullets = false) => (
    <div className="flex flex-col gap-4">
      <h3 className="font-semibold text-base leading-6 text-black">{title}</h3>
      {isBullets ? (
        renderWordBreakdown(content)
      ) : (
        <p className="text-base font-normal text-gray-900 leading-relaxed">{content}</p>
      )}
    </div>
  )

  const nuance = annotation.nuance_data
  const translation = nuance.translation || nuance.meaning
  const explanation = nuance.contextualExplanation || nuance.meaning
  const whenToUse = nuance.whenToUse || nuance.usageTiming
  const alternativeMeanings = nuance.alternativeMeanings || nuance.alternativeMeaning
  const pronunciation = formatPronunciation(nuance)

  useEffect(() => {
    if (scrollAreaRef.current) {
      scrollAreaRef.current.scrollTop = 0
    }
  }, [annotation, drawerState])

  return (
    <ScrollArea
      ref={scrollAreaRef}
      className="flex-1 pr-2 overscroll-contain min-h-0"
      style={{ overscrollBehavior: 'contain' }}
    >
      <div className="flex flex-col gap-6">
        <HighlightedTextSection text={annotation.highlighted_text} />
        {renderSection('Context', annotation.context_text || '')}
        {renderSection('Translation', translation)}
        {renderSection('Explanation', explanation)}
        {pronunciation && renderSection('Pronunciation', pronunciation)}
        {renderSection('Usage Example', annotation.nuance_data.usageExample)}
        {renderSection('When to Use', whenToUse)}
        {renderSection('Word Breakdown', annotation.nuance_data.wordBreakdown, true)}
        {renderSection('Alternative Meanings', alternativeMeanings)}
      </div>
    </ScrollArea>
  )
}

function formatPronunciation(nuance: NuanceData): string {
  const kana = nuance.pronunciation?.kana
  const romaji = nuance.pronunciation?.romaji
  if (kana && romaji) return `${kana} (${romaji})`
  return kana || romaji || ''
}
