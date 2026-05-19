import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'
import { addDays, subDays, formatISO } from 'date-fns'
import { DeadlineCountdown } from '../../components/ConferenceCard'

function renderCountdown(deadline: string | null | undefined) {
  return render(
    <MemoryRouter>
      <DeadlineCountdown deadline={deadline} />
    </MemoryRouter>,
  )
}

describe('DeadlineCountdown', () => {
  it('shows days remaining for far-future deadline (>30 days)', () => {
    const deadline = formatISO(addDays(new Date(), 60))
    renderCountdown(deadline)
    const el = screen.getByTestId('deadline-countdown')
    expect(el.textContent).toMatch(/\d+d left/)
    expect(el.className).toContain('text-gray-600')
  })

  it('shows warning color for near deadline (<=7 days)', () => {
    const deadline = formatISO(addDays(new Date(), 3))
    renderCountdown(deadline)
    const el = screen.getByTestId('deadline-countdown')
    expect(el.textContent).toMatch(/\d+d left/)
    expect(el.className).toContain('text-orange-600')
  })

  it('shows passed message for past deadline', () => {
    const deadline = formatISO(subDays(new Date(), 5))
    renderCountdown(deadline)
    const el = screen.getByTestId('deadline-countdown')
    expect(el.textContent).toBe('Deadline passed')
    expect(el.className).toContain('text-gray-400')
  })

  it('shows no deadline message when deadline is null', () => {
    renderCountdown(null)
    const el = screen.getByTestId('deadline-countdown')
    expect(el.textContent).toBe('No deadline')
    expect(el.className).toContain('text-gray-400')
  })

  it('shows no deadline message when deadline is undefined', () => {
    renderCountdown(undefined)
    const el = screen.getByTestId('deadline-countdown')
    expect(el.textContent).toBe('No deadline')
  })
})
