import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { qk } from '@/api/queryKeys'
import type { ID, PageParams } from '@/api/types'
import { usersApi } from './api'
import type { UserFormValues } from './types'

export function useUsers(params: PageParams) {
  return useQuery({
    queryKey: qk.users.list(params),
    queryFn: () => usersApi.list(params),
    placeholderData: keepPreviousData,
  })
}

/** Creates a user, or updates name, email and the admin flag when `id` is set. */
export function useSaveUser() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, values }: { id?: ID; values: UserFormValues }) =>
      id
        ? usersApi.update(id, { email: values.email, name: values.name, is_admin: values.is_admin })
        : usersApi.create({ email: values.email, name: values.name, password: values.password ?? '', is_admin: values.is_admin }),
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
