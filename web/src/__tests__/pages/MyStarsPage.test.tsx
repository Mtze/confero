import { screen, waitFor } from '@testing-library/react'
import { beforeAll, afterEach, afterAll, describe, it, expect } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { renderWithProviders } from '../test-utils'
import { MyStarsPage } from '../../pages/MyStarsPage'
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
  http.get(`${BASE_URL}/api/v1/me/stars`, () => HttpResponse.json([mockConference])),
)

beforeAll(() => {
  client.setConfig(createConfig({ baseUrl: BASE_URL }))
  server.listen({ onUnhandledRequest: 'error' })
})
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('MyStarsPage', () => {
  it('renders starred conferences', async () => {
    renderWithProviders(<MyStarsPage />)
    await waitFor(() => expect(screen.getAllByText('ICSE 2026').length).toBeGreaterThan(0))
    // The conference link should be present
    expect(screen.getByRole('link', { name: 'ICSE 2026' })).toBeInTheDocument()
  })

  it('shows page heading', async () => {
    renderWithProviders(<MyStarsPage />)
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'My Starred Conferences' })).toBeInTheDocument(),
    )
  })

  it('shows empty message when no stars', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/me/stars`, () => HttpResponse.json([])),
    )
    renderWithProviders(<MyStarsPage />)
    await waitFor(() =>
      expect(screen.getByText('You have not starred any conferences yet.')).toBeInTheDocument(),
    )
  })
})
