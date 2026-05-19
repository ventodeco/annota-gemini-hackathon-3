import type { NuanceData } from './types'

export interface FormattedNuanceData {
  translation: string
  explanation: string
  whenToUse: string
  alternativeMeanings: string
  pronunciation: string
}

export function formatNuanceData(nuance: NuanceData): FormattedNuanceData {
  const kana = nuance.pronunciation?.kana
  const romaji = nuance.pronunciation?.romaji

  return {
    translation: nuance.translation || nuance.meaning,
    explanation: nuance.contextualExplanation || nuance.meaning,
    whenToUse: nuance.whenToUse || nuance.usageTiming,
    alternativeMeanings: nuance.alternativeMeanings || nuance.alternativeMeaning,
    pronunciation: formatPronunciation(kana, romaji),
  }
}

function formatPronunciation(kana?: string, romaji?: string): string {
  if (kana && romaji) return `${kana} (${romaji})`
  return kana || romaji || ''
}
