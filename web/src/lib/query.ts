import { QueryClient } from '@tanstack/react-query'
import { client } from '../api/client.gen'

client.setConfig({ baseUrl: '', credentials: 'include' })

export { client }

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
})
