import type { Annotation } from './types'

const STORAGE_KEY = 'savedAnnotations'

function getAllSavedAnnotations(): Record<string, Annotation> {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (!stored) {
      return {}
    }
    return JSON.parse(stored) as Record<string, Annotation>
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
