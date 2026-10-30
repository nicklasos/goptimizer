import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { formatBytes } from '@/shared/utils/format'
import { byteTicks, savedPercent, totalSavings } from './savings'
import type { DaySavings } from './types'

const KB = 1024
const MB = 1024 * KB

describe('byteTicks', () => {
  it('steps in round numbers of the unit the largest value is shown in', () => {
    assert.deepEqual(byteTicks(3.4 * MB).map(formatBytes), ['0 B', '1 MB', '2 MB', '3 MB', '4 MB'])
    assert.deepEqual(byteTicks(700 * KB).map(formatBytes), ['0 B', '200 KB', '400 KB', '600 KB', '800 KB'])
  })

  it('ends at or above the largest value', () => {
    for (const max of [1, 3, 999, 1024, 1500 * KB, 7.9 * MB, 1023 * MB, 2.2 * 1024 * MB]) {
      const ticks = byteTicks(max)
      assert.ok(ticks.at(-1)! >= max, `${max} is under the top tick ${ticks.at(-1)}`)
      assert.ok(ticks.length >= 2 && ticks.length <= 5, `${max} has ${ticks.length} ticks`)
    }
  })

  it('never labels two gridlines alike for a few bytes', () => {
    assert.deepEqual(byteTicks(1), [0, 1])
    assert.deepEqual(byteTicks(3), [0, 1, 2, 3])
  })

  it('is only the baseline when nothing was saved', () => {
    assert.deepEqual(byteTicks(0), [0])
  })
})

describe('totalSavings', () => {
  it('adds up the days', () => {
    const days = [
      { day: '2026-10-28', optimized: 2, bytes_saved: 300, bytes_before: 1000 },
      { day: '2026-10-29', optimized: 0, bytes_saved: 0, bytes_before: 0 },
      { day: '2026-10-30', optimized: 1, bytes_saved: 200, bytes_before: 1000 },
    ] satisfies DaySavings[]
    assert.deepEqual(totalSavings(days), { saved: 500, before: 2000 })
  })
})

describe('savedPercent', () => {
  it('is the share of the original size saved', () => {
    assert.equal(savedPercent({ saved: 500, before: 2000 }), 25)
  })

  it('is null before anything was optimized', () => {
    assert.equal(savedPercent({ saved: 0, before: 0 }), null)
  })
})
