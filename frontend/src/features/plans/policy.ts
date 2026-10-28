import { hasAnyRole, ROLE_ADMIN } from '@/auth/roles'
import type { User } from '@/auth/types'

/*
 * Backup plans hold database passwords and decide what leaves the server, so only admins (and
 * super admins) see or change them. This mirrors RequireRole(RoleAdmin) on /api/v1/plans,
 * which is the rule that counts.
 */
export const canManagePlans = (user: User | null) => hasAnyRole(user, [ROLE_ADMIN])
