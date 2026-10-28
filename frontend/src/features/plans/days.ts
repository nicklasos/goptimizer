import type { ColorToken } from './health'
import type { Day } from './types'

/** A day of the 30-day strip: red when any run failed, green when runs only succeeded, grey with none. */
export type DayKind = 'failed' | 'succeeded' | 'none'

export const DAY_COLOR: Record<DayKind, ColorToken> = {
  failed: 'colorError',
  succeeded: 'colorSuccess',
  none: 'colorFillSecondary',
}

export function dayKind(day: Day): DayKind {
  if (day.failed > 0) return 'failed'
  if (day.succeeded > 0) return 'succeeded'
  return 'none'
}
