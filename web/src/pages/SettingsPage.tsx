import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { getMySettings } from '../api'
import { client } from '../lib/query'
import { useCurrentUser } from '../lib/auth'
import { SettingsForm } from '../features/settings/SettingsForm'

export function SettingsPage() {
  const navigate = useNavigate()
  const { data: user, isLoading: userLoading } = useCurrentUser()

  useEffect(() => {
    if (!userLoading && !user) {
      navigate('/auth/login')
    }
  }, [user, userLoading, navigate])

  const { data: settings, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: () => getMySettings({ client }).then(r => r.data!),
    enabled: !!user,
  })

  if (userLoading || isLoading) return <div className="mx-auto max-w-5xl px-4 py-8 text-gray-500">Loading...</div>
  if (!user) return null

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>
      {settings && <SettingsForm settings={settings} />}
    </div>
  )
}
