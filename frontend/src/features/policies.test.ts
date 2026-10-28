import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type { User } from '@/auth/types'
import { canDeleteOrDemote } from './users/policy'

const admin: User = { id: 1, is_admin: true, email: 'u1@example.com', name: 'User 1', email_verified: true }

describe('users policy', () => {
  it('never offers deleting or demoting your own account', () => {
    assert.equal(canDeleteOrDemote(admin, { id: admin.id }), false)
    assert.equal(canDeleteOrDemote(admin, { id: 99 }), true)
    assert.equal(canDeleteOrDemote(null, { id: 99 }), false)
  })
})
