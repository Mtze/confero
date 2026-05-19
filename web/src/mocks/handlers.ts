import { http, HttpResponse } from 'msw'
import type { Conference, CurrentUser, UserSettings } from '../api'

const BASE_URL = 'http://localhost'

const mockUser: CurrentUser = {
  id: '1',
  email: 'test@tum.de',
  name: 'Test User',
  roles: ['member'],
}

const mockAdminUser: CurrentUser = {
  id: '2',
  email: 'admin@tum.de',
  name: 'Admin User',
  roles: ['member', 'admin'],
}

export { mockUser, mockAdminUser }

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

export { mockConference }

const mockSettings: UserSettings = {
  timezone: 'Europe/Berlin',
  reminder_lead_days: [7, 14],
  weekly_digest_enabled: false,
  weekly_digest_day: 1,
  weekly_digest_hour: 8,
  weekly_digest_horizon_weeks: 4,
}

export const handlers = [
  http.get(`${BASE_URL}/api/v1/me`, () => {
    return HttpResponse.json(mockUser)
  }),

  http.get(`${BASE_URL}/api/v1/conferences`, () => {
    return HttpResponse.json([mockConference])
  }),

  http.get(`${BASE_URL}/api/v1/conferences/:id`, ({ params }) => {
    return HttpResponse.json({ ...mockConference, id: params.id as string })
  }),

  http.get(`${BASE_URL}/api/v1/me/stars`, () => {
    return HttpResponse.json([mockConference])
  }),

  http.get(`${BASE_URL}/api/v1/me/settings`, () => {
    return HttpResponse.json(mockSettings)
  }),

  http.post(`${BASE_URL}/api/v1/conferences/:id/stars`, () => {
    return new HttpResponse(null, { status: 204 })
  }),

  http.delete(`${BASE_URL}/api/v1/conferences/:id/stars`, () => {
    return new HttpResponse(null, { status: 204 })
  }),
]
