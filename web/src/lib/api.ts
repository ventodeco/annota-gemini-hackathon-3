import type {
  CreateScanResponse,
  GetScanResponse,
  AnnotateRequest,
  AnnotateResponse,
} from './types'

export type { AnnotateRequest }

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'

export async function createScan(imageFile: File): Promise<CreateScanResponse> {
  const formData = new FormData()
  formData.append('image', imageFile)

  const response = await fetch(`${API_BASE_URL}/scans`, {
    method: 'POST',
    body: formData,
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(error || 'Failed to create scan')
  }

  return response.json()
}

export async function getScan(scanID: string): Promise<GetScanResponse> {
  const response = await fetch(`${API_BASE_URL}/scans/${scanID}`)

  if (!response.ok) {
    if (response.status === 404) {
      throw new Error('Scan not found')
    }
    const error = await response.text()
    throw new Error(error || 'Failed to get scan')
  }

  return response.json()
}

export async function annotate(
  scanID: string,
  request: AnnotateRequest,
): Promise<AnnotateResponse> {
  const response = await fetch(`${API_BASE_URL}/scans/${scanID}/annotate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(error || 'Failed to create annotation')
  }

  return response.json()
}

export function getScanImageURL(scanID: string): string {
  return `${API_BASE_URL}/scans/${scanID}/image`
}
