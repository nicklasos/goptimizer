import fs from 'fs'
import os from 'os'
import path from 'path'
import { DatabaseSync } from 'node:sqlite'
import { test, expect, type Page } from '@playwright/test'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { PlanPage } from '../pages/PlanPage'

/** A run and a restore take a moment and the pages poll every few seconds. */
const JOB_TIMEOUT = 30_000

function notes(file: string): string[] {
  const db = new DatabaseSync(file, { readOnly: true })
  try {
    return (db.prepare('SELECT body FROM notes ORDER BY id').all() as { body: string }[]).map((row) => row.body)
  } finally {
    db.close()
  }
}

/** A small SQLite database of its own in a temp directory, backed up once through the page. */
async function backedUpSQLitePlan(page: Page, label: string) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-restore-'))
  const source = path.join(dir, 'notes.db')
  const db = new DatabaseSync(source)
  db.exec("CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT); INSERT INTO notes (body) VALUES ('alpha'), ('beta')")
  db.close()
  const stamp = Date.now()
  const storage = await dbHelper.createStorage(`E2E ${label} storage ${stamp}`, fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-restore-storage-')))
  const plan = await dbHelper.createSQLitePlan(`E2E ${label} ${stamp}`, storage.id, source)

  const planPage = new PlanPage(page)
  await planPage.gotoPlan(plan.id)
  await planPage.runButton.click()
  await expect(planPage.health).toHaveAttribute('data-health', 'ok', { timeout: JOB_TIMEOUT })
  await planPage.openTab('archives')
  await expect(planPage.restoreButtons).toHaveCount(1)
  return { dir, source, planPage }
}

test.describe('Restoring archives', () => {
  test('a SQLite archive restores to a new file, and the log shows it', async ({ page }) => {
    await loginAs(page, 'admin')
    const { dir, source, planPage } = await backedUpSQLitePlan(page, 'Restore copy')
    const copy = path.join(dir, 'copy.db')

    await planPage.restoreButtons.first().click()
    await expect(planPage.restoreSQLitePath).toHaveValue(source)
    await expect(planPage.restoreOwnWarning).toBeVisible()
    await planPage.restoreSQLitePath.fill(copy)
    await expect(planPage.restoreOwnWarning).toBeHidden()
    await planPage.restoreStart.click()

    await expect(planPage.restoreStatus).toHaveAttribute('data-status', 'succeeded', { timeout: JOB_TIMEOUT })
    await expect(planPage.restoreLog).toContainText('Integrity check: ok')
    expect(notes(copy)).toEqual(['alpha', 'beta'])
    await expect(planPage.restoreRows).toHaveCount(1)
  })

  test('restoring over the source needs its path typed first', async ({ page }) => {
    await loginAs(page, 'admin')
    const { dir, source, planPage } = await backedUpSQLitePlan(page, 'Restore over')
    const db = new DatabaseSync(source)
    db.exec("DELETE FROM notes WHERE body = 'alpha'")
    db.close()

    await planPage.restoreButtons.first().click()
    await expect(planPage.restoreOwnWarning).toBeVisible()
    await expect(planPage.restoreStart).toBeDisabled()
    await planPage.restoreConfirm.fill(`${source}x`)
    await expect(planPage.restoreStart).toBeDisabled()
    await planPage.restoreConfirm.fill(source)
    await expect(planPage.restoreStart).toBeEnabled()
    await planPage.restoreStart.click()

    await expect(planPage.restoreStatus).toHaveAttribute('data-status', 'succeeded', { timeout: JOB_TIMEOUT })
    expect(notes(source)).toEqual(['alpha', 'beta'])
    expect(fs.readdirSync(dir).some((name) => name.startsWith('notes.db.before-restore-'))).toBe(true)
  })
})
