import type { InternalFoldersFolderRequest, InternalFoldersFolderResponse, InternalFoldersProfile } from '@/api/schema'

export type Folder = InternalFoldersFolderResponse
export type FolderRequest = InternalFoldersFolderRequest
export type Profile = InternalFoldersProfile
export type Mode = Profile['mode']

export const MODES: readonly Mode[] = ['lossy', 'lossless']
