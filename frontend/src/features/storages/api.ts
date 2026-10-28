import { api } from '@/api/client'
import type { ID, MessageResponse, PageParams } from '@/api/types'
import type { Storage, StorageObject, StorageRequest, StorageTestResult } from './types'

export const storagesApi = {
  list: (params: PageParams) => api.getPage<Storage>('/storages', { query: params }),
  get: (id: ID) => api.get<Storage>(`/storages/${id}`),
  create: (body: StorageRequest) => api.post<Storage>('/storages', body),
  update: (id: ID, body: StorageRequest) => api.put<Storage>(`/storages/${id}`, body),
  remove: (id: ID) => api.del<MessageResponse>(`/storages/${id}`),
  /** Tests a form; on an edit form `storageId` lets blank credentials mean the stored ones. */
  test: (body: StorageRequest, storageId?: ID) => api.post<StorageTestResult>('/storages/test', body, { query: { storage_id: storageId } }),
  objects: (id: ID, prefix: string) => api.get<StorageObject[]>(`/storages/${id}/objects`, { query: { prefix } }),
}
