import { ApiError } from './errors'
import type { Page } from './types'
import { buildUrl, type Query } from './url'

export const API_BASE_URL = import.meta.env.VITE_API_URL.replace(/\/+$/, '')

export interface RequestOptions {
  query?: Query
  headers?: Record<string, string>
  signal?: AbortSignal
  /** Do not try to refresh the session on 401. For the auth endpoints themselves. */
  skipAuthRefresh?: boolean
}

interface AuthBridge {
  getToken: () => string | null
  refresh: () => Promise<boolean>
}

let auth: AuthBridge = { getToken: () => null, refresh: async () => false }

/** Called once by the auth store, so this module never imports it. */
export function bindAuth(bridge: AuthBridge): void {
  auth = bridge
}

let refreshing: Promise<boolean> | null = null

function refreshOnce(): Promise<boolean> {
  if (!refreshing) {
    refreshing = auth.refresh().finally(() => {
      refreshing = null
    })
  }
  return refreshing
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text()
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

function send(method: string, path: string, body: unknown, options: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = { ...options.headers }
  const token = auth.getToken()
  if (token) headers.Authorization = `Bearer ${token}`

  let payload: BodyInit | undefined
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }

  return fetch(buildUrl(API_BASE_URL, path, options.query), {
    method,
    headers,
    body: payload,
    signal: options.signal,
  })
}

async function exchange(method: string, path: string, body: unknown, options: RequestOptions): Promise<unknown> {
  let response = await send(method, path, body, options)

  if (response.status === 401 && !options.skipAuthRefresh && (await refreshOnce())) {
    response = await send(method, path, body, options)
  }

  const parsed = await readBody(response)
  if (!response.ok) {
    throw ApiError.fromBody(response.status, parsed)
  }
  return parsed
}

/** Sends a request and returns the `data` field of the response. Throws `ApiError` on a non-2xx status. */
export async function request<T>(method: string, path: string, body?: unknown, options: RequestOptions = {}): Promise<T> {
  const parsed = await exchange(method, path, body, options)
  if (parsed && typeof parsed === 'object' && 'data' in parsed) {
    return (parsed as { data: T }).data
  }
  return parsed as T
}

/** GET for a paginated list: returns `{ data, pagination }` as the backend sent it. */
export async function requestPage<T>(path: string, options: RequestOptions = {}): Promise<Page<T>> {
  return (await exchange('GET', path, undefined, options)) as Page<T>
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) => request<T>('GET', path, undefined, options),
  getPage: requestPage,
  post: <T>(path: string, body?: unknown, options?: RequestOptions) => request<T>('POST', path, body, options),
  put: <T>(path: string, body?: unknown, options?: RequestOptions) => request<T>('PUT', path, body, options),
  patch: <T>(path: string, body?: unknown, options?: RequestOptions) => request<T>('PATCH', path, body, options),
  del: <T>(path: string, options?: RequestOptions) => request<T>('DELETE', path, undefined, options),
}
