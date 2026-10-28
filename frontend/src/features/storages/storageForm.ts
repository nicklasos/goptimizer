import type { Storage, StorageKind, StorageRequest } from './types'

/** The form keeps a block per kind, so switching kinds back and forth loses nothing typed. */
export interface StorageFormValues {
  name: string
  kind: StorageKind
  local: { path: string }
  s3: {
    endpoint: string
    region: string
    bucket: string
    prefix: string
    use_path_style: boolean
    access_key_id: string
    secret_access_key: string
  }
  gcs: { bucket: string; prefix: string; credentials_json: string }
}

/** Every field the API can report an error on, as `block.field` paths. */
export const STORAGE_FORM_FIELDS = [
  'name',
  'kind',
  'local',
  'local.path',
  's3',
  's3.endpoint',
  's3.region',
  's3.bucket',
  's3.prefix',
  's3.access_key_id',
  's3.secret_access_key',
  'gcs',
  'gcs.bucket',
  'gcs.prefix',
  'gcs.credentials_json',
] as const

export function emptyStorageForm(): StorageFormValues {
  return {
    name: '',
    kind: 'local',
    local: { path: '' },
    s3: { endpoint: '', region: '', bucket: '', prefix: '', use_path_style: false, access_key_id: '', secret_access_key: '' },
    gcs: { bucket: '', prefix: '', credentials_json: '' },
  }
}

/** The stored settings of a storage. Credentials are never sent back, so their fields start blank. */
export function storageToForm(storage: Storage): StorageFormValues {
  const values = emptyStorageForm()
  values.name = storage.name
  values.kind = storage.kind
  if (storage.local) values.local = { ...storage.local }
  if (storage.s3) values.s3 = { ...values.s3, ...storage.s3 }
  if (storage.gcs) values.gcs = { ...values.gcs, ...storage.gcs }
  return values
}

/** The request sends only the block of the chosen kind, as the API requires. */
export function formToRequest(values: StorageFormValues): StorageRequest {
  const request: StorageRequest = { name: values.name.trim(), kind: values.kind }
  if (values.kind === 'local') request.local = { path: values.local.path.trim() }
  if (values.kind === 's3') request.s3 = { ...values.s3 }
  if (values.kind === 'gcs') request.gcs = { ...values.gcs }
  return request
}

/** Credentials are stored for this kind, so blank secret fields keep them. */
export function hasStoredSecret(storage: Storage | undefined, kind: StorageKind): boolean {
  return storage != null && storage.kind === kind && storage.secret_set
}

/** Where the storage keeps its files, in one line: a path, s3://bucket/prefix, gs://bucket/prefix. */
export function storageLocation(storage: Storage): string {
  if (storage.local) return storage.local.path
  if (storage.s3) {
    const where = joinLocation(`s3://${storage.s3.bucket}`, storage.s3.prefix)
    const host = storage.s3.endpoint.replace(/^https?:\/\//, '').replace(/\/$/, '')
    return host ? `${where} (${host})` : where
  }
  if (storage.gcs) return joinLocation(`gs://${storage.gcs.bucket}`, storage.gcs.prefix)
  return ''
}

const joinLocation = (bucket: string, prefix: string) => (prefix ? `${bucket}/${prefix}` : bucket)
