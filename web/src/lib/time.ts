import { differenceInDays, parseISO, isPast } from 'date-fns'

export function daysUntil(dateStr: string): number {
  return differenceInDays(parseISO(dateStr), new Date())
}

export function formatDeadline(dateStr: string | null | undefined): {
  label: string
  variant: 'far' | 'near' | 'past' | 'none'
} {
  if (!dateStr) return { label: 'No deadline', variant: 'none' }
  const date = parseISO(dateStr)
  if (isPast(date)) return { label: 'Deadline passed', variant: 'past' }
  const days = differenceInDays(date, new Date())
  if (days <= 7) return { label: `${days}d left`, variant: 'near' }
  return { label: `${days}d left`, variant: 'far' }
}
