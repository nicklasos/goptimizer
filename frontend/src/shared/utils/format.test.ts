import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import {
  EMPTY,
  formatBytes,
  formatDateTime,
  formatDay,
  formatDuration,
  formatNextRun,
  formatNumber,
  formatRelative,
  formatUtcDateTime,
  setFormatLanguage,
} from './format'

const plain = (s: string) => s.replace(/[\u00a0\u202f]/g, ' ')

describe('format', () => {
  beforeEach(() => setFormatLanguage('en'))

  it('uses the app language, not the browser', () => {
    assert.equal(plain(formatNumber(1234.5)), '1,234.5')
    setFormatLanguage('uk')
    assert.equal(plain(formatNumber(1234.5)), '1 234,5')
  })

  it('shows a dash for missing values', () => {
    assert.equal(formatNumber(null), EMPTY)
    assert.equal(formatNumber(Number.NaN), EMPTY)
    assert.equal(formatDateTime(null), EMPTY)
    assert.equal(formatBytes(null), EMPTY)
  })

  it('formats a local timestamp', () => {
    assert.equal(formatDateTime('2026-09-11T14:05:00'), '11.09.2026 14:05')
    assert.equal(formatDateTime('not a date'), 'not a date')
  })

  it('formats a moment in UTC, whatever the offset it was written with', () => {
    assert.equal(formatUtcDateTime('2026-10-19T21:00:00+03:00'), 'Mon 19.10.2026 18:00 UTC')
    assert.equal(formatUtcDateTime(new Date(Date.UTC(2027, 0, 1, 0, 0))), 'Fri 01.01.2027 00:00 UTC')
    assert.equal(formatUtcDateTime(null), EMPTY)
  })

  it('formats byte counts in binary units', () => {
    assert.equal(formatBytes(0), '0 B')
    assert.equal(formatBytes(512), '512 B')
    assert.equal(formatBytes(1536), '1.5 KB')
    assert.equal(formatBytes(5 * 1024 * 1024), '5 MB')
  })

  const now = new Date('2026-10-17T15:00:00Z')

  it('says how long ago in the largest unit that fits', () => {
    assert.equal(formatRelative('2026-10-17T12:00:00Z', now), '3 hours ago')
    assert.equal(formatRelative('2026-10-17T14:59:40Z', now), 'now')
    assert.equal(formatRelative('2026-10-17T14:20:00Z', now), '40 minutes ago')
    assert.equal(formatRelative('2026-10-16T10:00:00Z', now), 'yesterday')
    assert.equal(formatRelative('2026-10-10T15:00:00Z', now), '7 days ago')
    assert.equal(formatRelative('2026-10-17T15:05:00Z', now), 'in 5 minutes')
    assert.equal(formatRelative(null, now), EMPTY)
    setFormatLanguage('uk')
    assert.equal(formatRelative('2026-10-17T12:00:00Z', now), '3 години тому')
  })

  it('names the next run by its UTC day', () => {
    assert.equal(formatNextRun('2026-10-17T18:00:00Z', now), 'today 18:00 UTC')
    assert.equal(formatNextRun('2026-10-18T03:00:00Z', now), 'tomorrow 03:00 UTC')
    assert.equal(formatNextRun('2026-10-19T03:00:00Z', now), 'Mon 19.10 03:00 UTC')
    assert.equal(formatNextRun('2026-10-18T00:30:00Z', new Date('2026-10-17T23:50:00Z')), 'tomorrow 00:30 UTC', 'the UTC date decides, not the hours between')
    setFormatLanguage('uk')
    assert.equal(formatNextRun('2026-10-17T18:00:00Z', now), 'сьогодні 18:00 UTC')
  })

  it('shows a UTC day as day and month', () => {
    assert.equal(plain(formatDay('2026-10-14')), '14 Oct')
    assert.equal(plain(formatDay('2026-01-01')), '1 Jan', 'the UTC day, whatever the local zone')
  })

  it('shows a duration in its two largest units', () => {
    assert.equal(plain(formatDuration(850)), '850 ms')
    assert.equal(plain(formatDuration(12_400)), '12 secs')
    assert.equal(plain(formatDuration(185_000)), '3 mins, 5 secs')
    assert.equal(plain(formatDuration(3_720_000)), '1 hr, 2 mins')
    assert.equal(formatDuration(null), EMPTY)
  })
})
