import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { ApiError } from '@/api/errors'
import { childPrefix, normalizePrefix, prefixCrumbs } from './browsePath'
import { plansUsingStorage } from './inUse'
import { emptyStorageForm, formToRequest, hasStoredSecret, storageLocation, storageToForm } from './storageForm'
import type { Storage } from './types'

const base = { id: 1, name: 'Offsite', secret_set: true, created_at: '', updated_at: '' }

describe('storage form', () => {
  it('sends only the block of the chosen kind', () => {
    const values = emptyStorageForm()
    values.name = '  Nightly  '
    values.local.path = ' /var/backups '
    values.s3.bucket = 'typed before switching'
    assert.deepEqual(formToRequest(values), { name: 'Nightly', kind: 'local', local: { path: '/var/backups' } })

    values.kind = 's3'
    const request = formToRequest(values)
    assert.equal(request.local, undefined)
    assert.equal(request.s3?.bucket, 'typed before switching')
  })

  it('fills the form from a storage, with blank credentials', () => {
    const storage: Storage = { ...base, kind: 's3', s3: { endpoint: 'https://minio:9000', region: 'eu', bucket: 'b', prefix: 'p', use_path_style: true } }
    const values = storageToForm(storage)
    assert.equal(values.kind, 's3')
    assert.equal(values.s3.bucket, 'b')
    assert.equal(values.s3.secret_access_key, '')
    assert.equal(values.local.path, '')
  })

  it('keeps stored credentials only for the same kind', () => {
    const storage: Storage = { ...base, kind: 's3', s3: { endpoint: '', region: '', bucket: 'b', prefix: '', use_path_style: false } }
    assert.equal(hasStoredSecret(storage, 's3'), true)
    assert.equal(hasStoredSecret(storage, 'gcs'), false)
    assert.equal(hasStoredSecret(undefined, 's3'), false)
    assert.equal(hasStoredSecret({ ...storage, secret_set: false }, 's3'), false)
  })

  it('summarises the location', () => {
    assert.equal(storageLocation({ ...base, kind: 'local', local: { path: '/var/backups' } }), '/var/backups')
    assert.equal(storageLocation({ ...base, kind: 's3', s3: { endpoint: '', region: '', bucket: 'b', prefix: 'db', use_path_style: false } }), 's3://b/db')
    assert.equal(
      storageLocation({ ...base, kind: 's3', s3: { endpoint: 'https://s3.example.com/', region: '', bucket: 'b', prefix: '', use_path_style: false } }),
      's3://b (s3.example.com)'
    )
    assert.equal(storageLocation({ ...base, kind: 'gcs', gcs: { bucket: 'g', prefix: '' } }), 'gs://g')
  })
})

describe('browse path', () => {
  it('splits a prefix into crumbs', () => {
    assert.deepEqual(prefixCrumbs(''), [])
    assert.deepEqual(prefixCrumbs('a/b/'), [
      { name: 'a', prefix: 'a/' },
      { name: 'b', prefix: 'a/b/' },
    ])
  })

  it('descends and normalises', () => {
    assert.equal(childPrefix('', 'a'), 'a/')
    assert.equal(childPrefix('a', 'b'), 'a/b/')
    assert.equal(normalizePrefix('/a//b'), 'a/b/')
    assert.equal(normalizePrefix(null), '')
  })
})

describe('a storage in use', () => {
  it('reads the plans from a 409', () => {
    const error = new ApiError(409, 'storages.in_use', 'in use', undefined, { plans: [{ id: 2, name: 'Blog' }, { id: 1, name: 'Shop' }, { bad: true }] })
    assert.deepEqual(plansUsingStorage(error), [
      { id: 2, name: 'Blog' },
      { id: 1, name: 'Shop' },
    ])
  })

  it('ignores other errors', () => {
    assert.equal(plansUsingStorage(new ApiError(404, 'storages.not_found', 'gone')), null)
    assert.equal(plansUsingStorage(new Error('network')), null)
  })
})
