// User Types
export interface User {
  id: number
  email: string
  provider: 'google' | 'apple' | 'github'
  avatar_url?: string
  preferred_language: 'ID' | 'JP' | 'EN'
  created_at: string
  updated_at: string
}

// Auth Types
export interface TokenResponse {
  token: string
  expirySeconds: number
  expiresAt: string
}

// Scan Types (new schema)
export interface Scan {
  id: number
  user_id: number
  image_url: string
  full_ocr_text?: string
  detected_language?: string
  created_at: string
}

export interface CreateScanResponse {
  scanId: number
  fullText?: string
  imageUrl: string
}

export interface GetScanListItem {
  id: number
  imageUrl: string
  detectedLanguage?: string
  createdAt: string
}

export interface GetScansResponse {
  data: GetScanListItem[]
  meta: {
    currentPage: number
    pageSize: number
    nextPage?: number
    previousPage?: number
  }
}

// Annotation Types
export interface NuanceData {
  meaning: string
  usageExample: string
  usageTiming: string
  wordBreakdown: string
  alternativeMeaning: string
}

export interface Annotation {
  id: number
  user_id: number
  scan_id?: number
  highlighted_text: string
  context_text?: string
  nuance_data: NuanceData
  is_bookmarked: boolean
  created_at: string
}

export interface AnnotationListItem {
  id: number
  highlightedText: string
  nuanceSummary: string
  createdAt: string
}

export interface GetAnnotationsResponse {
  data: AnnotationListItem[]
  meta: {
    currentPage: number
    pageSize: number
    nextPage?: number
    previousPage?: number
  }
}

export interface CreateAnnotationRequest {
  scanId: number
  highlightedText: string
  contextText?: string
  nuanceData: NuanceData
}

export interface CreateAnnotationResponse {
  annotationId: number
  status: 'saved'
}

// AI Types
export interface AnalyzeRequest {
  textToAnalyze: string
  context: string
}

export type AnalyzeResponse = NuanceData

// Language Types
export interface Language {
  caption: string
  imageUrl: string
}

export interface GetLanguagesResponse {
  languages: Language[]
}

// User Preference Types
export interface UpdateUserPreferencesRequest {
  preferredLanguage: 'ID' | 'JP' | 'EN'
}

export interface GetUserProfileResponse {
  preferredLanguage: string
}

// Pagination
export interface PaginationMeta {
  currentPage: number
  pageSize: number
  nextPage?: number
  previousPage?: number
}

// Legacy Types (for reference during migration)
export interface LegacyScan {
  id: string
  sessionID: string
  userID?: string
  source: string
  status: 'uploaded' | 'ocr_done' | 'failed' | 'failed_overloaded' | 'failed_auth'
  createdAt: string
}

export interface LegacyOCRResult {
  id: string
  scanID: string
  model: string
  language?: string
  rawText: string
  structuredJSON?: string
  promptVersion: string
  createdAt: string
}

export interface LegacyAnnotation {
  id: string
  scanID: string
  ocrResultID: string
  selectedText: string
  selectionStart?: number
  selectionEnd?: number
  context: string
  meaning: string
  usageExample: string
  whenToUse: string
  wordBreakdown: string
  alternativeMeanings: string
  model: string
  promptVersion: string
  createdAt: string
}
