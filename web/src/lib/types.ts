export interface Scan {
  id: string
  sessionID: string
  userID?: string
  source: string
  status: 'uploaded' | 'ocr_done' | 'failed' | 'failed_overloaded' | 'failed_auth'
  createdAt: string
}

export interface OCRResult {
  id: string
  scanID: string
  model: string
  language?: string
  rawText: string
  structuredJSON?: string
  promptVersion: string
  createdAt: string
}

export interface Annotation {
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

export interface CreateScanResponse {
  scanID: string
  status: string
  createdAt: string
}

export interface GetScanResponse {
  scan: Scan
  ocrResult: OCRResult | null
  status: string
}

export interface AnnotateRequest {
  selectedText: string
}

export type AnnotateResponse = Annotation
