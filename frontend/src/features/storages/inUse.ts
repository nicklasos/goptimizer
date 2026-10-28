import { isApiError } from '@/api/errors'

export interface PlanUsingStorage {
  id: number
  name: string
}

/** The plans named by a 409 storages.in_use, or null for any other error. */
export function plansUsingStorage(error: unknown): PlanUsingStorage[] | null {
  if (!isApiError(error) || error.errorKey !== 'storages.in_use') return null
  const plans = error.details?.plans
  if (!Array.isArray(plans)) return []
  return plans.filter((plan): plan is PlanUsingStorage => typeof plan?.id === 'number' && typeof plan?.name === 'string')
}
