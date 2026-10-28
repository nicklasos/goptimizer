import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { qk } from '@/api/queryKeys'
import type { ID, PageParams } from '@/api/types'
import type { Role } from '@/auth/roles'
import { usersApi } from './api'
import type { UserFormValues } from './types'

export function useUsers(role: Role, params: PageParams) {
  return useQuery({
    queryKey: qk.users.list(role, params),
    queryFn: () => usersApi.list(role, params),
    placeholderData: keepPreviousData,
  })
}

/** Creates a user with the given role, or updates name and email when `id` is set. */
export function useSaveUser(role: Role) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, values }: { id?: ID; values: UserFormValues }) =>
      id
        ? usersApi.update(id, { email: values.email, name: values.name })
        : usersApi.create({ email: values.email, name: values.name, password: values.password ?? '', role }),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.users.all }),
    meta: { silent: true },
  })
}

export function useSetUserPassword() {
  return useMutation({
    mutationFn: ({ id, password }: { id: ID; password: string }) => usersApi.setPassword(id, password),
  })
}

export function useDeleteUser() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (id: ID) => usersApi.remove(id),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.users.all }),
  })
}
