import { useMutation } from '@tanstack/react-query'
import { authApi } from './api'

const silent = { silent: true }

export function useForgotPassword() {
  return useMutation({ mutationFn: (email: string) => authApi.forgotPassword(email), meta: silent })
}

export function useResetPassword() {
  return useMutation({
    mutationFn: ({ token, password }: { token: string; password: string }) => authApi.resetPassword(token, password),
    meta: silent,
  })
}

export function useVerifyEmail() {
  return useMutation({ mutationFn: (token: string) => authApi.verifyEmail(token), meta: silent })
}

export function useResendVerification() {
  return useMutation({ mutationFn: () => authApi.resendVerification() })
}
