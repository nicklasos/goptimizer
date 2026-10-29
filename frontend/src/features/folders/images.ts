import type { TagProps } from 'antd'
import type { Image, ImageOrder, ImageStatus } from './types'

/** Every image status in the order the folder page lists them, with the colour of its tag. */
export const IMAGE_STATUS = {
  pending: { color: 'default' },
  optimizing: { color: 'processing' },
  optimized: { color: 'success' },
  kept: { color: 'default' },
  skipped: { color: 'default' },
  failed: { color: 'error' },
  gone: { color: 'default' },
} as const satisfies Record<ImageStatus, { color: TagProps['color'] }>

export const IMAGE_STATUSES = Object.keys(IMAGE_STATUS) as ImageStatus[]

export function isImageStatus(value: string | null): value is ImageStatus {
  return value !== null && Object.hasOwn(IMAGE_STATUS, value)
}

/** The orders a page can ask for. A search is always listed by path, whatever it asks. */
export type ImageSort = Exclude<ImageOrder, 'path'>

/** Which of a folder's images the table lists: one status or every one (null), a path search, an order. */
export interface ImageFilters {
  status: ImageStatus | null
  q: string
  sort: ImageSort
}

/** Most saved is only for optimized images: the API refuses it for any other list. */
function sortFor(status: ImageStatus | null, sort: string | null): ImageSort {
  return sort === 'saved' && status === 'optimized' ? 'saved' : 'updated'
}

/** The filters in the URL. A status the API does not know means every status; a sort it would refuse means latest change. */
export function readImageFilters(search: URLSearchParams): ImageFilters {
  const raw = search.get('status')
  const status = isImageStatus(raw) ? raw : null
  return { status, q: search.get('q') ?? '', sort: sortFor(status, search.get('sort')) }
}

/**
 * The URL with some filters changed: the rest are read from the URL itself, so two changes in a row
 * never undo each other. Other parameters stay as they are; defaults are left out.
 */
export function writeImageFilters(search: URLSearchParams, change: Partial<ImageFilters>): URLSearchParams {
  const filters = { ...readImageFilters(search), ...change }
  const next = new URLSearchParams(search)
  const put = (key: string, value: string | null) => (value ? next.set(key, value) : next.delete(key))
  put('status', filters.status)
  put('q', filters.q)
  put('sort', sortFor(filters.status, filters.sort) === 'saved' ? 'saved' : null)
  return next
}

export interface ImageSizes {
  before: number
  /** The size after optimizing; null for an image that was not optimized. */
  after: number | null
  savedPercent: number | null
}

/** Before and after for the size column: an optimized image's original size and its size now, any other image's size on disk. */
export function imageSizes(image: Pick<Image, 'status' | 'size' | 'original_size'>): ImageSizes {
  if (image.status !== 'optimized' || image.original_size == null) return { before: image.size, after: null, savedPercent: null }
  const before = image.original_size
  return { before, after: image.size, savedPercent: before > 0 ? ((before - image.size) / before) * 100 : null }
}
