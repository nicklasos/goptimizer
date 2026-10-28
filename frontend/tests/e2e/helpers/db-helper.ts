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

  /** Refresh tokens and emailed tokens go with their user (ON DELETE CASCADE). */
  async cleanupTestData(): Promise<void> {
    this.connection().exec(`DELETE FROM users WHERE email LIKE 'e2e-%@example.com'`)
  }
}

const dbHelper = new DatabaseHelper()
export default dbHelper
