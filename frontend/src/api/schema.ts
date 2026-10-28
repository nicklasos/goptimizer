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
  name: string;
  roles: string[];
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
  created_at: string;
  enabled: boolean;
  full_scan_interval_hours: number;
  id: number;
  kind: "local";
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

export interface InternalUsersCreateUserRequest {
  email: string;
  name: string;
  /** @minLength 8 */
  password: string;
  role: "super-admin" | "admin" | "user";
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
  name: string;
  roles: string[];
  updated_at: string;
}
