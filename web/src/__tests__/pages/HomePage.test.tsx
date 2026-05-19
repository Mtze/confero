import { screen, waitFor } from '@testing-library/react'
import { axe } from 'jest-axe'
import { beforeAll, afterEach, afterAll, describe, it, expect } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { renderWithProviders } from '../test-utils'
import { HomePage } from '../../pages/HomePage'
import type { Conference, CurrentUser } from '../../api'
import { client } from '../../lib/query'
import { createConfig } from '../../api/client'

const BASE_URL = 'http://localhost'

const memberUser: CurrentUser = { id: '1', email: 'test@tum.de', name: 'Test User', roles: ['member'] }

const mockConference: Conference = {
  id: '1',
  name: 'ICSE 2026',
  acronym: 'ICSE',
  year: 2026,
  location: 'Rio',
  tags: [],
  tracks: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const server = setupServer(
  http.get(`${BASE_URL}/api/v1/me`, () => HttpResponse.json(memberUser)),
  http.get(`${BASE_URL}/api/v1/conferences`, () => HttpResponse.json([mockConference])),
  http.get(`${BASE_URL}/api/v1/me/stars`, () => HttpResponse.json([])),
)

beforeAll(() => {
  client.setConfig(createConfig({ baseUrl: BASE_URL }))
  server.listen({ onUnhandledRequest: 'error' })
})
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('HomePage', () => {
  it('renders the conference list', async () => {
    renderWithProviders(<HomePage />)
    // Use the link element specifically
    await waitFor(() => expect(screen.getByRole('link', { name: 'ICSE 2026' })).toBeInTheDocument())
  })

  it('renders "New conference" button for member', async () => {
    renderWithProviders(<HomePage />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'New conference' })).toBeInTheDocument())
  })

  it('does not render "New conference" button for anonymous user', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/me`, () =>
        HttpResponse.json({ title: 'Unauthorized', status: 401 }, { status: 401 }),
      ),
    )
    renderWithProviders(<HomePage />)
    await waitFor(() => expect(screen.getByRole('link', { name: 'ICSE 2026' })).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'New conference' })).not.toBeInTheDocument()
  })

  it('shows "No conferences found" when list is empty', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/conferences`, () => HttpResponse.json([])),
    )
    renderWithProviders(<HomePage />)
    await waitFor(() => expect(screen.getByText('No conferences found.')).toBeInTheDocument())
  })

  it('has no accessibility violations', async () => {
    const { container } = renderWithProviders(<HomePage />)
    await waitFor(() => expect(screen.getByRole('link', { name: 'ICSE 2026' })).toBeInTheDocument())
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
