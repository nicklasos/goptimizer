import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { buildUrl } from './url'

describe('buildUrl', () => {
  it('joins base and path', () => {
    assert.equal(buildUrl('http://api/api/v1', '/users'), 'http://api/api/v1/users')
  })

  it('adds query values and skips empty ones', () => {
    const url = buildUrl('', '/users', { role: 'admin', page: 2, search: '', flag: false, missing: undefined, none: null })
    assert.equal(url, '/users?role=admin&page=2&flag=false')
  })

  it('encodes values', () => {
    assert.equal(buildUrl('', '/users', { q: 'a b&c' }), '/users?q=a+b%26c')
  })
})
