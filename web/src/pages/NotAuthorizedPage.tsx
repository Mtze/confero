import { Link } from 'react-router-dom'

export function NotAuthorizedPage() {
  return (
    <div className="mx-auto max-w-5xl px-4 py-16 text-center">
      <h1 className="text-4xl font-bold text-gray-900 mb-4">403</h1>
      <p className="text-lg text-gray-600 mb-6">You do not have permission to view this page.</p>
      <Link to="/" className="text-blue-600 hover:underline">Back to home</Link>
    </div>
  )
}
