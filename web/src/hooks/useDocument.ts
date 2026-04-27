import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteDocument, getDocument, getDocumentPage, getDocuments, updateDocumentProgress } from '@/lib/api'
import type { Document, GetDocumentsResponse } from '@/lib/types'

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

export function useDocuments(page = 1, size = 20) {
  return useQuery({
    queryKey: ['documents', page, size],
    queryFn: () => getDocuments(page, size),
  })
}

export function useUpdateDocumentProgress(documentId: number | undefined) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (lastPageNumber: number) => {
      if (!documentId) throw new Error('Document ID is required')
      return updateDocumentProgress(documentId, lastPageNumber)
    },
    onSuccess: (_data, lastPageNumber) => {
      queryClient.setQueryData<Document>(['document', documentId], (document) => {
        return document ? { ...document, lastPageNumber } : document
      })
      queryClient.setQueriesData<GetDocumentsResponse>({ queryKey: ['documents'] }, (documents) => {
        if (!documentId || !documents) {
          return documents
        }
        return {
          ...documents,
          data: documents.data.map((doc) =>
            doc.id === documentId ? { ...doc, lastPageNumber } : doc
          ),
        }
      })
    },
  })
}

export function useDeleteDocument() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (documentId: number) => deleteDocument(documentId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['documents'] })
    },
  })
}
