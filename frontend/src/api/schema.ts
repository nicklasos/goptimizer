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
