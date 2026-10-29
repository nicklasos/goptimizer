import type {
  InternalFoldersFolderRequest,
  InternalFoldersFolderResponse,
  InternalFoldersProfile,
  InternalScansKind,
  ScanResponse,
} from '@/api/schema'

export type Folder = InternalFoldersFolderResponse
export type FolderRequest = InternalFoldersFolderRequest
export type Profile = InternalFoldersProfile
export type Mode = Profile['mode']
export type Scan = ScanResponse
export type ScanKind = InternalScansKind

export const MODES: readonly Mode[] = ['lossy', 'lossless']
export const SCAN_KINDS: readonly ScanKind[] = ['quick', 'full']
