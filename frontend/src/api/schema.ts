/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

export interface AppInternalErrsErrorResponse {
  details?: Record<string, any>;
  error_key: string;
  message?: string;
  status: number;
  timestamp?: string;
}

export type AppInternalToolsNeed =
  | "png_lossy"
  | "png_lossless"
  | "jpeg"
  | "gif";

export interface AppInternalToolsNeedStatus {
  /** Missing is the preferred program of each step nothing installed can do. */
  missing: string[];
  need: "png_lossy" | "png_lossless" | "jpeg" | "gif";
  satisfied: boolean;
}

export interface AppInternalToolsProgramStatus {
  /** Alternatives can do this program's work when it is missing. Empty means it is required. */
  alternatives: string[];
  apt: string;
  brew: string;
  installed: boolean;
  name: string;
  path: string;
  purpose: string;
  used_for: AppInternalToolsNeed[];
  version: string;
}

export interface AppInternalToolsReport {
  needs: AppInternalToolsNeedStatus[];
  programs: AppInternalToolsProgramStatus[];
}

export interface ImageResponse {
  attempts: number;
  folder_id: number;
  format: "png" | "jpeg" | "gif";
  id: number;
  lossless_reason?: ReasonResponse | null;
  next_attempt_at?: string | null;
  original_size?: number | null;
  path: string;
  reason?: ReasonResponse | null;
  size: number;
  status:
    | "pending"
    | "optimizing"
    | "optimized"
    | "kept"
    | "skipped"
    | "failed"
    | "gone";
  updated_at: string;
}

export interface InternalAuthForgotPasswordRequest {
  email: string;
}

export interface InternalAuthLoginDataResponse {
  data: InternalAuthLoginResponse;
}

export interface InternalAuthLoginRequest {
  email: string;
  password: string;
}

export interface InternalAuthLoginResponse {
  access_token: string;
  refresh_token: string;
  user: InternalAuthUserResponse;
}

export interface InternalAuthRefreshTokenDataResponse {
  data: InternalAuthRefreshTokenResponse;
}

export interface InternalAuthRefreshTokenRequest {
  refresh_token: string;
}

export interface InternalAuthRefreshTokenResponse {
  access_token: string;
  refresh_token: string;
}

export interface InternalAuthRegisterDataResponse {
  data: InternalAuthRegisterResponse;
}

export interface InternalAuthRegisterRequest {
  email: string;
  name: string;
  /** @minLength 6 */
  password: string;
}

export interface InternalAuthRegisterResponse {
  access_token: string;
  refresh_token: string;
  user: InternalAuthUserResponse;
}

export interface InternalAuthResetPasswordRequest {
  /** @minLength 8 */
  password: string;
  token: string;
}

export interface InternalAuthUpdatePasswordRequest {
  current_password: string;
  /** @minLength 8 */
  new_password: string;
}

export interface InternalAuthUpdateProfileRequest {
  email: string;
  name: string;
}

export interface InternalAuthUserDataResponse {
  data: InternalAuthUserResponse;
}

export interface InternalAuthUserResponse {
  email: string;
  email_verified: boolean;
  id: number;
  is_admin: boolean;
  name: string;
}

export interface InternalAuthVerifyEmailRequest {
  token: string;
}

export interface InternalFoldersFolderDataResponse {
  data: InternalFoldersFolderResponse;
}

export interface InternalFoldersFolderRequest {
  /**
   * @min 1
   * @max 720
   */
  full_scan_interval_hours?: number;
  kind?: "local";
  /** @maxLength 100 */
  name: string;
  path: string;
  profile?: InternalFoldersProfile;
  /**
   * @min 5
   * @max 10080
   */
  scan_interval_minutes?: number;
}

export interface InternalFoldersFolderResponse {
  bytes_saved: number;
  created_at: string;
  enabled: boolean;
  full_scan_interval_hours: number;
  id: number;
  image_counts: Record<string, number>;
  kind: "local";
  last_scan?: ScanResponse | null;
  missing_tools: string[];
  name: string;
  path: string;
  profile: InternalFoldersProfile;
  scan_interval_minutes: number;
  updated_at: string;
}

export interface InternalFoldersFoldersResponse {
  data: InternalFoldersFolderResponse[];
}

export type InternalFoldersKind = "local";

export type InternalFoldersMode = "lossy" | "lossless";

export interface InternalFoldersProfile {
  excludes: string[];
  gif_lossy: number;
  jpeg_max_quality: number;
  min_age_minutes: number;
  min_saving_percent: number;
  min_ssim: number;
  mode: "lossy" | "lossless";
  png_quality_max: number;
  png_quality_min: number;
}

export interface InternalFoldersSetEnabledRequest {
  enabled: boolean;
}

export interface InternalImagesDailySavingsResponse {
  data: InternalImagesDaySavingsResponse[];
}

export interface InternalImagesDaySavingsResponse {
  bytes_saved: number;
  day: string;
  optimized: number;
}

export interface InternalImagesImageDataResponse {
  data: ImageResponse;
}

export interface InternalImagesImagesPage {
  items: ImageResponse[];
  next_cursor?: string | null;
  order: "updated" | "saved" | "path";
}

export interface InternalImagesImagesPageResponse {
  data: InternalImagesImagesPage;
}

export interface InternalImagesRetriedData {
  retried: number;
}

export interface InternalImagesRetriedResponse {
  data: InternalImagesRetriedData;
}

export interface InternalMessageData {
  message: string;
}

export interface InternalMessageResponse {
  data: InternalMessageData;
}

export interface InternalPaginationMeta {
  current_page: number;
  last_page: number;
  per_page: number;
  total: number;
}

export type InternalScansKind = "quick" | "full";

export interface InternalScansPaginatedScansResponse {
  data: ScanResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalScansScanDataResponse {
  data: ScanResponse;
}

export interface InternalScansStartScanRequest {
  kind: "quick" | "full";
}

export interface InternalSettingsSettingsDataResponse {
  data: InternalSettingsSettingsResponse;
}

export interface InternalSettingsSettingsRequest {
  /** Nice runs the tools under nice and ionice, so the projects on the server stay fast. */
  nice: boolean;
  /**
   * @min 1
   * @max 16
   */
  scan_workers: number;
  /**
   * Workers is how many images are optimized at once; 0 means one less than the CPU count.
   * @min 0
   * @max 64
   */
  workers: number;
}

export interface InternalSettingsSettingsResponse {
  effective_workers: number;
  nice: boolean;
  scan_workers: number;
  updated_at: string;
  workers: number;
}

export interface InternalSettingsToolsResponse {
  data: AppInternalToolsReport;
}

export interface InternalUsersCreateUserRequest {
  email: string;
  is_admin: boolean;
  name: string;
  /** @minLength 8 */
  password: string;
}

export interface InternalUsersPaginatedUsersResponse {
  data: InternalUsersUserResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalUsersSetPasswordRequest {
  /** @minLength 8 */
  password: string;
}

export interface InternalUsersUpdateUserRequest {
  email: string;
  is_admin: boolean;
  name: string;
}

export interface InternalUsersUserDataResponse {
  data: InternalUsersUserResponse;
}

export interface InternalUsersUserResponse {
  created_at: string;
  email: string;
  email_verified: boolean;
  id: number;
  is_admin: boolean;
  name: string;
  updated_at: string;
}

export interface ReasonResponse {
  code?:
    | "symlink"
    | "not_regular_file"
    | "hard_link"
    | "access_control_list"
    | "wrong_content"
    | "apng"
    | "extra_data"
    | "too_many_pixels"
    | "too_large"
    | "unreadable"
    | "already_optimized"
    | "too_new"
    | "changed_while_optimizing"
    | "interrupted"
    | "tool_missing"
    | "tool_failed"
    | "swap_failed"
    | "no_saving"
    | "format_changed"
    | "result_undecodable"
    | "dimensions_changed"
    | "frames_changed"
    | "animation_changed"
    | "pixels_changed"
    | "ssim_below_threshold"
    | "color_changed"
    | "metadata_changed"
    | null;
  detail?: string | null;
}

export interface ScanResponse {
  dirs_listed: number;
  dirs_seen: number;
  error?: string | null;
  files_changed: number;
  files_gone: number;
  files_new: number;
  files_seen: number;
  finished_at?: string | null;
  folder_id: number;
  id: number;
  kind: "quick" | "full";
  queued_at: string;
  started_at?: string | null;
  status: "queued" | "running" | "succeeded" | "failed";
  trigger: "schedule" | "manual";
}
