import { enumLabel } from '@/shared/utils/enums'
import { formatNumber, formatRelative } from '@/shared/utils/format'
import type { Translate } from '@/shared/utils/serverErrors'
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

/** How long a scan took, or how long a running one has run so far. Null before it starts. */
export function scanDuration(scan: Pick<Scan, 'started_at' | 'finished_at'>, now: Date = new Date()): number | null {
  if (!scan.started_at) return null
  const end = scan.finished_at ? new Date(scan.finished_at) : now
  return Math.max(0, end.getTime() - new Date(scan.started_at).getTime())
}

/** The last scan in one line: queued, its progress, what it found, or why it failed. */
export function scanLine(t: Translate, scan: Scan | null | undefined): string {
  if (!scan) return t('folders.scan.never')
  const kind = enumLabel(t, 'scanKind', scan.kind)
  const ago = formatRelative(scan.finished_at)
  const files = { count: scan.files_seen, files: formatNumber(scan.files_seen, 0) }
  switch (scan.status) {
    case 'queued':
      return t('folders.scan.queued', { kind })
    case 'running':
      return t('folders.scan.running', files)
    case 'failed':
      return t('folders.scan.failed', { kind, ago, error: scan.error })
    case 'succeeded':
      return scan.files_seen > 0 ? t('folders.scan.succeeded', { kind, ago, ...files }) : t('folders.scan.unchanged', { kind, ago })
  }
}
