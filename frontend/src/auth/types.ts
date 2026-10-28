import type { InternalAuthLoginResponse, InternalAuthRefreshTokenResponse, InternalAuthUserResponse } from '@/api/schema'

export type User = InternalAuthUserResponse
export type AuthTokens = InternalAuthRefreshTokenResponse
export type LoginResponse = InternalAuthLoginResponse
