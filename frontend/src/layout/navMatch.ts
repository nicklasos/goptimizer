import type { ReactNode } from 'react'

export interface NavItem {
  key: string
  labelKey: string
  icon: ReactNode
  testId: string
  adminOnly?: boolean
  path: string
}

type Target = Pick<NavItem, 'key' | 'path'>

export function visibleNav(items: readonly NavItem[], isAdmin: boolean): NavItem[] {
  return items.filter((item) => !item.adminOnly || isAdmin)
}

/** The item whose path is the longest prefix of the current location. */
export function selectNavKey(items: readonly Target[], pathname: string): string | undefined {
  let best: { key: string; length: number } | undefined
  for (const { key, path } of items) {
    const matches = path === '/' ? pathname === '/' : pathname === path || pathname.startsWith(`${path}/`)
    if (matches && (!best || path.length > best.length)) {
      best = { key, length: path.length }
    }
  }
  return best?.key
}
