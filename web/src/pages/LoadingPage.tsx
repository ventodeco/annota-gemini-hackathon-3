import { useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

export default function LoadingPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  useEffect(() => {
    if (id) {
      navigate(`/scans/${id}`, { replace: true })
    } else {
      navigate('/welcome', { replace: true })
    }
  }, [id, navigate])

  return (
    <div className="min-h-screen bg-white flex flex-col pb-20">
      <div className="flex-1 flex flex-col items-center justify-center p-6">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900" />
        <p className="mt-4 text-center text-gray-600 text-sm">Loading...</p>
      </div>
    </div>
  )
}
