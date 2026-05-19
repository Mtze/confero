import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { createClient, createConfig } from '../../api/client'
import { getHealth } from '../../api/index'
import type { HealthStatus } from '../../api/index'

const BASE_URL = 'http://localhost'

const server = setupServer(
  http.get(`${BASE_URL}/healthz`, () => {
    const body: HealthStatus = { status: 'ok' }
    return HttpResponse.json(body)
  }),
)

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

const testClient = createClient(createConfig({ baseUrl: BASE_URL }))

describe('getHealth', () => {
  it('returns status ok on 200', async () => {
    const result = await getHealth({ client: testClient })

    expect(result.error).toBeUndefined()
    expect(result.data).toBeDefined()
    expect(result.data?.status).toBe('ok')
  })

  it('returns typed HealthStatus', async () => {
    const result = await getHealth({ client: testClient })

    // type check: status must be the literal 'ok'
    const status: 'ok' = result.data!.status
    expect(status).toBe('ok')
  })
})
