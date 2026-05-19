import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCurrentUser } from '../lib/auth'

export function CalendarTokensPage() {
  const navigate = useNavigate()
  const { data: user, isLoading } = useCurrentUser()

  useEffect(() => {
    if (!isLoading && !user) {
      navigate('/auth/login')
    }
  }, [user, isLoading, navigate])

  if (isLoading) return <div className="mx-auto max-w-5xl px-4 py-8 text-gray-500">Loading...</div>
  if (!user) return null

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <h1 className="text-2xl font-bold mb-4">Calendar Subscription</h1>
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <p className="text-gray-600">
          Calendar subscription is coming soon. This feature will be available after the next deployment.
        </p>
        <p className="text-sm text-gray-500 mt-2">
          Once available, you will be able to subscribe to an ICS feed of your starred conferences and
          the public conference calendar.
        </p>
      </div>
    </div>
  )
}
