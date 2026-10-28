import { isApiError, type FieldErrors } from '@/api/errors'

export type Translate = (key: string, options?: Record<string, unknown>) => string

export interface FormFieldError {
  name: string
  errors: string[]
}

export function translateServerError(t: Translate, errorKey?: string | null, fallbackMessage?: string): string {
  if (!errorKey || typeof errorKey !== 'string') {
    return fallbackMessage || t('errors.fallback')
  }
  return t(`errors.${errorKey}`, {
    defaultValue: fallbackMessage || t('errors.fallback'),
  })
}

export function translateValidationKey(t: Translate, key: string): string {
  const rule = key.split('.').pop() ?? key
  const generic = t(`errors.validation.${rule}`, { defaultValue: key })
  return t(`errors.${key}`, { defaultValue: generic })
}

export function errorMessage(t: Translate, error: unknown, fallbackKey = 'errors.fallback'): string {
  if (isApiError(error)) {
    if (error.fieldErrors) {
      const first = Object.values(error.fieldErrors).flat()[0]
      if (first) return translateValidationKey(t, first)
    }
    return translateServerError(t, error.errorKey, error.message || t(fallbackKey))
  }
  return t(fallbackKey)
}

export function mapValidationDetailsToFields(t: Translate, details: FieldErrors | null | undefined): FormFieldError[] {
  if (!details || typeof details !== 'object') return []
  return Object.entries(details).map(([field, keys]) => {
    const list = Array.isArray(keys) ? keys : [keys]
    const messages = list.map((key) => translateValidationKey(t, String(key)))
    return { name: field, errors: messages }
  })
}

export function splitFieldErrors(
  t: Translate,
  error: unknown,
  formFields: readonly string[]
): { fields: FormFieldError[]; other: string[] } {
  if (!isApiError(error) || !error.fieldErrors) return { fields: [], other: [] }
  const fields: FormFieldError[] = []
  const other: string[] = []
  for (const entry of mapValidationDetailsToFields(t, error.fieldErrors)) {
    if (formFields.includes(entry.name)) fields.push(entry)
    else other.push(...entry.errors)
  }
  return { fields, other }
}
