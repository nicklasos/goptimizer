import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { retryAfterMessage } from './retryAfter'

const t = (key: string, options?: Record<string, unknown>) => (options ? `${key}:${options.count}` : key)

describe('retryAfterMessage', () => {
  it('says a minute for anything up to 60 seconds', () => {
    assert.equal(retryAfterMessage(t, 1), 'auth.tooManyAttempts.minute')
    assert.equal(retryAfterMessage(t, 60), 'auth.tooManyAttempts.minute')
  })

  it('rounds minutes up', () => {
    assert.equal(retryAfterMessage(t, 61), 'auth.tooManyAttempts.minutes:2')
    assert.equal(retryAfterMessage(t, 300), 'auth.tooManyAttempts.minutes:5')
    assert.equal(retryAfterMessage(t, 3599), 'auth.tooManyAttempts.minutes:60'.replace('minutes:60', 'hours:1'))
  })

  it('switches to hours from 60 minutes and rounds up', () => {
    assert.equal(retryAfterMessage(t, 3600), 'auth.tooManyAttempts.hours:1')
    assert.equal(retryAfterMessage(t, 3601), 'auth.tooManyAttempts.hours:2')
    assert.equal(retryAfterMessage(t, 86400), 'auth.tooManyAttempts.hours:24')
  })
})
