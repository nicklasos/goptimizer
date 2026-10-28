import { hasAnyRole, ROLE_ADMIN } from '@/auth/roles'
import type { User } from '@/auth/types'

/*
 * Storages are shared by the whole installation and hold credentials, so only admins (and
 * super admins) see or change them. This mirrors RequireRole(RoleAdmin) on /api/v1/storages,
 * which is the rule that counts.
 */
export const canManageStorages = (user: User | null) => hasAnyRole(user, [ROLE_ADMIN])
