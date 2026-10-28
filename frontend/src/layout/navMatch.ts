import type { ReactNode } from 'react'
import { hasAnyRole, type Role, type RoleHolder } from '@/auth/roles'

export interface NavItem {
  key: string
  labelKey: string
  icon: ReactNode
  testId: string
  roles: readonly Role[] | 'all'
  path: string
  /** Other sections that belong to this item, such as the pages a dashboard links to. */
  alsoPaths?: readonly string[]
}

type Target = Pick<NavItem, 'key' | 'path' | 'alsoPaths'>

export function visibleNav(items: readonly NavItem[], user: RoleHolder | null): NavItem[] {
  return items.filter((item) => item.roles === 'all' || hasAnyRole(user, item.roles))
}

/** The item whose path is the longest prefix of the current location. */
export function selectNavKey(items: readonly Target[], pathname: string): string | undefined {
  let best: { key: string; length: number } | undefined
  for (const item of items) {
    for (const path of [item.path, ...(item.alsoPaths ?? [])]) {
      const matches = path === '/' ? pathname === '/' : pathname === path || pathname.startsWith(`${path}/`)
      if (matches && (!best || path.length > best.length)) {
        best = { key: item.key, length: path.length }
      }
    }
  }
  return best?.key
}
