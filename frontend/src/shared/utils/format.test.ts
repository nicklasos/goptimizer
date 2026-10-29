import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import {
  EMPTY,
  formatBytes,
  formatCount,
  formatDateTime,
  formatDay,
  formatDuration,
  formatNumber,
  formatRelative,
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

  it('writes a headline count in full up to 9,999, then compact', () => {
    assert.equal(plain(formatCount(1284)), '1,284')
    assert.equal(plain(formatCount(12_940)), '12.9k')
    assert.equal(plain(formatCount(1_250_000)), '1.3m')
    assert.equal(formatCount(null), EMPTY)
  })

  it('formats a local timestamp', () => {
    assert.equal(formatDateTime('2026-09-11T14:05:00'), '11.09.2026 14:05')
    assert.equal(formatDateTime('not a date'), 'not a date')
  })

  it('formats a UTC calendar day whatever the local time zone', () => {
    assert.equal(formatDay('2026-10-29'), '29 Oct')
    assert.equal(formatDay('2026-10-29', 'long'), 'Thu, 29 Oct 2026')
    assert.equal(formatDay('not a day'), 'not a day')
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

  it('shows a duration in its two largest units', () => {
    assert.equal(plain(formatDuration(850)), '850 ms')
    assert.equal(plain(formatDuration(12_400)), '12 secs')
    assert.equal(plain(formatDuration(185_000)), '3 mins, 5 secs')
    assert.equal(plain(formatDuration(3_720_000)), '1 hr, 2 mins')
    assert.equal(formatDuration(null), EMPTY)
  })
})
