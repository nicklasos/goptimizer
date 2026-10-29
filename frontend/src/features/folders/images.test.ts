import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { imageSizes, readImageFilters, writeImageFilters } from './images'

const read = (query: string) => readImageFilters(new URLSearchParams(query))

describe('readImageFilters', () => {
  it('reads the status, search and sort', () => {
    assert.deepEqual(read('status=optimized&q=banners&sort=saved'), { status: 'optimized', q: 'banners', sort: 'saved' })
  })

  it('lists every status for a missing or unknown one', () => {
    assert.equal(read('').status, null)
    assert.equal(read('status=deleted').status, null)
    assert.equal(read('status=toString').status, null)
  })

  it('sorts by latest change unless the list is of optimized images', () => {
    assert.equal(read('status=failed&sort=saved').sort, 'updated')
    assert.equal(read('sort=saved').sort, 'updated')
    assert.equal(read('status=optimized&sort=bogus').sort, 'updated')
  })
})

describe('writeImageFilters', () => {
  it('changes only the filters given and keeps the other parameters', () => {
    const next = writeImageFilters(new URLSearchParams('page=2&status=failed&q=old'), { q: 'new' })
    assert.deepEqual(Object.fromEntries(next), { page: '2', status: 'failed', q: 'new' })
  })

  it('leaves the defaults out', () => {
    const next = writeImageFilters(new URLSearchParams('page=2&status=failed&q=old'), { status: null, q: '' })
    assert.equal(next.toString(), 'page=2')
  })

  it('drops most saved when the status changes away from optimized', () => {
    const next = writeImageFilters(new URLSearchParams('status=optimized&sort=saved'), { status: 'kept' })
    assert.equal(next.toString(), 'status=kept')
  })

  it('reads back what it wrote', () => {
    const filters = { status: 'optimized', q: 'shop/banners', sort: 'saved' } as const
    assert.deepEqual(readImageFilters(writeImageFilters(new URLSearchParams(), filters)), filters)
  })
})

describe('imageSizes', () => {
  it('shows an optimized image before and after, with the saving', () => {
    assert.deepEqual(imageSizes({ status: 'optimized', size: 600, original_size: 1000 }), { before: 1000, after: 600, savedPercent: 40 })
  })

  it('shows any other image as its size on disk', () => {
    assert.deepEqual(imageSizes({ status: 'kept', size: 1000, original_size: null }), { before: 1000, after: null, savedPercent: null })
    assert.deepEqual(imageSizes({ status: 'pending', size: 1000 }), { before: 1000, after: null, savedPercent: null })
  })

  it('shows an optimized image whose original size was lost as its size', () => {
    assert.deepEqual(imageSizes({ status: 'optimized', size: 600, original_size: null }), { before: 600, after: null, savedPercent: null })
  })
})
