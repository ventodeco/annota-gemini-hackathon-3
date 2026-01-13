import { Link } from 'react-router-dom'

export default function NotFoundPage() {
  return (
    <div className="
      flex min-h-screen items-center justify-center bg-gray-50 p-4
    ">
      <div className="text-center">
        <h1 className="mb-4 text-4xl font-bold">404</h1>
        <p className="mb-6 text-gray-600">Page not found</p>
        <Link
          to="/"
          className="
            text-blue-600 underline
            hover:text-blue-800
          "
        >
          Go back home
        </Link>
      </div>
    </div>
  )
}
