import type { DaySavings } from './types'

const UNIT = 1024
const STEPS = [1, 2, 5, 10]

/**
 * Gridline values for a byte axis, from 0 to the first round value at or above `max`: two to four
 * steps of 1, 2 or 5 in the unit `formatBytes` shows `max` in, so every label is a round number.
 */
export function byteTicks(max: number): number[] {
  if (!(max > 0)) return [0]
  let unit = 1
  while (max / unit >= UNIT && unit < UNIT ** 4) unit *= UNIT
  const value = max / unit
  const rough = value / 4
  const magnitude = 10 ** Math.floor(Math.log10(rough))
  const multiple = STEPS.find((step) => step * magnitude >= rough) ?? 10
  // Bytes are whole: a 0.5 B step would label two gridlines alike.
  const step = unit === 1 ? Math.max(1, multiple * magnitude) : multiple * magnitude
  const count = Math.ceil(value / step - 1e-9)
  return Array.from({ length: count + 1 }, (_, i) => Math.round(i * step * unit))
}

export function totalSaved(days: readonly DaySavings[]): number {
  return days.reduce((total, day) => total + day.bytes_saved, 0)
}
