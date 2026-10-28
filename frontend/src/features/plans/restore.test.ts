import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanPath, formTarget, ORIGINAL_LOCATIONS, ownTarget, restoreFormValues, restoreRequest, targetSummary } from './restore'
import type { Plan, SourceKind } from './types'

const here = path.dirname(fileURLToPath(import.meta.url))
// The API's RestoreTargetIsOwn table: both sides run the same cases.
const fixture = path.resolve(here, '../../../../backend/tests/unit/testdata/own_target.json')
const cases = JSON.parse(fs.readFileSync(fixture, 'utf8')) as { name: string; kind: SourceKind; source: object; target: object; own: boolean; phrase: string }[]

describe('ownTarget', () => {
  it('has the shared cases', () => assert.ok(cases.length > 10))

  for (const tc of cases) {
    it(tc.name, () => {
      assert.deepEqual(ownTarget(tc.kind, tc.source, tc.target), { own: tc.own, phrase: tc.phrase })
    })
  }
})

const pgPlan = {
  source_kind: 'postgres',
  postgres: { host: 'db.internal', port: 5432, user: 'app', database: 'shop', ssl_mode: 'disable' },
} as Plan

describe('restore form', () => {
  it('cleans a path as the API does', () => {
    assert.equal(cleanPath('/srv//app/./data/../app.db'), '/srv/app/app.db')
    assert.equal(cleanPath('/../x'), '/x')
  })

  it('starts from the proposed target with no password', () => {
    const values = restoreFormValues({ source_kind: 'postgres', postgres: { ...pgPlan.postgres!, create_database: false } })
    assert.deepEqual(values, { postgres: { host: 'db.internal', port: 5432, user: 'app', database: 'shop', ssl_mode: 'disable' } })
    assert.deepEqual(restoreFormValues({ source_kind: 'files', files: { directory: '/work/restores/x', original_locations: false } }), {
      files: { directory: '/work/restores/x', original_locations: false },
    })
  })

  it('fills blank connection fields from the plan before deciding whether it is the plan’s own database', () => {
    const target = formTarget(pgPlan, { postgres: { host: ' ', port: null, user: '', database: ' shop ', ssl_mode: 'disable' } })
    assert.deepEqual(target, { host: 'db.internal', port: 5432, database: 'shop' })
    assert.equal(ownTarget('postgres', pgPlan.postgres!, target).own, true)
  })

  it('builds the request: the plan’s password when blank, confirm only when given', () => {
    const values = { postgres: { host: 'db.internal', port: 5432, user: 'app', database: 'shop_copy ', ssl_mode: 'disable' as const, password: '' } }
    assert.deepEqual(restoreRequest('postgres', values, undefined, undefined), {
      postgres: { host: 'db.internal', port: 5432, user: 'app', database: 'shop_copy', ssl_mode: 'disable' },
    })
    assert.deepEqual(restoreRequest('postgres', { postgres: { ...values.postgres, password: 'pw' } }, false, 'shop'), {
      confirm: 'shop',
      postgres: { host: 'db.internal', port: 5432, user: 'app', database: 'shop_copy', ssl_mode: 'disable', password: 'pw', create_database: false },
    })
    assert.deepEqual(restoreRequest('files', { files: { directory: '/ignored', original_locations: true } }, undefined, ORIGINAL_LOCATIONS), {
      confirm: ORIGINAL_LOCATIONS,
      files: { original_locations: true },
    })
    assert.deepEqual(restoreRequest('sqlite', { sqlite: { path: ' /tmp/copy.db ' } }, undefined, undefined), { sqlite: { path: '/tmp/copy.db' } })
  })

  it('sums up a target in one line', () => {
    assert.equal(targetSummary({ source_kind: 'postgres', postgres: { ...pgPlan.postgres!, database: 'shop_copy', create_database: true } }), 'app@db.internal:5432/shop_copy')
    assert.equal(targetSummary({ source_kind: 'sqlite', sqlite: { path: '/tmp/a.db' } }), '/tmp/a.db')
    assert.equal(targetSummary({ source_kind: 'files', files: { directory: '', original_locations: true } }), null)
  })
})
