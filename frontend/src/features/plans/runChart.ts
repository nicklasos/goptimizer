import type { Run } from './types'

export interface Bar {
  run: Run
  x: number
  y: number
  width: number
  height: number
}

export interface ChartBox {
  width: number
  height: number
  /** Slots across the chart; the newest run takes the rightmost. */
  slots: number
  gap: number
  /** The height of a run with no archive size (failed, or not finished), and the least any bar gets. */
  minHeight: number
}

/**
 * The bars of the runs chart, oldest on the left: one slot per run, ending at the right edge,
 * with the height of each in proportion to the largest archive. `runs` is newest first, as the
 * API lists them; only the newest `slots` are drawn.
 */
export function chartBars(runs: readonly Run[], box: ChartBox): Bar[] {
  const shown = runs.slice(0, box.slots).reverse()
  const largest = Math.max(0, ...shown.map((run) => run.size_bytes ?? 0))
  const slot = box.width / box.slots
  const first = box.slots - shown.length
  return shown.map((run, i) => {
    const size = run.size_bytes ?? 0
    const height = largest > 0 && size > 0 ? Math.max(box.minHeight, (size / largest) * box.height) : box.minHeight
    return { run, x: (first + i) * slot, y: box.height - height, width: Math.max(1, slot - box.gap), height }
  })
}
