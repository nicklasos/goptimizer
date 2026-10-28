import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { hasAnyRole, isSuperAdmin, ROLE_ADMIN, ROLE_SUPER_ADMIN, ROLE_USER } from './roles'

describe('isSuperAdmin', () => {
  it('is true only for the super-admin role', () => {
    assert.equal(isSuperAdmin({ roles: [ROLE_SUPER_ADMIN] }), true)
    assert.equal(isSuperAdmin({ roles: [ROLE_ADMIN] }), false)
    assert.equal(isSuperAdmin({ roles: null }), false)
    assert.equal(isSuperAdmin(null), false)
  })
})

describe('hasAnyRole', () => {
  it('matches one of the required roles', () => {
    assert.equal(hasAnyRole({ roles: [ROLE_ADMIN] }, [ROLE_ADMIN]), true)
    assert.equal(hasAnyRole({ roles: [ROLE_USER] }, [ROLE_ADMIN]), false)
    assert.equal(hasAnyRole({ roles: [ROLE_USER, ROLE_ADMIN] }, [ROLE_ADMIN]), true)
  })

  it('lets a super admin through every requirement', () => {
    assert.equal(hasAnyRole({ roles: [ROLE_SUPER_ADMIN] }, [ROLE_ADMIN]), true)
    assert.equal(hasAnyRole({ roles: [ROLE_SUPER_ADMIN] }, []), true)
  })

  it('rejects a missing user or missing roles', () => {
    assert.equal(hasAnyRole(null, [ROLE_USER]), false)
    assert.equal(hasAnyRole({}, [ROLE_USER]), false)
    assert.equal(hasAnyRole({ roles: [] }, [ROLE_USER]), false)
  })
})
