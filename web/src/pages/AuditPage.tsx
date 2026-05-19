import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCurrentUser, useIsAdmin } from '../lib/auth'

export function AuditPage() {
  const navigate = useNavigate()
  const { data: user, isLoading } = useCurrentUser()
  const isAdmin = useIsAdmin()

  useEffect(() => {
    if (!isLoading) {
      if (!user) {
        navigate('/auth/login')
      } else if (!isAdmin) {
        navigate('/not-authorized')
      }
    }
  }, [user, isLoading, isAdmin, navigate])

  if (isLoading) return <div className="mx-auto max-w-5xl px-4 py-8 text-gray-500">Loading...</div>
  if (!user || !isAdmin) return null

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <h1 className="text-2xl font-bold mb-4">Audit Log</h1>
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <p className="text-gray-600">
          Audit log is coming soon. This feature will be available after the next deployment.
        </p>
      </div>
    </div>
  )
}
