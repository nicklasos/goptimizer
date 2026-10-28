import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { emptyPlanForm, formToRequest, hasStoredSecret, parseConnectionUrl, planFormFields, planToForm, retentionSummary, sourceFieldPaths } from './planForm'
import type { Plan } from './types'

const base: Plan = {
  id: 7,
  name: 'Shop',
  source_kind: 'postgres',
  postgres: { host: 'db', port: 5433, user: 'shop', database: 'shop', ssl_mode: 'require' },
  secret_set: true,
  schedules: [
    { time: '03:00', days: [] },
    { time: '18:00', days: ['mon', 'fri'] },
  ],
  max_age_days: 30,
  keep_weekly: 0,
  keep_monthly: 12,
  storages: [{ id: 3, name: 'Nightly', kind: 'local' }],
  enabled: true,
  next_run_at: null,
  health: 'never',
  folder: 'shop/',
  created_at: '',
  updated_at: '',
}

describe('plan form', () => {
  it('sends only the source block of the chosen kind', () => {
    const values = emptyPlanForm()
    values.name = '  Shop  '
    values.source_kind = 'sqlite'
    values.sqlite.path = ' /srv/app.db '
    values.postgres.host = 'typed before switching'
    values.storage_ids = [3]
    const request = formToRequest(values)
    assert.equal(request.name, 'Shop')
    assert.deepEqual(request.sqlite, { path: '/srv/app.db' })
    assert.equal(request.postgres, undefined)
    assert.deepEqual(request.storage_ids, [3])

    values.source_kind = 'postgres'
    assert.equal(formToRequest(values).postgres?.host, 'typed before switching')
    assert.equal(formToRequest(values).sqlite, undefined)
  })

  it('sends retention rules that are switched off as 0', () => {
    const values = emptyPlanForm()
    values.keep_weekly_on = false
    assert.deepEqual(pick(formToRequest(values)), { max_age_days: 30, keep_weekly: 0, keep_monthly: 12 })
    values.prune = false
    assert.deepEqual(pick(formToRequest(values)), { max_age_days: 0, keep_weekly: 0, keep_monthly: 0 })
  })

  it('stores days in week order and drops blank excludes', () => {
    const values = emptyPlanForm()
    values.source_kind = 'files'
    values.files = { paths: [' /srv/uploads '], excludes: ['*.tmp', '  '] }
    values.schedules = [{ time: '18:00', days: ['fri', 'mon', 'tue', 'wed', 'thu', 'sat', 'sun'] }]
    const request = formToRequest(values)
    assert.deepEqual(request.files, { paths: ['/srv/uploads'], excludes: ['*.tmp'] })
    assert.deepEqual(request.schedules, [{ time: '18:00', days: [] }])
  })

  it('fills the form from a plan, with a blank password', () => {
    const values = planToForm(base)
    assert.equal(values.postgres.port, 5433)
    assert.equal(values.postgres.password, '')
    assert.equal(values.keep_weekly_on, false)
    assert.equal(values.keep_weekly, 8, 'a rule that is off keeps the default for when it is switched on')
    assert.equal(values.keep_monthly, 12)
    assert.deepEqual(values.storage_ids, [3])
    assert.deepEqual(formToRequest(values).schedules, base.schedules)
  })

  it('keeps a stored password only for the same kind', () => {
    assert.equal(hasStoredSecret(base, 'postgres'), true)
    assert.equal(hasStoredSecret(base, 'mysql'), false)
    assert.equal(hasStoredSecret({ ...base, secret_set: false }, 'postgres'), false)
    assert.equal(hasStoredSecret(undefined, 'postgres'), false)
  })

  it('names the fields of each row', () => {
    const values = emptyPlanForm()
    values.files = { paths: ['/a', '/b'], excludes: ['*.log'] }
    const fields = planFormFields(values)
    assert.ok(fields.includes('files.paths.1'))
    assert.ok(fields.includes('files.excludes.0'))
    assert.ok(fields.includes('schedules.0.time'))
    assert.deepEqual(sourceFieldPaths({ ...values, source_kind: 'files' }), [
      ['files', 'paths', 0],
      ['files', 'paths', 1],
      ['files', 'excludes', 0],
    ])
  })
})

describe('parseConnectionUrl', () => {
  it('reads a Postgres URL', () => {
    assert.deepEqual(parseConnectionUrl(' postgres://shop:s3cr%40t@db.internal:5433/shop_prod?sslmode=verify-full '), {
      kind: 'postgres',
      host: 'db.internal',
      port: 5433,
      user: 'shop',
      password: 's3cr@t',
      database: 'shop_prod',
      ssl_mode: 'verify-full',
    })
  })

  it('fills in default ports and maps libpq ssl modes', () => {
    const parsed = parseConnectionUrl('postgresql://app@localhost/app?sslmode=prefer')
    assert.equal(parsed?.port, 5432)
    assert.equal(parsed?.ssl_mode, 'disable')
    assert.equal(parsed?.password, '')
    assert.equal(parseConnectionUrl('postgres://a@h/d?sslmode=verify-ca')?.ssl_mode, 'require')
  })

  it('reads a MySQL URL', () => {
    assert.deepEqual(parseConnectionUrl('mysql://root:pw@[::1]/blog'), {
      kind: 'mysql',
      host: '::1',
      port: 3306,
      user: 'root',
      password: 'pw',
      database: 'blog',
    })
  })

  it('refuses anything else', () => {
    assert.equal(parseConnectionUrl('not a url'), null)
    assert.equal(parseConnectionUrl('https://example.com/db'), null)
    assert.equal(parseConnectionUrl('postgres:///db'), null)
  })
})

describe('retentionSummary', () => {
  const t = (key: string, options?: Record<string, unknown>) => {
    const count = options?.count
    const texts: Record<string, string> = {
      'plans.retention.forever': 'Kept forever.',
      'plans.retention.olderThan': `older than ${count}d`,
      'plans.retention.weekly': `${count} weekly`,
      'plans.retention.monthly': `${count} monthly`,
      'plans.retention.and': ' and ',
      'plans.retention.sentence': `Delete ${options?.rule}.`,
      'plans.retention.sentenceExcept': `Delete ${options?.rule}, except ${options?.keeps}.`,
    }
    return texts[key] ?? key
  }

  it('says what is kept', () => {
    assert.equal(retentionSummary(t, { max_age_days: 0, keep_weekly: 0, keep_monthly: 0 }), 'Kept forever.')
    assert.equal(retentionSummary(t, { max_age_days: 30, keep_weekly: 0, keep_monthly: 0 }), 'Delete older than 30d.')
    assert.equal(retentionSummary(t, { max_age_days: 30, keep_weekly: 8, keep_monthly: 12 }), 'Delete older than 30d, except 8 weekly and 12 monthly.')
    assert.equal(retentionSummary(t, { max_age_days: 7, keep_weekly: 0, keep_monthly: 6 }), 'Delete older than 7d, except 6 monthly.')
  })
})

function pick({ max_age_days, keep_weekly, keep_monthly }: { max_age_days: number; keep_weekly: number; keep_monthly: number }) {
  return { max_age_days, keep_weekly, keep_monthly }
}
