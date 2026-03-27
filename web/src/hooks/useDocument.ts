import { useQuery } from '@tanstack/react-query'
import { getDocument, getDocumentPage } from '@/lib/api'

export function useDocument(documentId: number | undefined) {
  return useQuery({
    queryKey: ['document', documentId],
    queryFn: () => getDocument(documentId!),
    enabled: !!documentId,
  })
}

export function useDocumentPage(documentId: number | undefined, pageNumber: number) {
  return useQuery({
    queryKey: ['documentPage', documentId, pageNumber],
    queryFn: () => getDocumentPage(documentId!, pageNumber),
    enabled: !!documentId && pageNumber >= 1,
    staleTime: 5 * 60 * 1000,
  })
}
