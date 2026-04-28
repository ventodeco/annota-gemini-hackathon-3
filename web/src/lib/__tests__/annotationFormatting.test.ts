import { describe, expect, it } from 'vitest'
import { formatNuanceData } from '../annotationFormatting'
import type { NuanceData } from '../types'

describe('formatNuanceData', () => {
  it('normalizes fallback fields for annotation UI', () => {
    const nuance: NuanceData = {
      translation: 'translation',
      contextualExplanation: 'contextual explanation',
      meaning: 'meaning',
      usageExample: 'example',
      whenToUse: 'when to use',
      usageTiming: 'timing',
      wordBreakdown: 'breakdown',
      alternativeMeanings: 'alternatives',
      alternativeMeaning: 'alternative',
      pronunciation: {
        kana: 'かな',
        romaji: 'kana',
      },
    }

    expect(formatNuanceData(nuance)).toEqual({
      translation: 'translation',
      explanation: 'contextual explanation',
      whenToUse: 'when to use',
      alternativeMeanings: 'alternatives',
      pronunciation: 'かな (kana)',
    })
  })

  it('falls back to legacy fields when newer fields are absent', () => {
    const nuance: NuanceData = {
      meaning: 'meaning',
      usageExample: 'example',
      usageTiming: 'timing',
      wordBreakdown: 'breakdown',
      alternativeMeaning: 'alternative',
      pronunciation: {
        romaji: 'kana',
      },
    }

    expect(formatNuanceData(nuance)).toEqual({
      translation: 'meaning',
      explanation: 'meaning',
      whenToUse: 'timing',
      alternativeMeanings: 'alternative',
      pronunciation: 'kana',
    })
  })
})
