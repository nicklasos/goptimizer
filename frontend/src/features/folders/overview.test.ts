import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { folderTotals, pollInterval, POLL_WAITING, POLL_WORKING, saving } from './overview'
import type { Folder, Scan } from './types'

const folder = (overrides: Partial<Folder> = {}): Folder =>
  ({ enabled: true, missing_tools: [], image_counts: {}, bytes_before: 0, bytes_now: 0, last_scan: null, ...overrides }) as Folder

const scan = (status: Scan['status']) => ({ status }) as Scan

describe('folderTotals', () => {
  it('adds up every folder', () => {
    const totals = folderTotals([
      folder({ bytes_before: 4000, bytes_now: 3000, image_counts: { optimized: 3, pending: 2, failed: 1 }, last_scan: scan('running') }),
      folder({ bytes_before: 1000, bytes_now: 500, image_counts: { optimized: 1, optimizing: 2 }, last_scan: scan('succeeded') }),
      folder({ last_scan: scan('queued') }),
    ])
    assert.deepEqual(totals, { bytesBefore: 5000, bytesNow: 3500, optimized: 4, pending: 2, failed: 1, optimizing: 2, scans: 2 })
  })

  it('is all zero without folders', () => {
    assert.deepEqual(folderTotals([]), { bytesBefore: 0, bytesNow: 0, optimized: 0, pending: 0, failed: 0, optimizing: 0, scans: 0 })
  })
})

describe('saving', () => {
  it('is the bytes saved and their share of the size before', () => {
    assert.deepEqual(saving({ before: 214, now: 148 }), { bytes: 66, percent: (66 / 214) * 100 })
  })

  it('has no share without images', () => {
    assert.deepEqual(saving({ before: 0, now: 0 }), { bytes: 0, percent: null })
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
