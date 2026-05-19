import { screen, waitFor } from '@testing-library/react'
import { axe } from 'jest-axe'
import { beforeAll, afterEach, afterAll, describe, it, expect } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { renderWithProviders } from '../test-utils'
import { ConferenceDetailPage } from '../../pages/ConferenceDetailPage'
import type { Conference, CurrentUser } from '../../api'
import { client } from '../../lib/query'
import { createConfig } from '../../api/client'

const BASE_URL = 'http://localhost'

const memberUser: CurrentUser = { id: '1', email: 'test@tum.de', name: 'Test User', roles: ['member'] }
const adminUser: CurrentUser = { id: '2', email: 'admin@tum.de', name: 'Admin User', roles: ['member', 'admin'] }

const mockConference: Conference = {
  id: '1',
  name: 'ICSE 2026',
  acronym: 'ICSE',
  year: 2026,
  location: 'Rio de Janeiro',
  core_rank: 'A*',
  notes: 'Important venue',
  tags: [],
  tracks: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const server = setupServer(
  http.get(`${BASE_URL}/api/v1/me`, () => HttpResponse.json(memberUser)),
  http.get(`${BASE_URL}/api/v1/conferences/:id`, () => HttpResponse.json(mockConference)),
  http.get(`${BASE_URL}/api/v1/me/stars`, () => HttpResponse.json([])),
)

beforeAll(() => {
  client.setConfig(createConfig({ baseUrl: BASE_URL }))
  server.listen({ onUnhandledRequest: 'error' })
})
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('ConferenceDetailPage', () => {
  it('renders conference fields', async () => {
    renderWithProviders(<ConferenceDetailPage />, {
      initialEntries: ['/conferences/1'],
      routePath: '/conferences/:id',
    })
    await waitFor(() => expect(screen.getByRole('heading', { name: 'ICSE 2026' })).toBeInTheDocument())
    expect(screen.getByText(/Rio de Janeiro/)).toBeInTheDocument()
    expect(screen.getByText('A*')).toBeInTheDocument()
    expect(screen.getByText('Important venue')).toBeInTheDocument()
  })

  it('shows edit button for member', async () => {
    renderWithProviders(<ConferenceDetailPage />, {
      initialEntries: ['/conferences/1'],
      routePath: '/conferences/:id',
    })
    await waitFor(() => expect(screen.getByRole('heading', { name: 'ICSE 2026' })).toBeInTheDocument())
    expect(screen.getByText('Edit')).toBeInTheDocument()
  })

  it('shows delete button for admin', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/me`, () => HttpResponse.json(adminUser)),
    )
    renderWithProviders(<ConferenceDetailPage />, {
      initialEntries: ['/conferences/1'],
      routePath: '/conferences/:id',
    })
    await waitFor(() => expect(screen.getByText('Delete')).toBeInTheDocument())
  })

  it('does not show delete button for non-admin', async () => {
    renderWithProviders(<ConferenceDetailPage />, {
      initialEntries: ['/conferences/1'],
      routePath: '/conferences/:id',
    })
    await waitFor(() => expect(screen.getByRole('heading', { name: 'ICSE 2026' })).toBeInTheDocument())
    expect(screen.queryByText('Delete')).not.toBeInTheDocument()
  })

  it('has no accessibility violations', async () => {
    const { container } = renderWithProviders(<ConferenceDetailPage />, {
      initialEntries: ['/conferences/1'],
      routePath: '/conferences/:id',
    })
    await waitFor(() => expect(screen.getByRole('heading', { name: 'ICSE 2026' })).toBeInTheDocument())
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
