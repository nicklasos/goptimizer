import { api } from '@/api/client'
import type { ID, MessageResponse } from '@/api/types'
import type { Folder, FolderRequest } from './types'

export const foldersApi = {
  list: () => api.get<Folder[]>('/folders'),
  get: (id: ID) => api.get<Folder>(`/folders/${id}`),
  create: (body: FolderRequest) => api.post<Folder>('/folders', body),
  update: (id: ID, body: FolderRequest) => api.put<Folder>(`/folders/${id}`, body),
  setEnabled: (id: ID, enabled: boolean) => api.put<Folder>(`/folders/${id}/enabled`, { enabled }),
  remove: (id: ID) => api.del<MessageResponse>(`/folders/${id}`),
}
