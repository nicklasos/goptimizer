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

export interface AppInternalErrsValidationErrorResponse {
  error_key: string;
  errors: Record<string, string[]>;
  message: string;
}

export interface AppInternalPlansFilesTarget {
  directory: string;
  original_locations: boolean;
}

export interface AppInternalPlansFilesTargetInput {
  directory?: string;
  original_locations?: boolean;
}

export interface AppInternalPlansMySQLTarget {
  create_database: boolean;
  database: string;
  host: string;
  port: number;
  user: string;
}

export interface AppInternalPlansMySQLTargetInput {
  create_database?: boolean;
  /** @maxLength 255 */
  database: string;
  /** @maxLength 255 */
  host?: string;
  password?: string;
  /**
   * @min 1
   * @max 65535
   */
  port?: number;
  /** @maxLength 255 */
  user?: string;
}

export interface AppInternalPlansPostgresTarget {
  create_database: boolean;
  database: string;
  host: string;
  port: number;
  ssl_mode: "disable" | "require" | "verify-full";
  user: string;
}

export interface AppInternalPlansPostgresTargetInput {
  create_database?: boolean;
  /** @maxLength 255 */
  database: string;
  /** @maxLength 255 */
  host?: string;
  password?: string;
  /**
   * @min 1
   * @max 65535
   */
  port?: number;
  ssl_mode?: "disable" | "require" | "verify-full";
  /** @maxLength 255 */
  user?: string;
}

export interface AppInternalPlansRestoreRequest {
  confirm?: string;
  files?: AppInternalPlansFilesTargetInput;
  mysql?: AppInternalPlansMySQLTargetInput;
  postgres?: AppInternalPlansPostgresTargetInput;
  sqlite?: AppInternalPlansSQLiteTargetInput;
}

export interface AppInternalPlansRestoreTarget {
  files?: AppInternalPlansFilesTarget;
  mysql?: AppInternalPlansMySQLTarget;
  postgres?: AppInternalPlansPostgresTarget;
  source_kind: "postgres" | "mysql" | "sqlite" | "files";
  sqlite?: AppInternalPlansSQLiteTarget;
}

export interface AppInternalPlansSQLiteTarget {
  path: string;
}

export interface AppInternalPlansSQLiteTargetInput {
  path: string;
}

export type AppInternalPlansSourceKind =
  | "postgres"
  | "mysql"
  | "sqlite"
  | "files";

export type AppInternalStoragesKind = "local" | "s3" | "gcs";

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

export interface InternalPlansDashboardDataResponse {
  data: InternalPlansDashboardResponse;
}

export interface InternalPlansDashboardPlanResponse {
  created_at: string;
  days: InternalPlansDayResponse[];
  enabled: boolean;
  files?: InternalPlansFilesConfig;
  folder: string;
  health:
    | "restoring"
    | "running"
    | "failed"
    | "paused"
    | "overdue"
    | "never"
    | "ok";
  id: number;
  keep_monthly: number;
  keep_weekly: number;
  last_run?: InternalPlansLastRunResponse | null;
  last_size_bytes?: number | null;
  last_success_at?: string | null;
  max_age_days: number;
  mysql?: InternalPlansMySQLConfig;
  name: string;
  next_run_at?: string | null;
  postgres?: InternalPlansPostgresConfig;
  schedules: InternalPlansSchedule[];
  secret_set: boolean;
  source_kind: "postgres" | "mysql" | "sqlite" | "files";
  sqlite?: InternalPlansSQLiteConfig;
  storages: InternalPlansPlanStorageResponse[];
  updated_at: string;
}

export interface InternalPlansDashboardResponse {
  plans: InternalPlansDashboardPlanResponse[];
  summary: InternalPlansHealthSummaryResponse;
}

export interface InternalPlansDayResponse {
  /** @example "2026-10-17" */
  date: string;
  failed: number;
  succeeded: number;
}

export interface InternalPlansFilesConfig {
  excludes: string[];
  paths: string[];
}

export interface InternalPlansFilesInput {
  excludes?: string[];
  /** @minItems 1 */
  paths: string[];
}

export type InternalPlansHealth =
  | "restoring"
  | "running"
  | "failed"
  | "paused"
  | "overdue"
  | "never"
  | "ok";

export interface InternalPlansHealthSummaryResponse {
  failed: number;
  never: number;
  ok: number;
  overdue: number;
  paused: number;
  restoring: number;
  running: number;
}

export interface InternalPlansLastRunResponse {
  finished_at?: string | null;
  id: number;
  size_bytes?: number | null;
  started_at?: string | null;
  status: "queued" | "running" | "succeeded" | "failed";
  trigger: "schedule" | "manual";
}

export interface InternalPlansMySQLConfig {
  database: string;
  host: string;
  port: number;
  user: string;
}

export interface InternalPlansMySQLInput {
  /** @maxLength 255 */
  database: string;
  /** @maxLength 255 */
  host: string;
  password?: string;
  /**
   * @min 1
   * @max 65535
   */
  port?: number;
  /** @maxLength 255 */
  user: string;
}

export interface InternalPlansPaginatedPlansResponse {
  data: InternalPlansPlanResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalPlansPlanDataResponse {
  data: InternalPlansPlanResponse;
}

export interface InternalPlansPlanRequest {
  enabled: boolean;
  files?: InternalPlansFilesInput;
  /**
   * @min 0
   * @max 240
   */
  keep_monthly: number;
  /**
   * @min 0
   * @max 520
   */
  keep_weekly: number;
  /**
   * @min 0
   * @max 3650
   */
  max_age_days: number;
  mysql?: InternalPlansMySQLInput;
  /** @maxLength 100 */
  name: string;
  postgres?: InternalPlansPostgresInput;
  schedules: InternalPlansScheduleInput[];
  source_kind: "postgres" | "mysql" | "sqlite" | "files";
  sqlite?: InternalPlansSQLiteInput;
  /** @minItems 1 */
  storage_ids: number[];
}

export interface InternalPlansPlanResponse {
  created_at: string;
  enabled: boolean;
  files?: InternalPlansFilesConfig;
  folder: string;
  health:
    | "restoring"
    | "running"
    | "failed"
    | "paused"
    | "overdue"
    | "never"
    | "ok";
  id: number;
  keep_monthly: number;
  keep_weekly: number;
  last_run?: InternalPlansLastRunResponse | null;
  last_size_bytes?: number | null;
  last_success_at?: string | null;
  max_age_days: number;
  mysql?: InternalPlansMySQLConfig;
  name: string;
  next_run_at?: string | null;
  postgres?: InternalPlansPostgresConfig;
  schedules: InternalPlansSchedule[];
  secret_set: boolean;
  source_kind: "postgres" | "mysql" | "sqlite" | "files";
  sqlite?: InternalPlansSQLiteConfig;
  storages: InternalPlansPlanStorageResponse[];
  updated_at: string;
}

export interface InternalPlansPlanStorageResponse {
  id: number;
  kind: "local" | "s3" | "gcs";
  name: string;
}

export interface InternalPlansPostgresConfig {
  database: string;
  host: string;
  port: number;
  ssl_mode: "disable" | "require" | "verify-full";
  user: string;
}

export interface InternalPlansPostgresInput {
  /** @maxLength 255 */
  database: string;
  /** @maxLength 255 */
  host: string;
  password?: string;
  /**
   * @min 1
   * @max 65535
   */
  port?: number;
  ssl_mode?: "disable" | "require" | "verify-full";
  /** @maxLength 255 */
  user: string;
}

export interface InternalPlansSQLiteConfig {
  path: string;
}

export interface InternalPlansSQLiteInput {
  path: string;
}

export interface InternalPlansSchedule {
  days: InternalPlansWeekday[];
  time: string;
}

export interface InternalPlansScheduleInput {
  days: ("mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun")[];
  time: string;
}

export interface InternalPlansSetEnabledRequest {
  enabled: boolean;
}

export type InternalPlansSourceKind = "postgres" | "mysql" | "sqlite" | "files";

export interface InternalPlansSourceRequest {
  files?: InternalPlansFilesInput;
  mysql?: InternalPlansMySQLInput;
  postgres?: InternalPlansPostgresInput;
  source_kind: "postgres" | "mysql" | "sqlite" | "files";
  sqlite?: InternalPlansSQLiteInput;
}

export interface InternalPlansSourceTestDataResponse {
  data: InternalPlansSourceTestResponse;
}

export interface InternalPlansSourceTestResponse {
  error?: string;
  ok: boolean;
  server_version?: string;
  step?: "connect" | "tool" | "path";
  tool?: InternalPlansToolResponse;
}

export interface InternalPlansToolResponse {
  found: boolean;
  name: string;
  version?: string;
}

export type InternalPlansWeekday =
  | "mon"
  | "tue"
  | "wed"
  | "thu"
  | "fri"
  | "sat"
  | "sun";

export interface InternalRunsArchiveResponse {
  sha256: string;
  created_at: string;
  deleted_at?: string | null;
  id: number;
  object_key: string;
  plan_id: number;
  run_id: number;
  size_bytes: number;
  storage_id: number;
  storage_kind: "local" | "s3" | "gcs";
  storage_name: string;
}

export interface InternalRunsDownloadLinkDataResponse {
  data: InternalRunsDownloadLinkResponse;
}

export interface InternalRunsDownloadLinkResponse {
  expires_at: string;
  token: string;
}

export interface InternalRunsPaginatedArchivesResponse {
  data: InternalRunsArchiveResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalRunsPaginatedRestoresResponse {
  data: InternalRunsRestoreResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalRunsPaginatedRunsResponse {
  data: InternalRunsRunResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalRunsRestoreArchiveResponse {
  created_at: string;
  object_key: string;
  size_bytes: number;
  storage_name: string;
}

export interface InternalRunsRestoreDetailDataResponse {
  data: InternalRunsRestoreDetailResponse;
}

export interface InternalRunsRestoreDetailResponse {
  archive?: InternalRunsRestoreArchiveResponse | null;
  archive_id?: number | null;
  created_at: string;
  created_by?: InternalRunsRestoreUserResponse | null;
  duration_ms?: number | null;
  error?: string | null;
  finished_at?: string | null;
  id: number;
  log: string;
  plan_id: number;
  secret_set: boolean;
  started_at?: string | null;
  status: "queued" | "running" | "succeeded" | "failed";
  target: AppInternalPlansRestoreTarget;
}

export interface InternalRunsRestoreResponse {
  archive?: InternalRunsRestoreArchiveResponse | null;
  archive_id?: number | null;
  created_at: string;
  created_by?: InternalRunsRestoreUserResponse | null;
  duration_ms?: number | null;
  error?: string | null;
  finished_at?: string | null;
  id: number;
  plan_id: number;
  secret_set: boolean;
  started_at?: string | null;
  status: "queued" | "running" | "succeeded" | "failed";
  target: AppInternalPlansRestoreTarget;
}

export interface InternalRunsRestoreTargetDataResponse {
  data: AppInternalPlansRestoreTarget;
}

export interface InternalRunsRestoreUserResponse {
  email: string;
  id: number;
  name: string;
}

export interface InternalRunsRunDataResponse {
  data: InternalRunsRunResponse;
}

export interface InternalRunsRunDetailDataResponse {
  data: InternalRunsRunDetailResponse;
}

export interface InternalRunsRunDetailResponse {
  archives: InternalRunsArchiveResponse[];
  created_at: string;
  duration_ms?: number | null;
  error?: string | null;
  finished_at?: string | null;
  id: number;
  log: string;
  plan_id: number;
  scheduled_for?: string | null;
  size_bytes?: number | null;
  started_at?: string | null;
  status: "queued" | "running" | "succeeded" | "failed";
  trigger: "schedule" | "manual";
}

export interface InternalRunsRunResponse {
  created_at: string;
  duration_ms?: number | null;
  error?: string | null;
  finished_at?: string | null;
  id: number;
  plan_id: number;
  scheduled_for?: string | null;
  size_bytes?: number | null;
  started_at?: string | null;
  status: "queued" | "running" | "succeeded" | "failed";
  trigger: "schedule" | "manual";
}

export type InternalRunsStatus = "queued" | "running" | "succeeded" | "failed";

export type InternalRunsTrigger = "schedule" | "manual";

export interface InternalStoragesGCSConfig {
  bucket: string;
  prefix: string;
}

export interface InternalStoragesGCSInput {
  bucket: string;
  credentials_json?: string;
  prefix?: string;
}

export type InternalStoragesKind = "local" | "s3" | "gcs";

export interface InternalStoragesLocalConfig {
  path: string;
}

export interface InternalStoragesLocalInput {
  path: string;
}

export interface InternalStoragesObjectResponse {
  is_dir: boolean;
  modified_at?: string;
  name: string;
  size: number;
}

export interface InternalStoragesObjectsResponse {
  data: InternalStoragesObjectResponse[];
}

export interface InternalStoragesPaginatedStoragesResponse {
  data: InternalStoragesStorageResponse[];
  pagination: InternalPaginationMeta;
}

export interface InternalStoragesS3Config {
  bucket: string;
  endpoint: string;
  prefix: string;
  region: string;
  use_path_style: boolean;
}

export interface InternalStoragesS3Input {
  access_key_id?: string;
  bucket: string;
  endpoint?: string;
  prefix?: string;
  region?: string;
  secret_access_key?: string;
  use_path_style?: boolean;
}

export interface InternalStoragesStorageDataResponse {
  data: InternalStoragesStorageResponse;
}

export interface InternalStoragesStorageRequest {
  s3?: InternalStoragesS3Input;
  gcs?: InternalStoragesGCSInput;
  kind: "local" | "s3" | "gcs";
  local?: InternalStoragesLocalInput;
  /** @maxLength 100 */
  name: string;
}

export interface InternalStoragesStorageResponse {
  s3?: InternalStoragesS3Config;
  created_at: string;
  gcs?: InternalStoragesGCSConfig;
  id: number;
  kind: "local" | "s3" | "gcs";
  local?: InternalStoragesLocalConfig;
  name: string;
  secret_set: boolean;
  updated_at: string;
}

export interface InternalStoragesTestResultDataResponse {
  data: InternalStoragesTestResultResponse;
}

export interface InternalStoragesTestResultResponse {
  error?: string;
  ok: boolean;
  step?: "connect" | "write" | "read" | "delete";
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
