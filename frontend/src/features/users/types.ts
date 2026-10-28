import type { InternalUsersCreateUserRequest, InternalUsersUpdateUserRequest, InternalUsersUserResponse } from '@/api/schema'

export type ManagedUser = InternalUsersUserResponse
export type UserCreateRequest = InternalUsersCreateUserRequest
export type UserUpdateRequest = InternalUsersUpdateUserRequest

export interface UserFormValues {
  email: string
  name: string
  password?: string
}
