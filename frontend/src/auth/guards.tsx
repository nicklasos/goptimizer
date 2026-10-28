import type { ReactNode } from 'react'
import { Forbidden } from '@/shared/components/Forbidden'
import { useCurrentUser } from './authStore'
import { hasAnyRole, type Role } from './roles'

export function RequireRole({ roles, children }: { roles: readonly Role[]; children: ReactNode }) {
  const user = useCurrentUser()
  if (!hasAnyRole(user, roles)) return <Forbidden />
  return <>{children}</>
}
