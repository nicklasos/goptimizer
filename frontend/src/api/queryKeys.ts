import type { PageParams } from './types'

/**
 * Query keys are hierarchical tuples, so invalidating a prefix (`qk.users.all`)
 * invalidates every list and detail under it.
 */
export const qk = {
  users: {
    all: ['users'] as const,
    list: (role: string, params: PageParams) => ['users', 'list', role, params] as const,
  },
}
