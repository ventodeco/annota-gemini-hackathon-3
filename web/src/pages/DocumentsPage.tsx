import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { BookOpen, PackageOpen, Trash2 } from 'lucide-react'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { useDeleteDocument, useDocuments } from '@/hooks/useDocument'
import { formatDate } from '@/lib/api'
import type { Document } from '@/lib/types'
import { toast } from 'sonner'

export default function DocumentsPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [documentToDelete, setDocumentToDelete] = useState<Document | null>(null)
  const { data, isLoading, error } = useDocuments(page, 20)
  const deleteDocument = useDeleteDocument()

  const handleOpen = (doc: Document) => {
    navigate(`/documents/${doc.id}?page=${doc.lastPageNumber || 1}`)
  }

  const handleDelete = async () => {
    if (!documentToDelete) return
    try {
      await deleteDocument.mutateAsync(documentToDelete.id)
      setDocumentToDelete(null)
      toast.success('Document removed')
    } catch {
      toast.error('Failed to remove document')
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="Documents" />
      <main className="pt-4 px-4">
        {isLoading && (
          <div className="flex justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
          </div>
        )}

        {error && (
          <div className="text-center py-8 text-red-600">
            Failed to load documents. Please try again.
          </div>
        )}

        {!isLoading && !error && data?.data.length === 0 && (
          <div className="flex min-h-[60vh] flex-col items-center justify-center text-center text-gray-500">
            <PackageOpen className="mb-5 h-16 w-16 text-slate-400" aria-label="empty documents" />
            <p className="mb-4 max-w-[260px] text-base leading-relaxed text-slate-500">
              No saved documents yet.
            </p>
            <Link
              to="/welcome"
              className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white"
            >
              Upload PDF
            </Link>
          </div>
        )}

        {!isLoading && !error && data?.data.length !== undefined && data.data.length > 0 && (
          <div className="space-y-4">
            {data.data.map((doc) => (
              <div key={doc.id} className="flex items-center gap-3 rounded-lg bg-white p-4 shadow-sm">
                <button
                  onClick={() => handleOpen(doc)}
                  className="flex min-w-0 flex-1 items-start gap-3 text-left"
                  aria-label={`Open ${doc.filename}`}
                >
                  <BookOpen className="mt-1 h-5 w-5 shrink-0 text-slate-500" />
                  <span className="min-w-0">
                    <span className="block truncate font-medium text-gray-900">{doc.filename}</span>
                    <span className="mt-1 block text-sm text-gray-500">
                      {doc.pageCount} pages, page {doc.lastPageNumber || 1}
                    </span>
                    <span className="mt-2 block text-xs text-gray-400">
                      {doc.lastOpenedAt ? `Last opened ${formatDate(doc.lastOpenedAt)}` : `Added ${formatDate(doc.createdAt)}`}
                    </span>
                  </span>
                </button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => setDocumentToDelete(doc)}
                  aria-label="Delete document"
                  className="shrink-0 text-gray-500 hover:text-red-600"
                >
                  <Trash2 className="h-5 w-5" />
                </Button>
              </div>
            ))}
          </div>
        )}

        {data?.meta && (
          <div className="mt-6 flex justify-center gap-4">
            {data.meta.previousPage && (
              <button
                onClick={() => setPage(data.meta.previousPage!)}
                className="rounded-lg bg-white px-4 py-2 text-sm font-medium shadow-sm"
              >
                Previous
              </button>
            )}
            <span className="px-4 py-2 text-sm text-gray-500">
              Page {data.meta.currentPage}
            </span>
            {data.meta.nextPage && (
              <button
                onClick={() => setPage(data.meta.nextPage!)}
                className="rounded-lg bg-white px-4 py-2 text-sm font-medium shadow-sm"
              >
                Next
              </button>
            )}
          </div>
        )}
      </main>
      <BottomNavigation />

      <AlertDialog open={!!documentToDelete} onOpenChange={(open) => !open && setDocumentToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove document?</AlertDialogTitle>
            <AlertDialogDescription>
              Remove this PDF and its reader entries? Saved annotations stay in History.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={deleteDocument.isPending}
              className="bg-red-600 hover:bg-red-700"
            >
              {deleteDocument.isPending ? 'Removing...' : 'Remove'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
