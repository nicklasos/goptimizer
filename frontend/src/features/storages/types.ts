import type {
  InternalStoragesKind,
  InternalStoragesObjectResponse,
  InternalStoragesStorageRequest,
  InternalStoragesStorageResponse,
  InternalStoragesTestResultResponse,
} from '@/api/schema'

export type Storage = InternalStoragesStorageResponse
export type StorageRequest = InternalStoragesStorageRequest
export type StorageKind = InternalStoragesKind
export type StorageTestResult = InternalStoragesTestResultResponse
export type StorageObject = InternalStoragesObjectResponse

export const STORAGE_KINDS: readonly StorageKind[] = ['local', 's3', 'gcs']
