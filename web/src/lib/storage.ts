import type { Annotation } from './types'

const STORAGE_KEY = 'savedAnnotations'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isNuanceData(value: unknown): value is Annotation['nuance_data'] {
  if (!isRecord(value)) return false
  return (
    typeof value.meaning === 'string' &&
    typeof value.usageExample === 'string' &&
    typeof value.usageTiming === 'string' &&
    typeof value.wordBreakdown === 'string' &&
    typeof value.alternativeMeaning === 'string'
  )
}

function isAnnotation(value: unknown): value is Annotation {
  if (!isRecord(value)) return false
  return (
    typeof value.id === 'number' &&
    typeof value.user_id === 'number' &&
    typeof value.highlighted_text === 'string' &&
    isNuanceData(value.nuance_data) &&
    typeof value.is_bookmarked === 'boolean' &&
    typeof value.created_at === 'string' &&
    (value.scan_id === undefined || typeof value.scan_id === 'number') &&
    (value.context_text === undefined || typeof value.context_text === 'string')
  )
}

function getAllSavedAnnotations(): Record<string, Annotation> {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (!stored) {
      return {}
    }
    const parsed: unknown = JSON.parse(stored)
    if (!isRecord(parsed)) {
      return {}
    }
    const annotations: Record<string, Annotation> = {}
    for (const [key, value] of Object.entries(parsed)) {
      if (isAnnotation(value)) {
        annotations[key] = value
      }
    }
    return annotations
  } catch (error) {
    console.error('Failed to load saved annotations:', error)
    return {}
  }
}

export function saveAnnotation(annotation: Annotation): void {
  try {
    if (!annotation?.id) {
      throw new Error('Annotation must have an ID')
    }

    const saved = getAllSavedAnnotations()
    saved[annotation.id] = annotation
    localStorage.setItem(STORAGE_KEY, JSON.stringify(saved))
  } catch (error) {
    if (error instanceof DOMException && error.name === 'QuotaExceededError') {
      throw new Error('Storage quota exceeded. Please free up some space.')
    }
    throw new Error('Failed to save annotation to localStorage')
  }
}

export function isAnnotationSaved(annotationId: string): boolean {
  if (!annotationId) {
    return false
  }
  const saved = getAllSavedAnnotations()
  return annotationId in saved
}

export function loadAnnotation(annotationId: string): Annotation | null {
  if (!annotationId) {
    return null
  }
  const saved = getAllSavedAnnotations()
  return saved[annotationId] || null
}

export function getAllSavedAnnotationsList(): Annotation[] {
  const saved = getAllSavedAnnotations()
  return Object.values(saved)
}

export function removeAnnotation(annotationId: string): void {
  if (!annotationId) {
    return
  }
  try {
    const saved = getAllSavedAnnotations()
    delete saved[annotationId]
    localStorage.setItem(STORAGE_KEY, JSON.stringify(saved))
  } catch (error) {
    console.error('Failed to remove annotation:', error)
  }
}
