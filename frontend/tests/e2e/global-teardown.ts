import dbHelper from './helpers/db-helper'

async function globalTeardown() {
  try {
    await dbHelper.cleanupTestData()
    await dbHelper.disconnect()
  } catch (error) {
    console.error('Failed to clean up E2E test environment:', error instanceof Error ? error.message : error)
  }
}

export default globalTeardown
