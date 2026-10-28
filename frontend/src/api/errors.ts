export type FieldErrors = Record<string, string[]>

export class ApiError extends Error {
  readonly status: number
  readonly errorKey: string
  readonly fieldErrors?: FieldErrors
  /** Extra data some errors carry, e.g. `retry_after_seconds` on a 429. */
  readonly details?: Record<string, unknown>

  constructor(status: number, errorKey: string, message: string, fieldErrors?: FieldErrors, details?: Record<string, unknown>) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.errorKey = errorKey
    this.fieldErrors = fieldErrors
    this.details = details
  }

  static fromBody(status: number, body: unknown): ApiError {
    const data = (body && typeof body === 'object' ? body : {}) as Record<string, unknown>
    const errorKey = typeof data.error_key === 'string' ? data.error_key : ''
    const message = typeof data.message === 'string' ? data.message : ''
    const fieldErrors = isFieldErrors(data.errors) ? data.errors : undefined
    const details = data.details && typeof data.details === 'object' ? (data.details as Record<string, unknown>) : undefined
    return new ApiError(status, errorKey, message, fieldErrors, details)
  }
}

function isFieldErrors(value: unknown): value is FieldErrors {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  return Object.values(value).every((list) => Array.isArray(list))
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError
}
