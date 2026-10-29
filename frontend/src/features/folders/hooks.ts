import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { qk } from '@/api/queryKeys'
import type { ID } from '@/api/types'
import { foldersApi } from './api'
import { pollInterval } from './overview'
import type { FolderRequest, ScanKind } from './types'

/** Refetches on its own as `pollInterval` says. */
export function useFolders() {
  return useQuery({
    queryKey: qk.folders.list(),
    queryFn: foldersApi.list,
    refetchInterval: (query) => pollInterval(query.state.data ?? []),
  })
}

/** `live` refetches as `pollInterval` says; the editor leaves it off, so a refetch never resets the form. */
export function useFolder(id: ID | undefined, { live = false } = {}) {
  return useQuery({
    queryKey: qk.folders.detail(id ?? 0),
    queryFn: () => foldersApi.get(id as ID),
    enabled: id != null,
    refetchInterval: (query) => live && pollInterval(query.state.data ? [query.state.data] : []),
    meta: { silent: true },
  })
}

/**
 * Bytes saved on each of the last 30 UTC days, by one folder or by all of them. Refetched at the
 * rate of the folder data around it, since it grows as the same images are optimized.
 */
export function useDailySavings(folderId: ID | undefined, refetchInterval: number | false) {
  return useQuery({
    queryKey: qk.dailySavings(folderId),
    queryFn: () => foldersApi.dailySavings(folderId),
    refetchInterval,
  })
}

/** Field errors are shown in the form, so the global error toast is silenced. */
export function useSaveFolder() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id?: ID; body: FolderRequest }) => (id ? foldersApi.update(id, body) : foldersApi.create(body)),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.folders.all }),
    meta: { silent: true },
  })
}

export function useSetFolderEnabled() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, enabled }: { id: ID; enabled: boolean }) => foldersApi.setEnabled(id, enabled),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.folders.all }),
  })
}

export function useDeleteFolder() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (id: ID) => foldersApi.remove(id),
    onSuccess: () => client.invalidateQueries({ queryKey: qk.folders.all }),
  })
}

export function useStartScan() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, kind }: { id: ID; kind: ScanKind }) => foldersApi.startScan(id, kind),
    onSettled: () => client.invalidateQueries({ queryKey: qk.folders.all }),
  })
}
