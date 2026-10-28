import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { TOP_NAV, USER_MENU } from './nav'
import { selectNavKey, visibleNav, type NavItem } from './navMatch'

const items: NavItem[] = [
  { key: 'home', labelKey: '', icon: null, testId: '', path: '/' },
  { key: 'reports', labelKey: '', icon: null, testId: '', path: '/reports' },
  { key: 'users', labelKey: '', icon: null, testId: '', adminOnly: true, path: '/users' },
]

describe('visibleNav', () => {
  it('shows admin-only items to admins alone', () => {
    assert.deepEqual(visibleNav(items, false).map((i) => i.key), ['home', 'reports'])
    assert.deepEqual(visibleNav(items, true).map((i) => i.key), ['home', 'reports', 'users'])
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

  it('shows folders and settings to every user', () => {
    assert.deepEqual(visibleNav(TOP_NAV, false).map((item) => item.key), ['dashboard', 'settings'])
  })

  it('offers user management only to admins', () => {
    assert.deepEqual(visibleNav(USER_MENU, false).map((item) => item.key), ['profile'])
    assert.deepEqual(visibleNav(USER_MENU, true).map((item) => item.key), ['profile', 'users'])
  })
})
