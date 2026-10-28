import type { ReactNode } from 'react'
import { Forbidden } from '@/shared/components/Forbidden'
import { useCurrentUser } from './authStore'

export function RequireAdmin({ children }: { children: ReactNode }) {
  const user = useCurrentUser()
  if (!user?.is_admin) return <Forbidden />
  return <>{children}</>
}
