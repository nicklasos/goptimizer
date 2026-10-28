import type { ComponentType } from 'react'
import { DatabaseOutlined, FileOutlined, FolderOutlined } from '@ant-design/icons'
import type { Plan, SourceKind } from './types'

export const SOURCE_ICON: Record<SourceKind, ComponentType> = {
  postgres: DatabaseOutlined,
  mysql: DatabaseOutlined,
  sqlite: FileOutlined,
  files: FolderOutlined,
}

/** What the plan backs up, in one line: "shop@db:5432/shop", a file path, or the paths. */
export function sourceSummary(plan: Plan): string {
  const db = plan.postgres ?? plan.mysql
  if (db) return `${db.user}@${db.host}:${db.port}/${db.database}`
  if (plan.sqlite) return plan.sqlite.path
  return plan.files?.paths.join(', ') ?? ''
}

/** The folder an archive is in, for the storage browser: "shop/" for "shop/shop-1.db.gz". */
export function objectFolder(key: string): string {
  const slash = key.lastIndexOf('/')
  return slash < 0 ? '' : key.slice(0, slash + 1)
}

export function objectName(key: string): string {
  return key.slice(key.lastIndexOf('/') + 1)
}

/** The storage browser at a folder. */
export const browseUrl = (storageId: number, prefix: string) => `/storages/${storageId}/browse?prefix=${encodeURIComponent(prefix)}`
