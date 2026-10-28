import { createHash, randomBytes } from 'crypto'
import bcrypt from 'bcryptjs'
import dotenv from 'dotenv'
import path from 'path'
import { DatabaseSync } from 'node:sqlite'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const backendDir = path.resolve(__dirname, '../../../../backend')
dotenv.config({ path: path.resolve(__dirname, '../../../.env') })
dotenv.config({ path: path.join(backendDir, '.env') })

export type TestRole = 'super-admin' | 'admin' | 'user'

export interface TestUser {
  id: number
  email: string
  name: string
  password: string
  roles: TestRole[]
}

interface CreateUserOptions {
  email?: string
  password?: string
  name?: string
  roles?: TestRole[]
  emailVerified?: boolean
}

export type EmailTokenPurpose = 'password_reset' | 'email_verification'

export type RunStatus = 'queued' | 'running' | 'succeeded' | 'failed'

interface CreatePlanOptions {
  paths?: string[]
  schedules?: { time: string; days: string[] }[]
  enabled?: boolean
}

interface CreateRunOptions {
  status: RunStatus
  /** A finished run ends a minute after it started. */
  startedAt?: Date
  sizeBytes?: number
  log?: string
  error?: string
}

/** A timestamp as SQLite writes it, which the API reads back. */
const sqliteTime = (date: Date) => date.toISOString().replace('T', ' ').replace(/\.\d+Z$/, '')

let sequence = 0

/** Emails start with `e2e-` so cleanup can find every user a test created, including through the UI. */
export function uniqueEmail(label = 'user'): string {
  sequence += 1
  return `e2e-${label}-${Date.now()}-${process.pid}-${sequence}@example.com`
}

/** Cleanup deletes rows unconditionally, so refuse anything that is not clearly a test database. */
export function assertTestDatabasePath(rawPath: string | undefined): string {
  if (!rawPath) throw new Error('TEST_DATABASE_PATH environment variable is required')
  if (!path.basename(rawPath).endsWith('_test.db')) {
    throw new Error(`Refusing to use database "${rawPath}": TEST_DATABASE_PATH must point at a file whose name ends in "_test.db"`)
  }
  return path.resolve(backendDir, rawPath)
}

export class DatabaseHelper {
  private db: DatabaseSync | null = null

  private connection(): DatabaseSync {
    if (!this.db) {
      this.db = new DatabaseSync(assertTestDatabasePath(process.env.TEST_DATABASE_PATH))
      // The API writes the same file from another process.
      this.db.exec('PRAGMA busy_timeout = 5000; PRAGMA foreign_keys = ON;')
    }
    return this.db
  }

  async disconnect(): Promise<void> {
    if (this.db) {
      this.db.close()
      this.db = null
    }
  }

  async createUser({
    email = uniqueEmail(),
    password = 'testpassword123',
    name = 'E2E User',
    roles = ['user'],
    emailVerified = true,
  }: CreateUserOptions = {}): Promise<TestUser> {
    const hashed = await bcrypt.hash(password, 4)
    const row = this.connection()
      .prepare(
        `INSERT INTO users (email, name, password, roles, email_verified_at)
         VALUES (?, ?, ?, ?, CASE WHEN ? THEN CURRENT_TIMESTAMP END)
         RETURNING id, email, name`
      )
      .get(email, name, hashed, JSON.stringify(roles), emailVerified ? 1 : 0) as Pick<TestUser, 'id' | 'email' | 'name'>
    return { ...row, password, roles }
  }

  /**
   * Stands in for the emailed link: the API stores only a hash of the token, so a test
   * cannot read one back and plants its own instead.
   */
  async createEmailToken(userId: number, purpose: EmailTokenPurpose, { expired = false } = {}): Promise<string> {
    const token = randomBytes(32).toString('hex')
    this.connection()
      .prepare(
        `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
         VALUES (?, ?, ?, datetime('now', ?))`
      )
      .run(userId, purpose, createHash('sha256').update(token).digest('hex'), expired ? '-1 hour' : '+1 hour')
    return token
  }

  /** A local storage; the name should start with "E2E" so cleanup finds it. */
  async createStorage(name: string, dir: string): Promise<{ id: number; name: string }> {
    return this.connection()
      .prepare(`INSERT INTO storages (name, kind, config) VALUES (?, 'local', ?) RETURNING id, name`)
      .get(name, JSON.stringify({ path: dir })) as { id: number; name: string }
  }

  /**
   * A files plan, 03:00 every day unless `schedules` says otherwise, uploading to the given storage;
   * the name should start with "E2E". It is saved now, so the scheduler runs none of today's slots
   * that have passed.
   */
  async createPlan(
    name: string,
    storageId: number,
    { paths = ['/tmp'], schedules = [{ time: '03:00', days: [] }], enabled = true }: CreatePlanOptions = {}
  ): Promise<{ id: number; name: string }> {
    const db = this.connection()
    const plan = db
      .prepare(
        `INSERT INTO plans (name, source_kind, source_config, schedules, max_age_days, enabled)
         VALUES (?, 'files', ?, ?, 30, ?) RETURNING id, name`
      )
      .get(name, JSON.stringify({ paths, excludes: [] }), JSON.stringify(schedules), enabled ? 1 : 0) as { id: number; name: string }
    db.prepare('INSERT INTO plan_storages (plan_id, storage_id) VALUES (?, ?)').run(plan.id, storageId)
    return plan
  }

  /** A SQLite plan with no schedule, uploading to the given storage; the name should start with "E2E". */
  async createSQLitePlan(name: string, storageId: number, databasePath: string): Promise<{ id: number; name: string }> {
    const db = this.connection()
    const plan = db
      .prepare(`INSERT INTO plans (name, source_kind, source_config, schedules) VALUES (?, 'sqlite', ?, '[]') RETURNING id, name`)
      .get(name, JSON.stringify({ path: databasePath })) as { id: number; name: string }
    db.prepare('INSERT INTO plan_storages (plan_id, storage_id) VALUES (?, ?)').run(plan.id, storageId)
    return plan
  }

  /**
   * A manual run of the plan, recorded as it would be. Use `running` rather than `queued` for one
   * in progress: the test API's worker would pick a queued run up and run it.
   */
  async createRun(planId: number, { status, startedAt = new Date(), sizeBytes, log = '', error }: CreateRunOptions): Promise<{ id: number }> {
    const finished = status === 'succeeded' || status === 'failed'
    return this.connection()
      .prepare(
        `INSERT INTO runs (plan_id, trigger, status, started_at, finished_at, size_bytes, log, error)
         VALUES (?, 'manual', ?, ?, ?, ?, ?, ?) RETURNING id`
      )
      .get(
        planId,
        status,
        status === 'queued' ? null : sqliteTime(startedAt),
        finished ? sqliteTime(new Date(startedAt.getTime() + 60_000)) : null,
        sizeBytes ?? null,
        log,
        error ?? (status === 'failed' ? 'E2E: the dump failed' : null)
      ) as { id: number }
  }

  /** Records an archive of the run; the spec writes the object itself. */
  async createArchive(run: { id: number }, planId: number, storageId: number, key: string, content: Buffer): Promise<{ id: number }> {
    return this.connection()
      .prepare(
        `INSERT INTO archives (run_id, plan_id, storage_id, object_key, size_bytes, sha256)
         VALUES (?, ?, ?, ?, ?, ?) RETURNING id`
      )
      .get(run.id, planId, storageId, key, content.length, createHash('sha256').update(content).digest('hex')) as { id: number }
  }

  /**
   * Refresh tokens and emailed tokens go with their user (ON DELETE CASCADE). Plans and storages
   * are global: theirs start with "E2E", and their runs and archives go with them. Plans go first,
   * because a storage a plan uses cannot be deleted.
   */
  async cleanupTestData(): Promise<void> {
    this.connection().exec(
      `DELETE FROM users WHERE email LIKE 'e2e-%@example.com'; DELETE FROM plans WHERE name LIKE 'E2E%'; DELETE FROM storages WHERE name LIKE 'E2E%'`
    )
  }
}

const dbHelper = new DatabaseHelper()
export default dbHelper
