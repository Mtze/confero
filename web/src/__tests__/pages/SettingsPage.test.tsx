import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, afterEach, afterAll, describe, it, expect } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { renderWithProviders } from '../test-utils'
import { SettingsPage } from '../../pages/SettingsPage'
import type { CurrentUser, UserSettings } from '../../api'
import { client } from '../../lib/query'
import { createConfig } from '../../api/client'

const BASE_URL = 'http://localhost'

const memberUser: CurrentUser = { id: '1', email: 'test@tum.de', name: 'Test User', roles: ['member'] }

const mockSettings: UserSettings = {
  timezone: 'Europe/Berlin',
  reminder_lead_days: [7, 14],
  weekly_digest_enabled: false,
  weekly_digest_day: 1,
  weekly_digest_hour: 8,
  weekly_digest_horizon_weeks: 4,
}

const server = setupServer(
  http.get(`${BASE_URL}/api/v1/me`, () => HttpResponse.json(memberUser)),
  http.get(`${BASE_URL}/api/v1/me/settings`, () => HttpResponse.json(mockSettings)),
  http.put(`${BASE_URL}/api/v1/me/settings`, () => HttpResponse.json(mockSettings)),
)

beforeAll(() => {
  client.setConfig(createConfig({ baseUrl: BASE_URL }))
  server.listen({ onUnhandledRequest: 'error' })
})
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('SettingsPage', () => {
  it('renders form fields', async () => {
    renderWithProviders(<SettingsPage />)
    await waitFor(() => expect(screen.getByPlaceholderText('Europe/Berlin')).toBeInTheDocument())
    expect(screen.getByText('Weekly digest')).toBeInTheDocument()
  })

  it('shows current timezone value', async () => {
    renderWithProviders(<SettingsPage />)
    await waitFor(() => {
      const input = screen.getByPlaceholderText('Europe/Berlin') as HTMLInputElement
      expect(input.value).toBe('Europe/Berlin')
    })
  })

  it('submits valid form', async () => {
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />)
    await waitFor(() => expect(screen.getByPlaceholderText('Europe/Berlin')).toBeInTheDocument())
    await user.click(screen.getByText('Save settings'))
    await waitFor(() => expect(screen.getByText('Settings saved.')).toBeInTheDocument())
  })

  it('shows validation error on empty timezone', async () => {
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />)
    await waitFor(() => expect(screen.getByPlaceholderText('Europe/Berlin')).toBeInTheDocument())
    const tzInput = screen.getByPlaceholderText('Europe/Berlin')
    await user.clear(tzInput)
    await user.click(screen.getByText('Save settings'))
    await waitFor(() => expect(screen.getByText('Timezone is required')).toBeInTheDocument())
  })
})
