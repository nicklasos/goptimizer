import type {
  ImageResponse,
  InternalFoldersFolderRequest,
  InternalFoldersFolderResponse,
  InternalFoldersProfile,
  InternalImagesDaySavingsResponse,
  InternalImagesImagesPage,
  InternalScansKind,
  ReasonResponse,
  ScanResponse,
} from '@/api/schema'

export type Folder = InternalFoldersFolderResponse
export type FolderRequest = InternalFoldersFolderRequest
export type Profile = InternalFoldersProfile
export type Mode = Profile['mode']
export type Scan = ScanResponse
export type ScanKind = InternalScansKind
export type ScanStatus = Scan['status']
export type Image = ImageResponse
export type ImageStatus = Image['status']
export type ImagesPage = InternalImagesImagesPage
export type ImageOrder = ImagesPage['order']
export type Reason = ReasonResponse
export type ReasonCode = NonNullable<Reason['code']>
export type DaySavings = InternalImagesDaySavingsResponse

export const MODES: readonly Mode[] = ['lossy', 'lossless']
export const SCAN_KINDS: readonly ScanKind[] = ['quick', 'full']
