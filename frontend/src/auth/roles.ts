export const ROLE_SUPER_ADMIN = 'super-admin'
export const ROLE_ADMIN = 'admin'
export const ROLE_USER = 'user'

export const ROLES = [ROLE_SUPER_ADMIN, ROLE_ADMIN, ROLE_USER] as const

export type Role = (typeof ROLES)[number]

export interface RoleHolder {
  roles?: string[] | null
}

export function isSuperAdmin(user: RoleHolder | null | undefined): boolean {
  return Array.isArray(user?.roles) && user.roles.includes(ROLE_SUPER_ADMIN)
}

/** A super admin satisfies every role requirement, the same as on the backend. */
export function hasAnyRole(user: RoleHolder | null | undefined, roles: readonly Role[]): boolean {
  if (!user || !Array.isArray(user.roles)) return false
  if (isSuperAdmin(user)) return true
  return user.roles.some((role) => (roles as readonly string[]).includes(role))
}
