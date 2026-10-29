import type { Folder, Scan } from './types'

/** A queued or running scan: the card shows its progress and offers no second one. */
export function isScanActive(scan: Scan | null | undefined): boolean {
  return scan?.status === 'queued' || scan?.status === 'running'
}

/** Images the index holds that are still on disk: every status but gone. */
export function indexedImages(folder: Pick<Folder, 'image_counts'>): number {
  return Object.entries(folder.image_counts)
    .filter(([status]) => status !== 'gone')
    .reduce((sum, [, count]) => sum + count, 0)
}

/** How many of the folder's images are in one status. */
export function imagesIn(folder: Pick<Folder, 'image_counts'>, status: string): number {
  return folder.image_counts[status] ?? 0
}
