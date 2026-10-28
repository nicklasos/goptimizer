import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { TOP_NAV, USER_MENU } from './nav'
import { selectNavKey, visibleNav, type NavItem } from './navMatch'

const items: NavItem[] = [
  { key: 'home', labelKey: '', icon: null, testId: '', roles: 'all', path: '/' },
  { key: 'reports', labelKey: '', icon: null, testId: '', roles: 'all', path: '/reports' },
  { key: 'users', labelKey: '', icon: null, testId: '', roles: ['admin'], path: '/users' },
]

describe('visibleNav', () => {
  it('filters by role', () => {
    assert.deepEqual(visibleNav(items, { roles: ['user'] }).map((i) => i.key), ['home', 'reports'])
    assert.deepEqual(visibleNav(items, { roles: ['admin'] }).map((i) => i.key), ['home', 'reports', 'users'])
  })

  it('shows everything to a super admin', () => {
    assert.equal(visibleNav(items, { roles: ['super-admin'] }).length, 3)
  })

  it('keeps only public items without a user', () => {
    assert.deepEqual(visibleNav(items, null).map((i) => i.key), ['home', 'reports'])
  })
})

describe('selectNavKey', () => {
  it('matches the root only exactly', () => {
    assert.equal(selectNavKey(items, '/'), 'home')
    assert.equal(selectNavKey(items, '/profile'), undefined)
  })

  it('matches nested paths', () => {
    assert.equal(selectNavKey(items, '/reports/12/edit'), 'reports')
  })

  it('does not match a path that only shares a prefix', () => {
    assert.equal(selectNavKey(items, '/reports-archive'), undefined)
  })

  it('picks the longest matching prefix', () => {
    const nested = [
      { key: 'reports', path: '/reports' },
      { key: 'exports', path: '/reports/exports' },
    ]
    assert.equal(selectNavKey(nested, '/reports/exports/3'), 'exports')
    assert.equal(selectNavKey(nested, '/reports/3'), 'reports')
  })
})

describe('the top bar', () => {
  it('highlights nothing on a page from the user menu', () => {
    for (const path of ['/profile', '/users']) assert.equal(selectNavKey(TOP_NAV, path), undefined)
  })

  it('offers user management only to admins', () => {
    const keys = (roles: string[]) => visibleNav(USER_MENU, { roles }).map((item) => item.key)
    assert.deepEqual(keys(['user']), ['profile'])
    assert.deepEqual(keys(['admin']), ['profile', 'users'])
    assert.deepEqual(keys(['super-admin']), ['profile', 'users'])
  })
})
