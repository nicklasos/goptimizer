import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { ApiError } from '../../api/errors'
import {
  translateServerError,
  errorMessage,
  mapValidationDetailsToFields,
  splitFieldErrors,
} from './serverErrors'

const t = (key: string, opts: Record<string, unknown> = {}) => {
  if (key === 'errors.fallback') return 'Something went wrong'
  if (key === 'errors.auth.invalid_credentials') return 'Invalid email or password'
  if (key === 'errors.validation.title.required') return 'Title is required'
  if (key === 'errors.validation.required') return 'Required field'
  return (opts.defaultValue as string | undefined) ?? key
}

describe('translateServerError', () => {
  it('returns fallback when errorKey missing', () => {
    assert.equal(translateServerError(t, null), 'Something went wrong')
  })

  it('translates known error keys', () => {
    assert.equal(translateServerError(t, 'auth.invalid_credentials'), 'Invalid email or password')
  })

  it('uses server message when no translation', () => {
    assert.equal(translateServerError(t, 'unknown.key', 'Server says no'), 'Server says no')
  })
})

describe('errorMessage', () => {
  it('translates an ApiError by key', () => {
    const error = new ApiError(401, 'auth.invalid_credentials', 'x')
    assert.equal(errorMessage(t, error), 'Invalid email or password')
  })

  it('prefers the first validation key', () => {
    const error = new ApiError(400, 'validation.failed', 'invalid', { title: ['validation.title.required'] })
    assert.equal(errorMessage(t, error), 'Title is required')
  })

  it('falls back for non-API errors', () => {
    assert.equal(errorMessage(t, new Error('boom')), 'Something went wrong')
  })
})

describe('mapValidationDetailsToFields', () => {
  it('maps field keys to form errors', () => {
    assert.deepEqual(mapValidationDetailsToFields(t, { title: ['validation.title.required'] }), [
      { name: 'title', errors: ['Title is required'] },
    ])
  })

  it('returns empty array for missing details', () => {
    assert.deepEqual(mapValidationDetailsToFields(t, null), [])
  })
})

describe('translateValidationKey', () => {
  it('falls back to the generic rule', () => {
    const fields = mapValidationDetailsToFields(t, { code: ['validation.code.required'] })
    assert.deepEqual(fields, [{ name: 'code', errors: ['Required field'] }])
  })
})

describe('splitFieldErrors', () => {
  it('separates known form fields from nested ones', () => {
    const error = new ApiError(400, 'validation.failed', 'invalid', {
      title: ['validation.title.required'],
      age_group_id: ['validation.age_group_id.required'],
    })
    const result = splitFieldErrors(t, error, ['title'])
    assert.deepEqual(result.fields, [{ name: 'title', errors: ['Title is required'] }])
    assert.deepEqual(result.other, ['Required field'])
  })
})
