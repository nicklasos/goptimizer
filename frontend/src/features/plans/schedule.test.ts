import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { duplicateSchedules, nextRun, normalizeDays, parseTimeOfDay, scheduleSummary } from './schedule'
import type { Schedule } from './types'

const here = path.dirname(fileURLToPath(import.meta.url))
// The API's NextRun table: both sides run the same cases.
const fixture = path.resolve(here, '../../../../backend/tests/unit/testdata/next_run.json')
const cases = JSON.parse(fs.readFileSync(fixture, 'utf8')) as { name: string; schedules: Schedule[]; after: string; next: string | null }[]

describe('nextRun', () => {
  it('has the shared cases', () => assert.ok(cases.length > 5))

  for (const tc of cases) {
    it(tc.name, () => {
      const next = nextRun(tc.schedules, new Date(tc.after))
      assert.equal(next?.toISOString() ?? null, tc.next == null ? null : new Date(tc.next).toISOString())
    })
  }
})

describe('schedule rows', () => {
  it('reads a time of day', () => {
    assert.equal(parseTimeOfDay('00:00'), 0)
    assert.equal(parseTimeOfDay('18:30'), 1110)
    assert.equal(parseTimeOfDay('24:00'), null)
    assert.equal(parseTimeOfDay('3:00'), null)
  })

  it('stores days in week order, and all seven as every day', () => {
    assert.deepEqual(normalizeDays(['fri', 'mon']), ['mon', 'fri'])
    assert.deepEqual(normalizeDays(['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun']), [])
  })

  it('finds repeated rows', () => {
    const rows: Schedule[] = [
      { time: '03:00', days: [] },
      { time: '03:00', days: ['mon'] },
      { time: '03:00', days: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] },
      { time: '03:00', days: ['mon'] },
    ]
    assert.deepEqual(duplicateSchedules(rows), [2, 3])
  })
})

describe('scheduleSummary', () => {
  const t = (key: string) =>
    ({ 'plans.schedule.daily': 'Daily', 'plans.schedule.manualOnly': 'Manual runs only' })[key] ?? key.split('.').pop()!.replace(/^./, (c) => c.toUpperCase())

  it('says when a plan only runs by hand', () => {
    assert.equal(scheduleSummary(t, []), 'Manual runs only')
  })

  it('joins the times of rows with the same days', () => {
    assert.equal(
      scheduleSummary(t, [
        { time: '18:00', days: [] },
        { time: '03:00', days: [] },
      ]),
      'Daily 03:00, 18:00 UTC'
    )
  })

  it('names day ranges and lists', () => {
    assert.equal(
      scheduleSummary(t, [
        { time: '03:00', days: [] },
        { time: '18:00', days: ['mon', 'tue', 'wed', 'thu', 'fri'] },
        { time: '12:00', days: ['sat', 'mon'] },
      ]),
      'Daily 03:00 · Mon–Fri 18:00 · Mon, Sat 12:00 UTC'
    )
  })
})
