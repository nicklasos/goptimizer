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

export interface TestUser {
  id: number
  email: string
  name: string
  password: string
  isAdmin: boolean
}

interface CreateUserOptions {
  email?: string
  password?: string
  name?: string
  isAdmin?: boolean
  emailVerified?: boolean
}

export type EmailTokenPurpose = 'password_reset' | 'email_verification'

export type ImageStatus = 'pending' | 'optimizing' | 'optimized' | 'kept' | 'skipped' | 'failed' | 'gone'

export interface SeedImage {
  /** Relative to the folder, such as `shop/banners/hero.png`. */
  path: string
  status: ImageStatus
  size?: number
  /** For an optimized image: its size before. */
  originalSize?: number
  /** As the optimizer stores it: `code` or `code: detail`, a lossless attempt after `; lossless: `. */
  reason?: string
  attempts?: number
  /** For a failed image: minutes from now until it is retried; null when it no longer is. */
  retryInMinutes?: number | null
}

export interface SeedScan {
  kind: 'quick' | 'full'
  trigger: 'schedule' | 'manual'
  status: 'succeeded' | 'failed'
  /** How long ago it started. */
  startedMinutesAgo: number
  durationSeconds: number
  filesSeen?: number
  filesNew?: number
  error?: string
}

export interface SeedDay {
  /** Days before today (UTC); 0 is today. */
  daysAgo: number
  optimized: number
  bytesBefore: number
  bytesAfter: number
}

const FORMATS: Record<string, string> = { png: 'png', jpg: 'jpeg', jpeg: 'jpeg', gif: 'gif' }

/** A time as the API writes it (modernc sqlite, UTC): the image keyset compares these as text. */
function sqliteTime(date: Date): string {
  return `${date.toISOString().slice(0, 19).replace('T', ' ')}+00:00`
}

/** A UTC calendar day, as `daily_stats.day` and `GET /daily-savings` write it. */
export function utcDay(daysAgo = 0): string {
  return new Date(Date.now() - daysAgo * 86_400_000).toISOString().slice(0, 10)
}

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
    isAdmin = false,
    emailVerified = true,
  }: CreateUserOptions = {}): Promise<TestUser> {
    const hashed = await bcrypt.hash(password, 4)
    const row = this.connection()
      .prepare(
        `INSERT INTO users (email, name, password, is_admin, email_verified_at)
         VALUES (?, ?, ?, ?, CASE WHEN ? THEN CURRENT_TIMESTAMP END)
         RETURNING id, email, name`
      )
      .get(email, name, hashed, isAdmin ? 1 : 0, emailVerified ? 1 : 0) as Pick<TestUser, 'id' | 'email' | 'name'>
    return { ...row, password, isAdmin }
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

  /**
   * A folder written straight into the database, paused unless asked: the test API's scheduler and
   * optimizer leave a paused folder alone, so what a spec seeds stays as seeded. Name it `E2E ...`.
   */
  async createFolder({ name, path, enabled = false }: { name: string; path: string; enabled?: boolean }): Promise<number> {
    const row = this.connection()
      .prepare(`INSERT INTO folders (name, path, enabled) VALUES (?, ?, ?) RETURNING id`)
      .get(name, path, enabled ? 1 : 0) as { id: number }
    return row.id
  }

  /** Images in one transaction, the first the most recently changed. Returns their ids in the same order. */
  async createImages(folderId: number, images: SeedImage[]): Promise<number[]> {
    const db = this.connection()
    const insert = db.prepare(
      `INSERT INTO images (folder_id, dir, name, format, size, mtime_ns, status, original_size, optimized_size, mode, reason,
         attempts, next_attempt_at, discovered_at, optimized_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
       RETURNING id`
    )
    const now = Date.now()
    db.exec('BEGIN')
    try {
      const ids = images.map((image, index) => {
        const slash = image.path.lastIndexOf('/')
        const name = image.path.slice(slash + 1)
        const format = FORMATS[name.split('.').pop()!.toLowerCase()]
        const size = image.size ?? 100_000
        const optimized = image.status === 'optimized'
        const changed = sqliteTime(new Date(now - (index + 1) * 1000))
        const retry = image.retryInMinutes == null ? null : sqliteTime(new Date(now + image.retryInMinutes * 60_000))
        const row = insert.get(
          folderId,
          slash < 0 ? '' : image.path.slice(0, slash),
          name,
          format,
          size,
          (now - 86_400_000) * 1_000_000,
          image.status,
          optimized ? (image.originalSize ?? size * 2) : null,
          optimized ? size : null,
          optimized ? 'lossy' : null,
          image.reason ?? null,
          image.attempts ?? (image.status === 'failed' ? 3 : 0),
          image.status === 'failed' ? retry : null,
          changed,
          optimized ? changed : null,
          changed
        ) as { id: number }
        return row.id
      })
      db.exec('COMMIT')
      return ids
    } catch (error) {
      db.exec('ROLLBACK')
      throw error
    }
  }

  async createScans(folderId: number, scans: SeedScan[]): Promise<number[]> {
    const insert = this.connection().prepare(
      `INSERT INTO scans (folder_id, kind, trigger, status, queued_at, started_at, finished_at, files_seen, files_new, dirs_seen, dirs_listed, error)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?)
       RETURNING id`
    )
    return scans.map((scan) => {
      const started = new Date(Date.now() - scan.startedMinutesAgo * 60_000)
      const row = insert.get(
        folderId,
        scan.kind,
        scan.trigger,
        scan.status,
        sqliteTime(started),
        sqliteTime(started),
        sqliteTime(new Date(started.getTime() + scan.durationSeconds * 1000)),
        scan.filesSeen ?? 0,
        scan.filesNew ?? 0,
        scan.error ?? null
      ) as { id: number }
      return row.id
    })
  }

  async createDailyStats(folderId: number, days: SeedDay[]): Promise<void> {
    const insert = this.connection().prepare(`INSERT INTO daily_stats (folder_id, day, optimized, bytes_before, bytes_after) VALUES (?, ?, ?, ?, ?)`)
    for (const day of days) insert.run(folderId, utcDay(day.daysAgo), day.optimized, day.bytesBefore, day.bytesAfter)
  }

  /** Refresh tokens and emailed tokens go with their user (ON DELETE CASCADE). Folders, seeded or made through the UI, are named `E2E ...`; their images, scans and stats go with them. */
  async cleanupTestData(): Promise<void> {
    this.connection().exec(`DELETE FROM users WHERE email LIKE 'e2e-%@example.com'; DELETE FROM folders WHERE name LIKE 'E2E %'`)
  }
}

const dbHelper = new DatabaseHelper()
export default dbHelper
