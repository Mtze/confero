import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getConference, starConference, unstarConference, archiveConference, unarchiveConference, deleteConference, listMyStars } from '../api'
import { client } from '../lib/query'
import { useCurrentUser, useIsAdmin, useIsMember } from '../lib/auth'
import { Button } from '../components/ui/Button'
import { Badge } from '../components/ui/Badge'
import { DeadlineCountdown } from '../components/ConferenceCard'
import { ConferenceDialog } from '../features/conferences/ConferenceDialog'
import { parseISO, format } from 'date-fns'

function DateField({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-gray-500 uppercase tracking-wide">{label}</span>
      <span className="text-sm">{format(parseISO(value), 'PPP p')}</span>
    </div>
  )
}

export function ConferenceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: user } = useCurrentUser()
  const isAdmin = useIsAdmin()
  const isMember = useIsMember()
  const [editOpen, setEditOpen] = useState(false)

  const { data: conference, isLoading } = useQuery({
    queryKey: ['conference', id],
    queryFn: () => getConference({ client, path: { id: id! } }).then(r => r.data!),
    enabled: !!id,
  })

  const { data: stars = [] } = useQuery({
    queryKey: ['stars'],
    queryFn: () => user ? listMyStars({ client }).then(r => r.data ?? []) : [],
    enabled: !!user,
  })

  const starred = stars.some(s => s.id === id)

  const starMutation = useMutation({
    mutationFn: () => starConference({ client, path: { id: id! } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['stars'] }),
  })

  const unstarMutation = useMutation({
    mutationFn: () => unstarConference({ client, path: { id: id! } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['stars'] }),
  })

  const archiveMutation = useMutation({
    mutationFn: () => archiveConference({ client, path: { id: id! } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['conference', id] }),
  })

  const unarchiveMutation = useMutation({
    mutationFn: () => unarchiveConference({ client, path: { id: id! } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['conference', id] }),
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteConference({ client, path: { id: id! } }),
    onSuccess: () => navigate('/'),
  })

  if (isLoading) return <div className="mx-auto max-w-5xl px-4 py-8 text-gray-500">Loading...</div>
  if (!conference) return <div className="mx-auto max-w-5xl px-4 py-8 text-gray-500">Conference not found.</div>

  const isArchived = !!conference.archived_at

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <Link to="/" className="text-sm text-blue-600 hover:underline mb-4 inline-block">
        Back to list
      </Link>

      <div className="bg-white rounded-lg border border-gray-200 shadow-sm p-6">
        <div className="flex items-start justify-between gap-4 mb-4">
          <div>
            <h1 className="text-2xl font-bold">{conference.name}</h1>
            <p className="text-gray-500 mt-1">
              {conference.acronym} {conference.year} - {conference.location}
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {conference.core_rank && (
              <Badge variant="blue">{conference.core_rank}</Badge>
            )}
            {isArchived && <Badge variant="default">Archived</Badge>}
          </div>
        </div>

        <div className="flex flex-wrap gap-2 mb-6">
          {isMember && (
            <>
              <Button
                variant={starred ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => starred ? unstarMutation.mutate() : starMutation.mutate()}
              >
                {starred ? '★ Unstar' : '☆ Star'}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setEditOpen(true)}>
                Edit
              </Button>
              {isArchived ? (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => unarchiveMutation.mutate()}
                  disabled={unarchiveMutation.isPending}
                >
                  Unarchive
                </Button>
              ) : (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => archiveMutation.mutate()}
                  disabled={archiveMutation.isPending}
                >
                  Archive
                </Button>
              )}
            </>
          )}
          {isAdmin && (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => {
                if (confirm('Delete this conference permanently?')) {
                  deleteMutation.mutate()
                }
              }}
              disabled={deleteMutation.isPending}
            >
              Delete
            </Button>
          )}
        </div>

        <div className="grid grid-cols-2 gap-4 mb-6">
          <div className="flex flex-col gap-0.5">
            <span className="text-xs text-gray-500 uppercase tracking-wide">Primary Deadline</span>
            <DeadlineCountdown deadline={conference.primary_deadline} />
            {conference.primary_deadline && (
              <span className="text-xs text-gray-500">
                {format(parseISO(conference.primary_deadline), 'PPP p')}
              </span>
            )}
          </div>
          <DateField label="Abstract Deadline" value={conference.abstract_deadline} />
          <DateField label="Notification" value={conference.notification_date} />
          <DateField label="Camera Ready" value={conference.camera_ready_date} />
          <DateField label="Event Start" value={conference.event_start_date} />
          <DateField label="Event End" value={conference.event_end_date} />
        </div>

        {conference.website_url && (
          <div className="mb-3">
            <a
              href={conference.website_url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-blue-600 hover:underline"
            >
              Conference website
            </a>
          </div>
        )}

        {conference.cfp_url && (
          <div className="mb-3">
            <a
              href={conference.cfp_url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-blue-600 hover:underline"
            >
              Call for Papers
            </a>
          </div>
        )}

        {conference.h5_index && (
          <p className="text-sm text-gray-600 mb-2">H5 index: {conference.h5_index}</p>
        )}

        {conference.acceptance_rate_pct && (
          <p className="text-sm text-gray-600 mb-2">Acceptance rate: {conference.acceptance_rate_pct}%</p>
        )}

        {conference.tags.length > 0 && (
          <div className="flex gap-2 flex-wrap mb-4">
            {conference.tags.map(t => <Badge key={t.id}>{t.name}</Badge>)}
          </div>
        )}

        {conference.tracks.length > 0 && (
          <div className="flex gap-2 flex-wrap mb-4">
            {conference.tracks.map(t => <Badge key={t.code} variant="blue">{t.display_name}</Badge>)}
          </div>
        )}

        {conference.notes && (
          <div className="mt-4 p-3 bg-gray-50 rounded text-sm text-gray-700 whitespace-pre-wrap">
            {conference.notes}
          </div>
        )}
      </div>

      <ConferenceDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        conference={conference}
        onSuccess={() => qc.invalidateQueries({ queryKey: ['conference', id] })}
      />
    </div>
  )
}
