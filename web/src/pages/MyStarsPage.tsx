import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listMyStars, starConference, unstarConference } from '../api'
import { client } from '../lib/query'
import { useCurrentUser } from '../lib/auth'
import { ConferenceCard } from '../components/ConferenceCard'

export function MyStarsPage() {
  const navigate = useNavigate()
  const { data: user, isLoading: userLoading, error: userError } = useCurrentUser()
  const qc = useQueryClient()

  useEffect(() => {
    if (!userLoading && !user) {
      navigate('/auth/login')
    }
  }, [user, userLoading, navigate])

  const { data: stars = [], isLoading } = useQuery({
    queryKey: ['stars'],
    queryFn: () => listMyStars({ client }).then(r => r.data ?? []),
    enabled: !!user,
  })

  const starMutation = useMutation({
    mutationFn: (id: string) => starConference({ client, path: { id } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['stars'] }),
  })

  const unstarMutation = useMutation({
    mutationFn: (id: string) => unstarConference({ client, path: { id } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['stars'] }),
  })

  if (userLoading || isLoading) return <div className="mx-auto max-w-5xl px-4 py-8 text-gray-500">Loading...</div>
  if (!user) return null

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <h1 className="text-2xl font-bold mb-6">My Starred Conferences</h1>
      <div className="flex flex-col gap-3">
        {stars.map(conf => (
          <ConferenceCard
            key={conf.id}
            conference={conf}
            starred={true}
            isMember={true}
            onStar={id => starMutation.mutate(id)}
            onUnstar={id => unstarMutation.mutate(id)}
          />
        ))}
        {stars.length === 0 && (
          <p className="text-gray-500">You have not starred any conferences yet.</p>
        )}
      </div>
    </div>
  )
}
