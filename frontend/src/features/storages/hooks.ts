import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { qk } from '@/api/queryKeys'
import type { ID, PageParams } from '@/api/types'
import { storagesApi } from './api'
import type { StorageRequest } from './types'

export function useStorages(params: PageParams) {
  return useQuery({
    queryKey: qk.storages.list(params),
    queryFn: () => storagesApi.list(params),
    placeholderData: keepPreviousData,
  })
}

export function useStorage(id: ID | undefined) {
  return useQuery({
    queryKey: qk.storages.detail(id ?? 0),
    queryFn: () => storagesApi.get(id as ID),
    enabled: id != null,
    meta: { silent: true },
  })
}

/** The browser shows a failed listing itself, so the global toast is silenced. */
export function useStorageObjects(id: ID, prefix: string) {
  return useQuery({
    queryKey: qk.storages.objects(id, prefix),
    queryFn: () => storagesApi.objects(id, prefix),
    retry: false,
    meta: { silent: true },
  })
}

/** Field errors are shown in the form, so the global error toast is silenced. */
export function useSaveStorage() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id?: ID; body: StorageRequest }) => (id ? storagesApi.update(id, body) : storagesApi.create(body)),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.storages.all }),
    meta: { silent: true },
  })
}

/** A failed connection is a result shown next to the button; only a rejected form is an error. */
export function useTestStorage() {
  return useMutation({
    mutationFn: ({ storageId, body }: { storageId?: ID; body: StorageRequest }) => storagesApi.test(body, storageId),
    meta: { silent: true },
  })
}

/** A storage that plans use cannot be deleted; the page says which plans, so the toast is silenced. */
export function useDeleteStorage() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (id: ID) => storagesApi.remove(id),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.storages.all }),
    meta: { silent: true },
  })
}
