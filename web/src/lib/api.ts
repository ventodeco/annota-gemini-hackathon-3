import { getToken } from '@clerk/react'
import type {
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
import { parseEntitlements, type Entitlements } from './entitlements'
import { logger } from './logger'
import { notifyUnauthorized } from './authSession'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

// ============================================================================
// Auth Helpers
// ============================================================================

/**
 * Error for a non-OK API response. `message` keeps the raw response body (the
 * historical behaviour callers display); `serverMessage` is the parsed JSON
 * `message` field when the API returned the standard error shape.
 */
export class ApiError extends Error {
  readonly status: number
  readonly serverMessage: string | null
  /** Stable machine-readable reason, e.g. `usage_quota_exceeded`. */
  readonly code: string | null
  /** Parsed JSON error body, when the API returned one. */
  readonly data: Record<string, unknown> | null
  /** Server request ID (`X-Request-ID`), shown to users as a support reference. */
  readonly requestId: string | null

  constructor(status: number, body: string, requestId: string | null = null) {
    super(body)
    this.name = 'ApiError'
    this.status = status
    this.requestId = requestId || null
    this.data = parseErrorBody(body)
    this.serverMessage = readString(this.data, 'message')
    this.code = readString(this.data, 'code')
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function parseErrorBody(body: string): Record<string, unknown> | null {
  try {
    const parsed: unknown = JSON.parse(body)
    return isRecord(parsed) ? parsed : null
  } catch {
    // Not JSON; there is no structured error.
    return null
  }
}

function readString(data: Record<string, unknown> | null, key: string): string | null {
  const value = data?.[key]
  return typeof value === 'string' ? value : null
}

// Session token each response was requested with, so a late 401 can be tied
// to the session that caused it rather than whoever is signed in now.
const responseTokens = new WeakMap<Response, string>()

interface SessionFetchOptions {
  /** Send a JSON Content-Type header (omit for FormData and blob reads). */
  json?: boolean
  /** Use this session token instead of the current one. */
  token?: string
}

/** Sends a request with a Clerk session token; Clerk refreshes it as needed. */
async function fetchWithSession(
  url: string,
  init: RequestInit = {},
  { json = true, token }: SessionFetchOptions = {},
): Promise<Response> {
  const sessionToken = token ?? (await getToken())
  const headers: HeadersInit = {
    ...(sessionToken ? { Authorization: `Bearer ${sessionToken}` } : {}),
    ...(json ? { 'Content-Type': 'application/json' } : {}),
    ...init.headers,
  }
  // Account data must come from the server every time: the browser HTTP cache
  // is keyed by URL only, so a stored response (including ones kept from
  // earlier builds) could be shown to another account or outlive deletion.
  const response = await fetch(url, { ...init, headers, cache: 'no-store' })
  if (sessionToken) {
    responseTokens.set(response, sessionToken)
  }
  return response
}

async function throwIfNotOk(
  response: Response,
  method: string,
  url: string,
  startTime: number,
  errorTextFallback = 'An error occurred',
): Promise<void> {
  if (response.ok) return

  const body = await response.text().catch(() => errorTextFallback)
  const error = new ApiError(response.status, body, response.headers.get('X-Request-ID'))
  logger.apiCall(method, url, response.status, Date.now() - startTime, error)
  if (response.status === 401) {
    notifyUnauthorized({ message: error.serverMessage ?? body, token: responseTokens.get(response) ?? null })
  }
  throw error
}

async function handleResponse<T>(response: Response, method: string, url: string): Promise<T> {
  const startTime = Date.now()
  await throwIfNotOk(response, method, url, startTime)

  logger.apiCall(method, url, response.status, Date.now() - startTime)
  if (response.status === 204) {
    throw new Error(`Expected JSON response for ${method} ${url}`)
  }
  return response.json()
}

async function handleEmptyResponse(response: Response, method: string, url: string): Promise<void> {
  const startTime = Date.now()
  await throwIfNotOk(response, method, url, startTime)
  logger.apiCall(method, url, response.status, Date.now() - startTime)
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
  return fetchWithSession(url, options)
}

// ============================================================================
// User API
// ============================================================================

export async function getUserProfile(): Promise<GetUserProfileResponse> {
  const url = `${API_BASE_URL}/v1/users/me`
  // This response carries Clear-Site-Data: "cache" (loaded once per sign-in or
  // account switch). Browsers apply it to a cross-origin API only when the
  // request is credentialed; the API allows credentials for its exact origins.
  const response = await fetchWithAuth(url, { credentials: 'include' })
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

/**
 * Deletes the signed-in account. Pass `token` to finish a pending deletion for
 * the specific session that reported it.
 */
export async function deleteAccount(options: { token?: string } = {}): Promise<void> {
  const url = `${API_BASE_URL}/v1/users/me`
  const response = await fetchWithSession(url, { method: 'DELETE' }, { token: options.token })
  await handleEmptyResponse(response, 'DELETE', url)
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

export async function getEntitlements(): Promise<Entitlements> {
  const url = `${API_BASE_URL}/v1/entitlements/me`
  const response = await fetchWithAuth(url)
  const data: unknown = await handleResponse(response, 'GET', url)
  const entitlements = parseEntitlements(data)
  if (!entitlements) {
    throw new Error('Unexpected entitlements response')
  }
  return entitlements
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
  const response = await fetchWithSession(
    url,
    { method: 'POST', body: formData },
    { json: false },
  )
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
  await handleEmptyResponse(response, 'DELETE', url)
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
  await handleEmptyResponse(response, 'DELETE', url)
}

// ============================================================================
// Documents API
// ============================================================================

export async function uploadDocument(pdfFile: File): Promise<UploadDocumentResponse> {
  const formData = new FormData()
  formData.append('file', pdfFile)

  const url = `${API_BASE_URL}/v1/documents`
  const response = await fetchWithSession(
    url,
    { method: 'POST', body: formData },
    { json: false },
  )
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

export async function recordDocumentOpen(documentId: number): Promise<{ status: 'saved' }> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}/open`
  const response = await fetchWithAuth(url, { method: 'POST' })
  return handleResponse(response, 'POST', url)
}

export async function deleteDocument(documentId: number): Promise<void> {
  const url = `${API_BASE_URL}/v1/documents/${documentId}`
  const response = await fetchWithAuth(url, { method: 'DELETE' })
  await handleEmptyResponse(response, 'DELETE', url)
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
  const response = await fetchWithSession(
    url,
    { method: 'GET' },
    { json: false },
  )
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
  const response = await fetchWithSession(
    url,
    { method: 'GET' },
    { json: false },
  )
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
