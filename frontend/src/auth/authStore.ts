import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api, bindAuth } from '@/api/client'
import { isApiError } from '@/api/errors'
import type { MessageResponse } from '@/api/types'
import type { AuthTokens, LoginResponse, User } from './types'

export const AUTH_STORAGE_KEY = 'gogo-auth'

export type AuthError =
  | { type: 'api'; errorKey: string; message: string }
  | { type: 'rate_limit'; retryAfterSeconds: number }
  | { type: 'raw'; message: string }

type Result<T = object> = ({ success: true } & T) | { success: false; error?: AuthError; unauthorized?: boolean }

interface AuthState {
  user: User | null
  token: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  loading: boolean
  error: AuthError | null
  login: (email: string, password: string) => Promise<Result>
  logout: () => Promise<void>
  me: () => Promise<Result<{ user: User }>>
  refreshAccessToken: () => Promise<Result<{ token: string }>>
  setUser: (user: User) => void
  clearError: () => void
}

const signedOut = {
  user: null,
  token: null,
  refreshToken: null,
  isAuthenticated: false,
}

function toAuthError(error: unknown): AuthError {
  if (isApiError(error) && error.status === 429) {
    return { type: 'rate_limit', retryAfterSeconds: Number(error.details?.retry_after_seconds) || 60 }
  }
  if (isApiError(error)) return { type: 'api', errorKey: error.errorKey, message: error.message }
  return { type: 'raw', message: error instanceof Error ? error.message : String(error) }
}

/** The server rejected the session itself, as opposed to being unreachable or busy. */
function isSessionRejected(error: unknown): boolean {
  return isApiError(error) && [400, 401, 403].includes(error.status)
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      ...signedOut,
      loading: false,
      error: null,

      login: async (email, password) => {
        set({ loading: true, error: null })
        try {
          const session = await api.post<LoginResponse>('/auth/login', { email, password }, { skipAuthRefresh: true })
          set({
            user: session.user,
            token: session.access_token,
            refreshToken: session.refresh_token,
            isAuthenticated: true,
            loading: false,
            error: null,
          })
          return { success: true }
        } catch (err) {
          const error = toAuthError(err)
          set({ ...signedOut, loading: false, error })
          return { success: false, error }
        }
      },

      logout: async () => {
        if (get().token) {
          await api.post<MessageResponse>('/auth/logout', undefined, { skipAuthRefresh: true }).catch(() => {})
        }
        set({ ...signedOut, error: null })
      },

      me: async () => {
        try {
          const user = await api.get<User>('/auth/me')
          set({ user, isAuthenticated: true })
          return { success: true, user }
        } catch (err) {
          if (isApiError(err) && err.status === 401) {
            set({ ...signedOut })
            return { success: false, unauthorized: true }
          }
          return { success: false, error: toAuthError(err) }
        }
      },

      refreshAccessToken: async () => {
        const staleToken = get().token

        // Another tab may have rotated the tokens already: refresh tokens are single-use,
        // so reusing the stale one would fail and sign this tab out.
        await useAuthStore.persist.rehydrate()
        const { token, refreshToken } = get()
        if (token && token !== staleToken) return { success: true, token }

        if (!refreshToken) {
          set({ ...signedOut })
          return { success: false, unauthorized: true }
        }

        try {
          const tokens = await api.post<AuthTokens>('/auth/refresh', { refresh_token: refreshToken }, { skipAuthRefresh: true })
          set({ token: tokens.access_token, refreshToken: tokens.refresh_token || refreshToken, error: null })
          return { success: true, token: tokens.access_token }
        } catch (err) {
          // A network error, 429 or 5xx says nothing about the session, so keep it.
          if (isSessionRejected(err)) {
            set({ ...signedOut })
            return { success: false, unauthorized: true }
          }
          return { success: false, error: toAuthError(err) }
        }
      },

      setUser: (user) => set({ user }),
      clearError: () => set({ error: null }),
    }),
    {
      name: AUTH_STORAGE_KEY,
      partialize: (state) => ({
        token: state.token,
        refreshToken: state.refreshToken,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)

bindAuth({
  getToken: () => useAuthStore.getState().token,
  refresh: () =>
    useAuthStore
      .getState()
      .refreshAccessToken()
      .then((result) => result.success),
})

export function useCurrentUser(): User | null {
  return useAuthStore((s) => s.user)
}
