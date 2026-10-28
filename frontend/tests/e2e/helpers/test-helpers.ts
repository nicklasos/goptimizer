import type { Page } from '@playwright/test'
import dbHelper, { type TestRole, type TestUser } from './db-helper'

const TEST_BACKEND_PORT = process.env.TEST_BACKEND_PORT || '8184'
export const API_BASE = process.env.VITE_E2E_API_BASE_URL || `http://localhost:${TEST_BACKEND_PORT}/api/v1`

export const AUTH_STORAGE_KEY = 'gogo-auth'

/** Signs the page in without the login form: the session is put into localStorage before the app loads. */
export async function loginViaAPI(page: Page, email: string, password: string): Promise<void> {
  const response = await page.request.post(`${API_BASE}/auth/login`, { data: { email, password } })
  if (!response.ok()) {
    throw new Error(`Login failed: ${response.status()} ${await response.text()}`)
  }

  const body = await response.json()
  const session = body.data ?? body

  await page.addInitScript(
    ({ key, payload }) => {
      if (localStorage.getItem(key)) return
      localStorage.setItem(
        key,
        JSON.stringify({
          state: {
            token: payload.access_token,
            refreshToken: payload.refresh_token,
            user: payload.user,
            isAuthenticated: true,
          },
          version: 0,
        })
      )
    },
    { key: AUTH_STORAGE_KEY, payload: session }
  )
}

/** Creates a user with the given role and signs the page in as them. */
export async function loginAs(page: Page, role: TestRole, options: { emailVerified?: boolean } = {}): Promise<TestUser> {
  const user = await dbHelper.createUser({ roles: [role], name: `E2E ${role}`, ...options })
  await loginViaAPI(page, user.email, user.password)
  return user
}

export { dbHelper }
