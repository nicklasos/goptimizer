import fs from 'fs'
import os from 'os'
import path from 'path'
import { test, expect } from '@playwright/test'
import { assertTestDatabasePath } from '../helpers/db-helper'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { DashboardPage } from '../pages/DashboardPage'
import { PlanPage } from '../pages/PlanPage'

/** A run takes a moment and the pages poll every few seconds. */
const RUN_TIMEOUT = 30_000

async function sqlitePlanToNewDirectory(label: string) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-run-'))
  const stamp = Date.now()
  const storage = await dbHelper.createStorage(`E2E ${label} storage ${stamp}`, dir)
  const plan = await dbHelper.createSQLitePlan(`E2E ${label} ${stamp}`, storage.id, assertTestDatabasePath(process.env.TEST_DATABASE_PATH))
  return { dir, plan }
}

function archivesIn(dir: string): string[] {
  return (fs.readdirSync(dir, { recursive: true }) as string[]).filter((name) => name.endsWith('.db.gz'))
}

test.describe('Running backup plans', () => {
  test('Run now backs up the test database into the storage', async ({ page }) => {
    await loginAs(page, 'admin')
    const { dir, plan } = await sqlitePlanToNewDirectory('Run now')
    const dashboard = new DashboardPage(page)

    await dashboard.gotoDashboard()
    await dashboard.runNow(plan.id)
    await expect(dashboard.card(plan.id)).toHaveAttribute('data-health', 'ok', { timeout: RUN_TIMEOUT })

    const archives = archivesIn(dir)
    expect(archives).toHaveLength(1)
    const archive = fs.readFileSync(path.join(dir, archives[0]))
    expect(archive.subarray(0, 2)).toEqual(Buffer.from([0x1f, 0x8b]))
  })

  test('a plan whose storage directory is gone shows its run as failed, with the error and the log', async ({ page }) => {
    await loginAs(page, 'admin')
    const { dir, plan } = await sqlitePlanToNewDirectory('Broken')
    fs.rmSync(dir, { recursive: true, force: true })
    const dashboard = new DashboardPage(page)
    const planPage = new PlanPage(page)

    await planPage.gotoPlan(plan.id)
    await planPage.runButton.click()
    await expect(planPage.health).toHaveAttribute('data-health', 'failed', { timeout: RUN_TIMEOUT })
    await expect(planPage.runButton).toBeEnabled()

    await planPage.runRows.first().click()
    await expect(planPage.drawerStatus).toHaveAttribute('data-status', 'failed')
    await expect(planPage.runError).toBeVisible()
    await expect(planPage.log).toContainText(plan.name)

    await dashboard.gotoDashboard()
    await expect(dashboard.card(plan.id)).toHaveAttribute('data-health', 'failed')
  })
})
