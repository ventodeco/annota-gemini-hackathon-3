import type {
  // Auth types
  TokenResponse,
  // User types
  GetUserProfileResponse,
  UpdateUserPreferencesRequest,
  GetLanguagesResponse,
  // Scan types
  CreateScanResponse,
  GetScansResponse,
  Scan,
  // AI types
  AnalyzeRequest,
  AnalyzeResponse,
  SynthesizeSpeechRequest,
  // Annotation types
  CreateAnnotationRequest,
  CreateAnnotationResponse,
  GetAnnotationsResponse,
  AnnotationDetail,
  // Document types
  Document,
  GetDocumentsResponse,
  UploadDocumentResponse,
  DocumentPageResponse,
  CreateScanFromPageResponse,
} from './types'
import { logger } from './logger'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
const AUTH_TOKEN_KEY = 'auth_token'

// ============================================================================
// Auth Helpers
// ============================================================================

export function getAuthToken(): string | null {
  return localStorage.getItem(AUTH_TOKEN_KEY)
}

export function setAuthToken(token: string): void {
  localStorage.setItem(AUTH_TOKEN_KEY, token)
}

export function clearAuthToken(): void {
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

function getAuthHeaders(): HeadersInit {
  const token = getAuthToken()
  return token ? { 'x-token': token } : {}
}

function isAuthError(status: number): boolean {
  return status === 401
}

async function throwIfNotOk(
  response: Response,
  method: string,
  url: string,
  startTime: number,
  errorTextFallback = 'An error occurred',
): Promise<void> {
  if (response.ok) return

  if (isAuthError(response.status)) {
    clearAuthToken()
    window.location.href = '/login'
  }
  const error = await response.text().catch(() => errorTextFallback)
  logger.apiCall(method, url, response.status, Date.now() - startTime, new Error(error))
  throw new Error(error)
}

async function handleResponse<T>(response: Response, method: string, url: string): Promise<T> {
  const startTime = Date.now()
  await throwIfNotOk(response, method, url, startTime)

  logger.apiCall(method, url, response.status, Date.now() - startTime)
  if (response.status === 204) {
    return undefined as T
  }
  return response.json()
}

async function handleBlobResponse(
  response: Response,
  method: string,
  url: string,
  errorTextFallback = 'An error occurred',
): Promise<Blob> {
  const startTime = Date.now()
  await throwIfNotOk(response, method, url, startTime, errorTextFallback)

  logger.apiCall(method, url, response.status, Date.now() - startTime)
  return response.blob()
}

function fetchWithAuth(url: string, options: RequestInit = {}): Promise<Response> {
  const headers: HeadersInit = {
    ...getAuthHeaders(),
    'Content-Type': 'application/json',
    ...options.headers,
  }
  return fetch(url, { ...options, headers })
}

// ============================================================================
// Auth API
// ============================================================================

export async function getGoogleAuthUrl(): Promise<{ ssoRedirection: string }> {
  const url = `${API_BASE_URL}/v1/auth/google/state`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  })
  return handleResponse(response, 'GET', url)
}

export async function exchangeGoogleCode(code: string, state: string): Promise<TokenResponse> {
  const url = `${API_BASE_URL}/v1/auth/google/callback`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ code, state }),
  })
  return handleResponse(response, 'POST', url)
}

// ============================================================================
// User API
// ============================================================================

export async function getUserProfile(): Promise<GetUserProfileResponse> {
  const url = `${API_BASE_URL}/v1/users/me`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function updateUserPreferences(
  preferences: UpdateUserPreferencesRequest,
): Promise<GetUserProfileResponse> {
  const url = `${API_BASE_URL}/v1/users/me`
  const response = await fetchWithAuth(url, {
    method: 'PATCH',
    body: JSON.stringify(preferences),
  })
  return handleResponse(response, 'PATCH', url)
}

export async function deleteAccount(): Promise<void> {
  const url = `${API_BASE_URL}/v1/users/me`
  const response = await fetchWithAuth(url, { method: 'DELETE' })
  return handleResponse(response, 'DELETE', url)
}

export async function trackEvent(name: string, properties: Record<string, unknown> = {}): Promise<void> {
  const url = `${API_BASE_URL}/v1/events`
  const response = await fetchWithAuth(url, {
    method: 'POST',
    body: JSON.stringify({ name, properties }),
  })
  const startTime = Date.now()
  await throwIfNotOk(response, 'POST', url, startTime)
  logger.apiCall('POST', url, response.status, Date.now() - startTime)
}

export async function getLanguages(): Promise<GetLanguagesResponse> {
  const url = `${API_BASE_URL}/v1/users/me/languages`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

// ============================================================================
// Scans API
// ============================================================================

export async function createScan(imageFile: File): Promise<CreateScanResponse> {
  const formData = new FormData()
  formData.append('image', imageFile)

  const url = `${API_BASE_URL}/v1/scans`
  logger.debug(`Uploading image: ${imageFile.name}, size: ${imageFile.size} bytes`)
  const response = await fetch(url, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: formData,
  })
  return handleResponse(response, 'POST', url)
}

export async function getScans(page = 1, size = 20): Promise<GetScansResponse> {
  const url = `${API_BASE_URL}/v1/scans?page=${page}&size=${size}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function getScan(scanId: number): Promise<Scan> {
  const url = `${API_BASE_URL}/v1/scans/${scanId}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function deleteScan(scanId: number): Promise<void> {
  const url = `${API_BASE_URL}/v1/scans/${scanId}`
  const response = await fetchWithAuth(url, { method: 'DELETE' })
  return handleResponse(response, 'DELETE', url)
}

// ============================================================================
// AI API
// ============================================================================

export async function analyzeText(request: AnalyzeRequest): Promise<AnalyzeResponse> {
  const url = `${API_BASE_URL}/v1/ai/analyze`
  const response = await fetchWithAuth(url, {
    method: 'POST',
    body: JSON.stringify(request),
  })
  return handleResponse(response, 'POST', url)
}

export async function synthesizeSpeech(request: SynthesizeSpeechRequest): Promise<Blob> {
  const url = `${API_BASE_URL}/v1/ai/speech`
  const response = await fetchWithAuth(url, {
    method: 'POST',
    body: JSON.stringify(request),
  })
  return handleBlobResponse(response, 'POST', url)
}

// ============================================================================
// Annotations API
// ============================================================================

export async function createAnnotation(
  request: CreateAnnotationRequest,
): Promise<CreateAnnotationResponse> {
  const url = `${API_BASE_URL}/v1/annotations`
  const response = await fetchWithAuth(url, {
    method: 'POST',
    body: JSON.stringify(request),
  })
  return handleResponse(response, 'POST', url)
}

export async function getAnnotations(
  page = 1,
  size = 20,
  scanId?: number,
  documentId?: number,
  pageNumber?: number,
): Promise<GetAnnotationsResponse> {
  const params = new URLSearchParams({
    page: String(page),
    size: String(size),
  })
  if (scanId !== undefined) {
    params.set('scanId', String(scanId))
  }
  if (documentId !== undefined) {
    params.set('documentId', String(documentId))
  }
  if (pageNumber !== undefined) {
    params.set('pageNumber', String(pageNumber))
  }

  const url = `${API_BASE_URL}/v1/annotations?${params.toString()}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function getAnnotation(annotationId: number): Promise<AnnotationDetail> {
  const url = `${API_BASE_URL}/v1/annotations/${annotationId}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function deleteAnnotation(annotationId: number): Promise<void> {
  const url = `${API_BASE_URL}/v1/annotations/${annotationId}`
  const response = await fetchWithAuth(url, { method: 'DELETE' })
  return handleResponse(response, 'DELETE', url)
}

// ============================================================================
// Documents API
// ============================================================================

export async function uploadDocument(pdfFile: File): Promise<UploadDocumentResponse> {
  const formData = new FormData()
  formData.append('file', pdfFile)

  const url = `${API_BASE_URL}/v1/documents`
  const response = await fetch(url, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: formData,
  })
  return handleResponse(response, 'POST', url)
}

export async function getDocuments(page = 1, size = 20): Promise<GetDocumentsResponse> {
  const url = `${API_BASE_URL}/v1/documents?page=${page}&size=${size}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function getDocument(documentId: number): Promise<Document> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function updateDocumentProgress(
  documentId: number,
  lastPageNumber: number,
): Promise<{ status: 'saved' }> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}/progress`
  const response = await fetchWithAuth(url, {
    method: 'PATCH',
    body: JSON.stringify({ lastPageNumber }),
  })
  return handleResponse(response, 'PATCH', url)
}

export async function deleteDocument(documentId: number): Promise<void> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}`
  const response = await fetchWithAuth(url, { method: 'DELETE' })
  return handleResponse(response, 'DELETE', url)
}

export async function getDocumentPage(
  documentId: number,
  pageNumber: number,
): Promise<DocumentPageResponse> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}/pages/${pageNumber}`
  const response = await fetchWithAuth(url)
  return handleResponse(response, 'GET', url)
}

export async function createScanFromPage(
  documentId: number,
  pageNumber: number,
): Promise<CreateScanFromPageResponse> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}/pages/${pageNumber}/scan`
  const response = await fetchWithAuth(url, { method: 'POST' })
  return handleResponse(response, 'POST', url)
}

export async function getDocumentFile(documentId: number): Promise<Blob> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}/file`
  const response = await fetch(url, {
    method: 'GET',
    headers: getAuthHeaders(),
  })
  return handleBlobResponse(response, 'GET', url, 'Failed to fetch PDF file')
}

// ============================================================================
// Utility Functions
// ============================================================================

export function getScanImageUrl(imageUrl: string | undefined): string {
  if (!imageUrl) return ''
  if (imageUrl.startsWith('/')) {
    return `${API_BASE_URL}${imageUrl}`
  }
  return imageUrl
}

export async function getScanImageBlob(imageUrl: string): Promise<Blob> {
  const url = getScanImageUrl(imageUrl)
  const response = await fetch(url, {
    method: 'GET',
    headers: getAuthHeaders(),
  })
  return handleBlobResponse(response, 'GET', url, 'Failed to fetch scan image')
}

export function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
