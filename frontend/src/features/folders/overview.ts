import { imagesIn, isScanActive } from './scans'
import type { Folder } from './types'

/** What the folders add up to: the figures of the overview tiles. */
export interface Totals {
  bytesSaved: number
  optimized: number
  pending: number
  failed: number
  optimizing: number
  /** Folders with a scan queued or running. */
  scans: number
}

export function folderTotals(folders: readonly Folder[]): Totals {
  const sum = (count: (folder: Folder) => number) => folders.reduce((total, folder) => total + count(folder), 0)
  return {
    bytesSaved: sum((folder) => folder.bytes_saved),
    optimized: sum((folder) => imagesIn(folder, 'optimized')),
    pending: sum((folder) => imagesIn(folder, 'pending')),
    failed: sum((folder) => imagesIn(folder, 'failed')),
    optimizing: sum((folder) => imagesIn(folder, 'optimizing')),
    scans: sum((folder) => (isScanActive(folder.last_scan) ? 1 : 0)),
  }
}

export const POLL_WORKING = 2_000
export const POLL_WAITING = 10_000

/**
 * How often folder data refreshes on its own: fast while a scan or an optimization is in flight,
 * slowly while images wait for a worker that can take them, and not at all once nothing will change.
 */
export function pollInterval(folders: readonly Folder[]): number | false {
  if (folders.some((folder) => isScanActive(folder.last_scan) || imagesIn(folder, 'optimizing') > 0)) return POLL_WORKING
  const waiting = (folder: Folder) => folder.enabled && folder.missing_tools.length === 0 && imagesIn(folder, 'pending') > 0
  return folders.some(waiting) ? POLL_WAITING : false
}
