import type { FormInstance } from 'antd'
import { isApiError } from '@/api/errors'
import { errorMessage, splitFieldErrors, type Translate } from './serverErrors'

interface Options {
  /** Errors that are not validation errors but belong to one field, by error key: `{ 'auth.user_exists': 'email' }`. */
  keyFields?: Record<string, string>
}

/**
 * Puts a failed save onto a form. Errors for the given fields are shown under those
 * fields. Returns the message for whatever could not be placed on a field, or null when
 * everything was, so the caller decides where that message goes (an alert, a toast).
 */
export function applyFormErrors<Values>(
  form: FormInstance<Values>,
  t: Translate,
  error: unknown,
  fields: readonly string[],
  { keyFields = {} }: Options = {}
): string | null {
  const { fields: fieldErrors, other } = splitFieldErrors(t, error, fields)

  const keyField = isApiError(error) ? keyFields[error.errorKey] : undefined
  if (keyField) fieldErrors.push({ name: keyField, errors: [errorMessage(t, error)] })

  // "address.city" is the API's path to a field of a nested block; the form names it ['address', 'city'],
  // and a row of a list by its index: "items.1.name" is ['items', 1, 'name'].
  const named = fieldErrors.map(({ name, errors }) => ({ name: name.includes('.') ? fieldPath(name) : name, errors }))
  if (named.length) form.setFields(named as Parameters<typeof form.setFields>[0])
  if (other.length) return other.join(' ')
  return named.length ? null : errorMessage(t, error)
}

const fieldPath = (name: string) => name.split('.').map((part) => (/^\d+$/.test(part) ? Number(part) : part))
