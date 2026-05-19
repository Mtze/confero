import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listConferences, starConference, unstarConference, listMyStars } from '../api'
import { client } from '../lib/query'
import { useCurrentUser, useIsMember } from '../lib/auth'
import { ConferenceCard } from '../components/ConferenceCard'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { ConferenceDialog } from '../features/conferences/ConferenceDialog'

export function HomePage() {
  const [q, setQ] = useState('')
  const [showArchived, setShowArchived] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)

  const { data: user } = useCurrentUser()
  const isMember = useIsMember()
  const qc = useQueryClient()

  const { data: conferences = [], isLoading } = useQuery({
    queryKey: ['conferences', { q, showArchived }],
    queryFn: () =>
      listConferences({
        client,
        query: { q: q || undefined, archived: showArchived || undefined },
      }).then(r => r.data ?? []),
  })

  const { data: stars = [] } = useQuery({
    queryKey: ['stars'],
    queryFn: () => user ? listMyStars({ client }).then(r => r.data ?? []) : [],
    enabled: !!user,
  })

  const starredIds = new Set(stars.map(s => s.id))

  const starMutation = useMutation({
    mutationFn: (id: string) => starConference({ client, path: { id } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['stars'] }),
  })

  const unstarMutation = useMutation({
    mutationFn: (id: string) => unstarConference({ client, path: { id } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['stars'] }),
  })

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Conferences</h1>
        {isMember && (
          <Button onClick={() => setDialogOpen(true)}>New conference</Button>
        )}
      </div>

      <div className="flex items-center gap-4 mb-6">
        <Input
          placeholder="Search by name or acronym..."
          value={q}
          onChange={e => setQ(e.target.value)}
          className="w-72"
        />
        <label className="flex items-center gap-2 text-sm text-gray-600">
          <input
            type="checkbox"
            checked={showArchived}
            onChange={e => setShowArchived(e.target.checked)}
          />
          Show archived
        </label>
      </div>

      {isLoading && <p className="text-gray-500">Loading...</p>}

      <div className="flex flex-col gap-3">
        {conferences.map(conf => (
          <ConferenceCard
            key={conf.id}
            conference={conf}
            starred={starredIds.has(conf.id)}
            isMember={isMember}
            onStar={id => starMutation.mutate(id)}
            onUnstar={id => unstarMutation.mutate(id)}
          />
        ))}
        {!isLoading && conferences.length === 0 && (
          <p className="text-gray-500">No conferences found.</p>
        )}
      </div>

      <ConferenceDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSuccess={() => qc.invalidateQueries({ queryKey: ['conferences'] })}
      />
    </div>
  )
}
