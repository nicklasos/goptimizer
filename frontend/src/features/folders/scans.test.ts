import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { indexedImages, isScanActive } from './scans'
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
