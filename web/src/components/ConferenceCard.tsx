import { Link } from 'react-router-dom'
import { clsx } from 'clsx'
import type { Conference } from '../api'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { formatDeadline } from '../lib/time'

interface ConferenceCardProps {
  conference: Conference
  starred?: boolean
  isMember?: boolean
  onStar?: (id: string) => void
  onUnstar?: (id: string) => void
}

const coreRankColors: Record<string, string> = {
  'A*': 'blue',
  A: 'green',
  B: 'yellow',
  C: 'default',
  Unranked: 'default',
}

export function DeadlineCountdown({ deadline }: { deadline: string | null | undefined }) {
  const { label, variant } = formatDeadline(deadline)
  return (
    <span
      data-testid="deadline-countdown"
      className={clsx(
        'text-sm font-medium',
        variant === 'past' && 'text-gray-400',
        variant === 'near' && 'text-orange-600',
        variant === 'far' && 'text-gray-600',
        variant === 'none' && 'text-gray-400',
      )}
    >
      {label}
    </span>
  )
}

export function ConferenceCard({ conference, starred, isMember, onStar, onUnstar }: ConferenceCardProps) {
  const rank = conference.core_rank
  const rankColor = (rank && coreRankColors[rank]) || 'default'

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm flex flex-col gap-2">
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-col gap-1">
          <Link
            to={`/conferences/${conference.id}`}
            className="text-base font-semibold text-blue-700 hover:underline"
          >
            {conference.name}
          </Link>
          <div className="flex items-center gap-2 text-sm text-gray-500">
            <span className="font-mono font-bold">{conference.acronym} {conference.year}</span>
            <span>-</span>
            <span>{conference.location}</span>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {rank && (
            <Badge variant={rankColor as 'default' | 'blue' | 'green' | 'yellow' | 'red'}>
              {rank}
            </Badge>
          )}
          {isMember && (
            <Button
              size="sm"
              variant={starred ? 'secondary' : 'ghost'}
              onClick={() => starred ? onUnstar?.(conference.id) : onStar?.(conference.id)}
              aria-label={starred ? 'Unstar conference' : 'Star conference'}
              title={starred ? 'Unstar' : 'Star'}
            >
              {starred ? '★' : '☆'}
            </Button>
          )}
        </div>
      </div>
      <div className="flex items-center gap-4 text-xs text-gray-500">
        <DeadlineCountdown deadline={conference.primary_deadline} />
        {conference.tags.length > 0 && (
          <div className="flex gap-1">
            {conference.tags.map(t => (
              <Badge key={t.id}>{t.name}</Badge>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
