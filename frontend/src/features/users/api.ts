import { api } from '@/api/client'
import type { ID, MessageResponse, PageParams } from '@/api/types'
import type { Role } from '@/auth/roles'
import type { ManagedUser, UserCreateRequest, UserUpdateRequest } from './types'

export const usersApi = {
  list: (role: Role, params: PageParams) => api.getPage<ManagedUser>('/users', { query: { role, ...params } }),
  create: (body: UserCreateRequest) => api.post<ManagedUser>('/users', body),
  update: (id: ID, body: UserUpdateRequest) => api.put<ManagedUser>(`/users/${id}`, body),
  setPassword: (id: ID, password: string) => api.post<MessageResponse>(`/users/${id}/set-password`, { password }),
  remove: (id: ID) => api.del<MessageResponse>(`/users/${id}`),
}
