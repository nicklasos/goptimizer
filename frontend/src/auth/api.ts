import { api } from '@/api/client'
import type { MessageResponse } from '@/api/types'

const noRefresh = { skipAuthRefresh: true }

export const authApi = {
  forgotPassword: (email: string) => api.post<MessageResponse>('/auth/forgot-password', { email }, noRefresh),
  resetPassword: (token: string, password: string) => api.post<MessageResponse>('/auth/reset-password', { token, password }, noRefresh),
  verifyEmail: (token: string) => api.post<MessageResponse>('/auth/verify-email', { token }, noRefresh),
  resendVerification: () => api.post<MessageResponse>('/auth/me/verify-email'),
}
