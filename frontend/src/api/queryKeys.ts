import type { ID, PageParams } from './types'

/**
 * Query keys are hierarchical tuples, so invalidating a prefix (`qk.storages.all`)
 * invalidates every list and detail under it.
 */
export const qk = {
  plans: {
    all: ['plans'] as const,
    dashboard: ['plans', 'dashboard'] as const,
    detail: (id: ID) => ['plans', 'detail', id] as const,
    runsOf: (id: ID) => ['plans', 'runs', id] as const,
    runs: (id: ID, params: PageParams) => ['plans', 'runs', id, params] as const,
    run: (id: ID) => ['plans', 'run', id] as const,
    archives: (id: ID, params: PageParams, storageId?: ID) => ['plans', 'archives', id, params, storageId ?? null] as const,
    restoresOf: (id: ID) => ['plans', 'restores', id] as const,
    restores: (id: ID, params: PageParams) => ['plans', 'restores', id, params] as const,
    restore: (id: ID) => ['plans', 'restore', id] as const,
    restoreTarget: (archiveId: ID) => ['plans', 'restore-target', archiveId] as const,
  },
  storages: {
    all: ['storages'] as const,
    list: (params: PageParams) => ['storages', 'list', params] as const,
    detail: (id: ID) => ['storages', 'detail', id] as const,
    objects: (id: ID, prefix: string) => ['storages', 'objects', id, prefix] as const,
  },
  users: {
    all: ['users'] as const,
    list: (role: string, params: PageParams) => ['users', 'list', role, params] as const,
  },
}
