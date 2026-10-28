import { exec } from 'child_process'
import { promisify } from 'util'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
import dotenv from 'dotenv'
import dbHelper from './helpers/db-helper'

const execAsync = promisify(exec)
const __dirname = path.dirname(fileURLToPath(import.meta.url))
dotenv.config({ path: path.resolve(__dirname, '../../.env') })

async function globalSetup() {
  console.log('Setting up E2E test environment...')

  try {
    fs.mkdirSync(path.join(__dirname, '.auth'), { recursive: true })

    console.log('Setting up gogo test database...')
    await execAsync('cd ../backend && set -a && . ./.env && set +a && make test-db-setup')

    await dbHelper.cleanupTestData()
    await dbHelper.disconnect()
    console.log('E2E test environment ready')
  } catch (error) {
    console.error('Failed to set up E2E test environment:', error instanceof Error ? error.message : error)
    process.exit(1)
  }
}

export default globalSetup
