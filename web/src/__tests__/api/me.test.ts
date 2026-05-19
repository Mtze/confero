import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { createClient, createConfig } from '../../api/client'
import { getMe } from '../../api/index'
import type { CurrentUser } from '../../api/index'

const BASE_URL = 'http://localhost'

const mockUser: CurrentUser = {
  id: '550e8400-e29b-41d4-a716-446655440000',
  email: 'member@example.org',
  name: 'Test Member',
  roles: ['member'],
}

const server = setupServer()
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

const testClient = createClient(createConfig({ baseUrl: BASE_URL }))

describe('getMe', () => {
  it('returns the current user on 200', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/me`, () => HttpResponse.json(mockUser)),
    )

    const result = await getMe({ client: testClient })

    expect(result.error).toBeUndefined()
    expect(result.data?.email).toBe('member@example.org')
    expect(result.data?.roles).toContain('member')
  })

  it('returns error data on 401', async () => {
    server.use(
      http.get(`${BASE_URL}/api/v1/me`, () =>
        HttpResponse.json(
          { title: 'Unauthorized', status: 401, detail: 'authentication required' },
          { status: 401 },
        ),
      ),
    )

    const result = await getMe({ client: testClient })

    expect(result.data).toBeUndefined()
    expect(result.response.status).toBe(401)
  })
})
