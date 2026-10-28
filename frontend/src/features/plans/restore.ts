import type { Plan, RestoreRequest, RestoreTarget, SourceKind } from './types'

/** What an admin types to confirm a files restore over the plan's own paths. */
export const ORIGINAL_LOCATIONS = 'original locations'

type SSLMode = NonNullable<RestoreTarget['postgres']>['ssl_mode']

/** The restore form: the target block of the plan's kind, named as in the request. */
export interface RestoreFormValues {
  postgres?: { host: string; port?: number | null; user: string; database: string; ssl_mode: SSLMode; password?: string }
  mysql?: { host: string; port?: number | null; user: string; database: string; password?: string }
  sqlite?: { path: string }
  files?: { directory: string; original_locations: boolean }
  confirm?: string
}

export interface OwnTarget {
  /** The target is the plan's own source: restoring replaces what the plan backs up. */
  own: boolean
  /** What the admin types to confirm it: the database name, the file path or "original locations". */
  phrase: string
}

interface Server {
  host: string
  port: number
  database: string
}

/** localhost, 127.x and ::1 are one server, as on the API. */
function serverName(host: string): string {
  const name = host.trim().toLowerCase().replace(/^\[|\]$/g, '')
  return name === 'localhost' || name === '::1' || name.startsWith('127.') ? 'localhost' : name
}

/** An absolute path with `.`, `..` and repeated slashes resolved, like Go's filepath.Clean. */
export function cleanPath(path: string): string {
  const parts: string[] = []
  for (const part of path.split('/')) {
    if (part === '' || part === '.') continue
    if (part === '..') parts.pop()
    else parts.push(part)
  }
  return `/${parts.join('/')}`
}

function ownDatabase(source: object, target: object): OwnTarget {
  const s = source as Server
  const t = target as Server
  return { phrase: t.database, own: s.database === t.database && s.port === t.port && serverName(s.host) === serverName(t.host) }
}

const OWN: Record<SourceKind, (source: object, target: object) => OwnTarget> = {
  postgres: ownDatabase,
  mysql: ownDatabase,
  sqlite: (source, target) => {
    const path = (source as { path: string }).path
    return { phrase: path, own: cleanPath(path) === cleanPath((target as { path: string }).path) }
  },
  files: (_, target) => ({ phrase: ORIGINAL_LOCATIONS, own: (target as { original_locations: boolean }).original_locations }),
}

/**
 * Whether a target is the plan's own source, and the phrase that confirms it. The API decides
 * (plans.RestoreTargetIsOwn) and both run the same cases (backend testdata/own_target.json);
 * this only decides when the form asks for the confirmation.
 */
export function ownTarget(kind: SourceKind, source: object, target: object): OwnTarget {
  return OWN[kind](source, target)
}

/** The plan's source config, the block of its kind. */
export function planSource(plan: Plan): object {
  return plan.postgres ?? plan.mysql ?? plan.sqlite ?? plan.files ?? {}
}

/** The form's first values: the target the API proposes, with no password typed. */
export function restoreFormValues(target: RestoreTarget): RestoreFormValues {
  const { postgres, mysql, sqlite, files } = target
  if (postgres) return { postgres: { host: postgres.host, port: postgres.port, user: postgres.user, database: postgres.database, ssl_mode: postgres.ssl_mode } }
  if (mysql) return { mysql: { host: mysql.host, port: mysql.port, user: mysql.user, database: mysql.database } }
  if (sqlite) return { sqlite: { path: sqlite.path } }
  return { files: { directory: files?.directory ?? '', original_locations: false } }
}

/** The target the form names once the API fills its blanks from the plan, for the own-source check. */
export function formTarget(plan: Plan, values: RestoreFormValues): object {
  const db = values.postgres ?? values.mysql
  if (db) {
    const source = planSource(plan) as Server
    return { host: db.host?.trim() || source.host, port: db.port || source.port, database: (db.database ?? '').trim() }
  }
  if (values.sqlite) return { path: (values.sqlite.path ?? '').trim() }
  return { directory: (values.files?.directory ?? '').trim(), original_locations: values.files?.original_locations ?? false }
}

/**
 * The request for the form. A blank password is left out, so the plan's is used.
 * `createDatabase` undefined lets the API choose: yes for a database other than the plan's.
 */
export function restoreRequest(kind: SourceKind, values: RestoreFormValues, createDatabase: boolean | undefined, confirm: string | undefined): RestoreRequest {
  const request: RestoreRequest = confirm ? { confirm } : {}
  const db = kind === 'postgres' ? values.postgres : kind === 'mysql' ? values.mysql : undefined
  if (db) {
    const block = {
      host: db.host.trim(),
      port: db.port ?? undefined,
      user: db.user.trim(),
      database: db.database.trim(),
      ...(db.password ? { password: db.password } : {}),
      ...(createDatabase === undefined ? {} : { create_database: createDatabase }),
    }
    if (kind === 'postgres') request.postgres = { ...block, ssl_mode: values.postgres?.ssl_mode }
    else request.mysql = block
  }
  if (kind === 'sqlite') request.sqlite = { path: (values.sqlite?.path ?? '').trim() }
  if (kind === 'files') {
    const original = values.files?.original_locations ?? false
    request.files = original ? { original_locations: true } : { directory: (values.files?.directory ?? '').trim(), original_locations: false }
  }
  return request
}

/** Where a restore went, in one line; null for files put back where they were. */
export function targetSummary(target: RestoreTarget): string | null {
  const db = target.postgres ?? target.mysql
  if (db) return `${db.user}@${db.host}:${db.port}/${db.database}`
  if (target.sqlite) return target.sqlite.path
  if (target.files?.original_locations) return null
  return target.files?.directory ?? ''
}
