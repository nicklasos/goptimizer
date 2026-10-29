import { api } from '@/api/client'
import type { ID, MessageResponse, PageParams } from '@/api/types'
import type { ImageFilters } from './images'
import type { DaySavings, Folder, FolderRequest, Image, ImagesPage, Scan, ScanKind } from './types'

export const foldersApi = {
  list: () => api.get<Folder[]>('/folders'),
  get: (id: ID) => api.get<Folder>(`/folders/${id}`),
  create: (body: FolderRequest) => api.post<Folder>('/folders', body),
  update: (id: ID, body: FolderRequest) => api.put<Folder>(`/folders/${id}`, body),
  setEnabled: (id: ID, enabled: boolean) => api.put<Folder>(`/folders/${id}/enabled`, { enabled }),
  remove: (id: ID) => api.del<MessageResponse>(`/folders/${id}`),
  startScan: (id: ID, kind: ScanKind) => api.post<Scan>(`/folders/${id}/scans`, { kind }),
  scans: (id: ID, params: PageParams) => api.getPage<Scan>(`/folders/${id}/scans`, { query: params }),
  images: (id: ID, { status, q, sort }: ImageFilters, cursor: string | undefined, signal: AbortSignal) =>
    api.get<ImagesPage>(`/folders/${id}/images`, { query: { status, q, sort: sort === 'updated' ? undefined : sort, cursor }, signal }),
  retryImage: (id: ID) => api.post<Image>(`/images/${id}/retry`),
  retryFailedImages: (folderId: ID) => api.post<{ retried: number }>(`/folders/${folderId}/images/retry-failed`),
  dailySavings: (folderId?: ID) => api.get<DaySavings[]>('/daily-savings', { query: { folder_id: folderId } }),
}
