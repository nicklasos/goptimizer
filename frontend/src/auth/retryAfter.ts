import type { Translate } from '@/shared/utils/serverErrors'

/** "Try again in a minute / in N minutes / in N hours", rounded up so the wait is never understated. */
export function retryAfterMessage(t: Translate, seconds: number): string {
  if (seconds <= 60) return t('auth.tooManyAttempts.minute')
  const minutes = Math.ceil(seconds / 60)
  if (minutes < 60) return t('auth.tooManyAttempts.minutes', { count: minutes })
  return t('auth.tooManyAttempts.hours', { count: Math.ceil(minutes / 60) })
}
