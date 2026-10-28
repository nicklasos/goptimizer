import { isSuperAdmin, hasAnyRole, ROLE_ADMIN, ROLE_USER } from '@/auth/roles'
import type { User } from '@/auth/types'

/*
 * Who may manage whom. These mirror gogo's internal/users/policy.go and only decide what
 * the UI offers: the API enforces the same rules and is the one that counts.
 */

type Actor = User | null

/** A super admin manages everyone; an admin manages only accounts whose every role is "user". */
export function canManageUser(user: Actor, target: { roles: string[] }): boolean {
  if (isSuperAdmin(user)) return true
  if (!hasAnyRole(user, [ROLE_ADMIN]) || target.roles.length === 0) return false
  return target.roles.every((role) => role === ROLE_USER)
}

/** Nobody deletes their own account, which also keeps the last super admin in place. */
export function canDeleteUser(user: Actor, target: { id: number; roles: string[] }): boolean {
  return user != null && target.id !== user.id && canManageUser(user, target)
}
