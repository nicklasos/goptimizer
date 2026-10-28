import type { ID, PageParams } from './types'

/**
 * Query keys are hierarchical tuples, so invalidating a prefix (`qk.users.all`)
 * invalidates every list and detail under it.
 */
export const qk = {
  users: {
    all: ['users'] as const,
    list: (role: string, params: PageParams) => ['users', 'list', role, params] as const,
  },
  folders: {
    all: ['folders'] as const,
    list: () => ['folders', 'list'] as const,
    detail: (id: ID) => ['folders', 'detail', id] as const,
  },
  settings: ['settings'] as const,
  tools: ['tools'] as const,
}
