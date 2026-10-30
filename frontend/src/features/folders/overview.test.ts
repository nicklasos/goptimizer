import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { folderTotals, pollInterval, POLL_WAITING, POLL_WORKING } from './overview'
import type { Folder, Scan } from './types'

const folder = (overrides: Partial<Folder> = {}): Folder =>
  ({ enabled: true, missing_tools: [], image_counts: {}, bytes_saved: 0, bytes_before: 0, last_scan: null, ...overrides }) as Folder

const scan = (status: Scan['status']) => ({ status }) as Scan

describe('folderTotals', () => {
  it('adds up every folder', () => {
    const totals = folderTotals([
      folder({ bytes_saved: 1000, bytes_before: 4000, image_counts: { optimized: 3, pending: 2, failed: 1 }, last_scan: scan('running') }),
      folder({ bytes_saved: 500, bytes_before: 1000, image_counts: { optimized: 1, optimizing: 2 }, last_scan: scan('succeeded') }),
      folder({ last_scan: scan('queued') }),
    ])
    assert.deepEqual(totals, { bytesSaved: 1500, bytesBefore: 5000, optimized: 4, pending: 2, failed: 1, optimizing: 2, scans: 2 })
  })

  it('is all zero without folders', () => {
    assert.deepEqual(folderTotals([]), { bytesSaved: 0, bytesBefore: 0, optimized: 0, pending: 0, failed: 0, optimizing: 0, scans: 0 })
  })
})

describe('pollInterval', () => {
  it('polls fast while a scan is queued or running', () => {
    assert.equal(pollInterval([folder(), folder({ last_scan: scan('queued') })]), POLL_WORKING)
    assert.equal(pollInterval([folder({ last_scan: scan('running') })]), POLL_WORKING)
  })

  it('polls fast while an image is being optimized, even in a paused folder', () => {
    assert.equal(pollInterval([folder({ enabled: false, image_counts: { optimizing: 1 } })]), POLL_WORKING)
  })

  it('polls slowly while an active folder with every tool has images waiting', () => {
    assert.equal(pollInterval([folder({ image_counts: { pending: 5 } })]), POLL_WAITING)
  })

  it('stops when the waiting images cannot be taken', () => {
    assert.equal(pollInterval([folder({ enabled: false, image_counts: { pending: 5 } })]), false)
    assert.equal(pollInterval([folder({ missing_tools: ['pngquant'], image_counts: { pending: 5 } })]), false)
  })

  it('stops once nothing is in flight or waiting', () => {
    assert.equal(pollInterval([folder({ image_counts: { optimized: 10, failed: 2 }, last_scan: scan('failed') })]), false)
    assert.equal(pollInterval([]), false)
  })
})
