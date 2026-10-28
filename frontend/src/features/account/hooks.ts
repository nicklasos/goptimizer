import { useMutation } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { MessageResponse } from '@/api/types'
import { useAuthStore } from '@/auth/authStore'
import type { User } from '@/auth/types'

export function useUpdateProfile() {
  const setUser = useAuthStore((s) => s.setUser)
  return useMutation({
    mutationFn: (body: { name: string; email: string }) => api.put<User>('/auth/me', body),
    onSuccess: setUser,
  })
}

export { useResendVerification } from '@/auth/hooks'

export function useChangePassword() {
  return useMutation({
    mutationFn: (body: { current_password: string; new_password: string }) =>
      api.put<MessageResponse>('/auth/me/password', body),
  })
}
