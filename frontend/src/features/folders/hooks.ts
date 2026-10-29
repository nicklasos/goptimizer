import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { qk } from '@/api/queryKeys'
import type { ID } from '@/api/types'
import { foldersApi } from './api'
import { isScanActive } from './scans'
import type { FolderRequest, ScanKind } from './types'

/** Refetches every 2 s while any folder has a scan queued or running, and not at all otherwise. */
export function useFolders() {
  return useQuery({
    queryKey: qk.folders.list(),
    queryFn: foldersApi.list,
    refetchInterval: (query) => (query.state.data?.some((folder) => isScanActive(folder.last_scan)) ? 2000 : false),
  })
}

export function useFolder(id: ID | undefined) {
  return useQuery({
    queryKey: qk.folders.detail(id ?? 0),
    queryFn: () => foldersApi.get(id as ID),
    enabled: id != null,
    meta: { silent: true },
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
