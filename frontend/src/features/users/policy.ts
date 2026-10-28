import type { User } from '@/auth/types'

/**
 * An admin cannot delete themselves or remove their own admin flag. This mirrors the self
 * rules of the API's users.Lockout and only decides which controls are offered; the API
 * enforces them, and also refuses any change that would leave no admin.
 */
export function canDeleteOrDemote(actor: User | null, target: { id: number }): boolean {
  return actor != null && actor.id !== target.id
}
