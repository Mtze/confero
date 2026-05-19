import { useQuery } from '@tanstack/react-query'
import { getMe } from '../api'
import { client } from './query'

export function useCurrentUser() {
  return useQuery({
    queryKey: ['me'],
    queryFn: () => getMe({ client }).then(r => r.data),
    retry: false,
  })
}

export function useIsAdmin() {
  const { data } = useCurrentUser()
  return data?.roles?.includes('admin') ?? false
}

export function useIsMember() {
  const { data } = useCurrentUser()
  return (data?.roles?.includes('member') || data?.roles?.includes('admin')) ?? false
}
