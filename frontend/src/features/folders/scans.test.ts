import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { imagesIn, indexedImages, isScanActive, scanDuration } from './scans'
import type { Scan } from './types'

const scan = (status: Scan['status']) => ({ status }) as Scan

describe('isScanActive', () => {
  it('is true while a scan is queued or running', () => {
    assert.equal(isScanActive(scan('queued')), true)
    assert.equal(isScanActive(scan('running')), true)
  })

  it('is false for a finished scan or none', () => {
    assert.equal(isScanActive(scan('succeeded')), false)
    assert.equal(isScanActive(scan('failed')), false)
    assert.equal(isScanActive(null), false)
  })
})

describe('indexedImages', () => {
  it('counts every status but gone', () => {
    assert.equal(indexedImages({ image_counts: { pending: 3, optimized: 2, kept: 1, gone: 7 } }), 6)
  })

  it('is 0 before the first scan', () => {
    assert.equal(indexedImages({ image_counts: {} }), 0)
  })
})

describe('imagesIn', () => {
  it('is the count of one status', () => {
    assert.equal(imagesIn({ image_counts: { optimized: 312, failed: 4 } }, 'optimized'), 312)
  })

  it('is 0 for a status the folder has none of', () => {
    assert.equal(imagesIn({ image_counts: { pending: 3 } }, 'failed'), 0)
  })
})

describe('scanDuration', () => {
  const now = new Date('2026-10-29T12:00:00Z')

  it('is what the API measured for a finished scan, below the second its timestamps show', () => {
    assert.equal(scanDuration({ started_at: '2026-10-29T11:00:00Z', duration_ms: 18 }, now), 18)
  })

  it('is the time so far for a running scan', () => {
    assert.equal(scanDuration({ started_at: '2026-10-29T11:59:00Z', duration_ms: null }, now), 60_000)
  })

  it('is null for a scan still queued', () => {
    assert.equal(scanDuration({ started_at: null, duration_ms: null }, now), null)
  })
})
