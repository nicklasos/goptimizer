import type { InternalPaginationMeta } from './schema'

/*
 * `schema.ts` is generated from the backend's OpenAPI file (`make api-types`). Features
 * give its types short names in their own `types.ts`; nothing else imports it directly.
 */

export type ID = number

export interface Timestamps {
  created_at: string
  updated_at: string
}

export interface MessageResponse {
  message: string
}

export type PaginationMeta = InternalPaginationMeta

/** A paginated list response: `api.getPage` keeps `pagination`, which `api.get` drops. */
export interface Page<T> {
  data: T[]
  pagination: PaginationMeta
}

export type PageParams = {
  page: number
  page_size: number
}
