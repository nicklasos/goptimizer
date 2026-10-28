import { normalizeDays } from './schedule'
import type { Plan, PlanRequest, SourceKind, SourceRequest, Weekday } from './types'

export type SslMode = 'disable' | 'require' | 'verify-full'

export const SSL_MODES: readonly SslMode[] = ['disable', 'require', 'verify-full']

export const DEFAULT_PORTS = { postgres: 5432, mysql: 3306 } as const

export interface ScheduleRow {
  time: string
  days: Weekday[]
}

/**
 * The form keeps a block per source kind, so switching kinds back and forth loses nothing typed.
 * Retention is switches plus numbers: a rule that is off sends 0, and the numbers are kept for
 * when it is switched back on.
 */
export interface PlanFormValues {
  name: string
  source_kind: SourceKind
  postgres: { host: string; port: number | null; user: string; database: string; ssl_mode: SslMode; password: string }
  mysql: { host: string; port: number | null; user: string; database: string; password: string }
  sqlite: { path: string }
  files: { paths: string[]; excludes: string[] }
  schedules: ScheduleRow[]
  prune: boolean
  max_age_days: number
  keep_weekly_on: boolean
  keep_weekly: number
  keep_monthly_on: boolean
  keep_monthly: number
  storage_ids: number[]
  enabled: boolean
}

export function emptyPlanForm(): PlanFormValues {
  return {
    name: '',
    source_kind: 'postgres',
    postgres: { host: '', port: DEFAULT_PORTS.postgres, user: '', database: '', ssl_mode: 'disable', password: '' },
    mysql: { host: '', port: DEFAULT_PORTS.mysql, user: '', database: '', password: '' },
    sqlite: { path: '' },
    files: { paths: [''], excludes: [] },
    schedules: [{ time: '03:00', days: [] }],
    prune: true,
    max_age_days: 30,
    keep_weekly_on: true,
    keep_weekly: 8,
    keep_monthly_on: true,
    keep_monthly: 12,
    storage_ids: [],
    enabled: true,
  }
}

/** The stored settings of a plan. Its password is never sent back, so that field starts blank. */
export function planToForm(plan: Plan): PlanFormValues {
  const values = emptyPlanForm()
  values.name = plan.name
  values.source_kind = plan.source_kind
  if (plan.postgres) values.postgres = { ...plan.postgres, password: '' }
  if (plan.mysql) values.mysql = { ...plan.mysql, password: '' }
  if (plan.sqlite) values.sqlite = { ...plan.sqlite }
  if (plan.files) values.files = { paths: [...plan.files.paths], excludes: [...plan.files.excludes] }
  values.schedules = plan.schedules.map((s) => ({ time: s.time, days: [...s.days] }))
  values.prune = plan.max_age_days > 0
  if (values.prune) values.max_age_days = plan.max_age_days
  values.keep_weekly_on = plan.keep_weekly > 0
  if (values.keep_weekly_on) values.keep_weekly = plan.keep_weekly
  values.keep_monthly_on = plan.keep_monthly > 0
  if (values.keep_monthly_on) values.keep_monthly = plan.keep_monthly
  values.storage_ids = plan.storages.map((s) => s.id)
  values.enabled = plan.enabled
  return values
}

/** The source block of the chosen kind only, as the API requires. Blank exclude rows are dropped. */
export function formToSource(values: PlanFormValues): SourceRequest {
  const kind = values.source_kind
  const request: SourceRequest = { source_kind: kind }
  if (kind === 'postgres') {
    const { port, ...rest } = values.postgres
    request.postgres = { ...rest, host: rest.host.trim(), port: port ?? undefined }
  }
  if (kind === 'mysql') {
    const { port, ...rest } = values.mysql
    request.mysql = { ...rest, host: rest.host.trim(), port: port ?? undefined }
  }
  if (kind === 'sqlite') request.sqlite = { path: values.sqlite.path.trim() }
  if (kind === 'files') {
    request.files = {
      paths: values.files.paths.map((p) => p.trim()),
      excludes: values.files.excludes.map((p) => p.trim()).filter(Boolean),
    }
  }
  return request
}

export interface Retention {
  max_age_days: number
  keep_weekly: number
  keep_monthly: number
}

export function formRetention(values: Pick<PlanFormValues, 'prune' | 'max_age_days' | 'keep_weekly_on' | 'keep_weekly' | 'keep_monthly_on' | 'keep_monthly'>): Retention {
  if (!values.prune) return { max_age_days: 0, keep_weekly: 0, keep_monthly: 0 }
  return {
    max_age_days: values.max_age_days,
    keep_weekly: values.keep_weekly_on ? values.keep_weekly : 0,
    keep_monthly: values.keep_monthly_on ? values.keep_monthly : 0,
  }
}

export function formToRequest(values: PlanFormValues): PlanRequest {
  return {
    name: values.name.trim(),
    ...formToSource(values),
    schedules: values.schedules.map((s) => ({ time: s.time, days: normalizeDays(s.days) })),
    ...formRetention(values),
    storage_ids: values.storage_ids,
    enabled: values.enabled,
  }
}

/** A password is stored for this kind, so a blank password field keeps it. */
export function hasStoredSecret(plan: Plan | undefined, kind: SourceKind): boolean {
  return plan != null && plan.source_kind === kind && plan.secret_set
}

/** The form fields of the chosen source, to validate before a test without the rest of the form. */
export function sourceFieldPaths(values: Pick<PlanFormValues, 'source_kind' | 'files'>): (string | number)[][] {
  switch (values.source_kind) {
    case 'postgres':
      return ['host', 'port', 'user', 'database', 'ssl_mode', 'password'].map((f) => ['postgres', f])
    case 'mysql':
      return ['host', 'port', 'user', 'database', 'password'].map((f) => ['mysql', f])
    case 'sqlite':
      return [['sqlite', 'path']]
    case 'files':
      return [
        ...values.files.paths.map((_, i) => ['files', 'paths', i]),
        ...values.files.excludes.map((_, i) => ['files', 'excludes', i]),
      ]
  }
}

/** Every field the API can report an error on, as `block.field` paths, for the rows the form has now. */
export function planFormFields(values: Pick<PlanFormValues, 'files' | 'schedules'>): string[] {
  return [
    'name',
    'source_kind',
    ...['host', 'port', 'user', 'database', 'ssl_mode', 'password'].map((f) => `postgres.${f}`),
    ...['host', 'port', 'user', 'database', 'password'].map((f) => `mysql.${f}`),
    'sqlite.path',
    ...values.files.paths.map((_, i) => `files.paths.${i}`),
    ...values.files.excludes.map((_, i) => `files.excludes.${i}`),
    ...values.schedules.flatMap((_, i) => [`schedules.${i}.time`, `schedules.${i}.days`]),
    'max_age_days',
    'keep_weekly',
    'keep_monthly',
    'storage_ids',
  ]
}

export interface ConnectionUrl {
  kind: 'postgres' | 'mysql'
  host: string
  port: number
  user: string
  password: string
  database: string
  /** Postgres only; libpq's modes mapped onto the three the API knows. */
  ssl_mode?: SslMode
}

const SSL_FROM_LIBPQ: Record<string, SslMode> = {
  disable: 'disable',
  allow: 'disable',
  prefer: 'disable',
  require: 'require',
  'verify-ca': 'require',
  'verify-full': 'verify-full',
}

const URL_KINDS: Record<string, ConnectionUrl['kind']> = { 'postgres:': 'postgres', 'postgresql:': 'postgres', 'mysql:': 'mysql' }

/**
 * Reads `postgres://user:pass@host:5432/db?sslmode=require` or `mysql://user:pass@host/db` into
 * the fields of the form. Null when it is not such a URL. The URL itself is never sent.
 */
export function parseConnectionUrl(raw: string): ConnectionUrl | null {
  let url: URL
  try {
    url = new URL(raw.trim())
  } catch {
    return null
  }
  const kind = URL_KINDS[url.protocol]
  const host = url.hostname.replace(/^\[(.*)\]$/, '$1')
  if (!kind || !host) return null

  const decode = (value: string) => {
    try {
      return decodeURIComponent(value)
    } catch {
      return value
    }
  }
  const parsed: ConnectionUrl = {
    kind,
    host,
    port: url.port ? Number(url.port) : DEFAULT_PORTS[kind],
    user: decode(url.username),
    password: decode(url.password),
    database: decode(url.pathname.replace(/^\//, '')),
  }
  if (kind === 'postgres') parsed.ssl_mode = SSL_FROM_LIBPQ[url.searchParams.get('sslmode') ?? ''] ?? 'disable'
  return parsed
}

type Translate = (key: string, options?: Record<string, unknown>) => string

/** The retention rule in one sentence. */
export function retentionSummary(t: Translate, { max_age_days, keep_weekly, keep_monthly }: Retention): string {
  if (max_age_days <= 0) return t('plans.retention.forever')
  const keeps = [
    keep_weekly > 0 ? t('plans.retention.weekly', { count: keep_weekly }) : null,
    keep_monthly > 0 ? t('plans.retention.monthly', { count: keep_monthly }) : null,
  ].filter((part): part is string => part != null)
  const deleted = t('plans.retention.olderThan', { count: max_age_days })
  if (keeps.length === 0) return t('plans.retention.sentence', { rule: deleted })
  return t('plans.retention.sentenceExcept', { rule: deleted, keeps: keeps.join(t('plans.retention.and')) })
}
