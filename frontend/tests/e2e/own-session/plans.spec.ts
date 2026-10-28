import fs from 'fs'
import net from 'net'
import os from 'os'
import path from 'path'
import { test, expect } from '@playwright/test'
import { assertTestDatabasePath } from '../helpers/db-helper'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { PlanEditorPage } from '../pages/PlanEditorPage'
import { DashboardPage } from '../pages/DashboardPage'
import { PlanPage } from '../pages/PlanPage'
import { StoragesPage } from '../pages/StoragesPage'

/** A local storage the API can write to, named so cleanup finds it. */
async function storage(label: string) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-plan-'))
  return dbHelper.createStorage(`E2E ${label} ${Date.now()}`, dir)
}

/** A port nothing listens on, so a connection is refused at once. */
async function closedPort(): Promise<number> {
  const server = net.createServer()
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const { port } = server.address() as net.AddressInfo
  await new Promise((resolve) => server.close(resolve))
  return port
}

test.describe('Backup plans as an admin', () => {
  test('creates a SQLite plan with two schedules, tests it, edits it and deletes it', async ({ page }) => {
    await loginAs(page, 'admin')
    const target = await storage('Plan target')
    const name = `E2E Plan ${Date.now()}`
    const dashboard = new DashboardPage(page)
    const planPage = new PlanPage(page)
    const editor = new PlanEditorPage(page)

    await dashboard.gotoDashboard()
    await dashboard.clickAdd()
    await editor.nameInput.fill(name)
    await editor.chooseKind('sqlite')
    await editor.sqlitePathInput.fill(assertTestDatabasePath(process.env.TEST_DATABASE_PATH))
    await editor.testConnection()
    await expect(editor.testPassed).toBeVisible()
    await expect(editor.serverVersion).toBeVisible()
    await expect(editor.toolVersion).toHaveAttribute('data-found', 'true')

    await expect(editor.scheduleRows).toHaveCount(1)
    await editor.setTime(0, '03:00')
    await editor.addScheduleButton.click()
    await expect(editor.scheduleRows).toHaveCount(2)
    await editor.setTime(1, '18:00')
    for (const day of ['mon', 'tue', 'wed', 'thu', 'fri'] as const) await editor.toggleDay(1, day)
    await expect(editor.nextRun).toBeVisible()
    await expect(editor.nextRun).toHaveAttribute('data-next', /T(03|18):00:00\.000Z$/)

    await editor.chooseStorage(target.name)
    await editor.setRetention(30, 8, 12)
    await expect(editor.retentionSummary).toContainText('30')
    await editor.save()

    await expect(planPage.root).toBeVisible()
    await expect(page.getByTestId('plan-title')).toHaveText(name)
    await expect(planPage.scheduleSummary).toContainText('03:00')
    await expect(planPage.scheduleSummary).toContainText('18:00')
    await expect(planPage.nextRun).toContainText('UTC')
    await planPage.openTab('storages')
    await expect(page.getByTestId('plan-storages-list')).toContainText(target.name)

    await planPage.editButton.click()
    await expect(editor.root).toBeVisible()
    await expect(editor.sqlitePathInput).toHaveValue(assertTestDatabasePath(process.env.TEST_DATABASE_PATH))
    await expect(editor.scheduleRows).toHaveCount(2)
    await editor.removeSchedule(1)
    await expect(editor.scheduleRows).toHaveCount(1)
    await editor.save()
    await expect(planPage.scheduleSummary).toContainText('03:00')
    await expect(planPage.scheduleSummary).not.toContainText('18:00')

    const id = Number(new URL(page.url()).pathname.split('/').pop())
    await planPage.editButton.click()
    await editor.delete()
    await expect(dashboard.root).toBeVisible()
    await expect(dashboard.card(id)).toHaveCount(0)
  })

  test('fills a Postgres source from a pasted URL and reports a refused connection', async ({ page }) => {
    await loginAs(page, 'admin')
    const port = await closedPort()
    const dashboard = new DashboardPage(page)
    const editor = new PlanEditorPage(page)

    await dashboard.gotoDashboard()
    await dashboard.clickAdd()
    await editor.chooseKind('postgres')
    await editor.urlInput.fill(`postgres://shop:s3cret@127.0.0.1:${port}/shop?sslmode=require`)
    await editor.urlInput.press('Enter')
    await expect(editor.urlInput).toHaveValue('')
    await expect(editor.hostInput).toHaveValue('127.0.0.1')
    await expect(editor.portInput).toHaveValue(String(port))
    await expect(editor.databaseInput).toHaveValue('shop')
    await expect(editor.userInput).toHaveValue('shop')
    await expect(editor.passwordInput).toHaveValue('s3cret')

    await editor.testConnection()
    await expect(editor.testFailed).toHaveAttribute('data-step', 'connect')
  })

  test('shows a name that is taken under the name field', async ({ page }) => {
    await loginAs(page, 'admin')
    const target = await storage('Plan names')
    const taken = await dbHelper.createPlan(`E2E Taken ${Date.now()}`, target.id)
    const dashboard = new DashboardPage(page)
    const editor = new PlanEditorPage(page)

    await dashboard.gotoDashboard()
    await dashboard.clickAdd()
    await editor.nameInput.fill(taken.name.toUpperCase())
    await editor.chooseKind('files')
    await page.getByTestId('plan-files-path-input-0').fill(os.tmpdir())
    await editor.chooseStorage(target.name)
    await editor.saveButton.click()
    await expect(editor.nameInput).toHaveAttribute('aria-invalid', 'true')
    await expect(editor.root).toBeVisible()
  })

  test('a storage that a plan uses cannot be deleted, and the page says which plan', async ({ page }) => {
    await loginAs(page, 'admin')
    const used = await storage('Used')
    const plan = await dbHelper.createPlan(`E2E Uses storage ${Date.now()}`, used.id)
    const storages = new StoragesPage(page)

    await storages.gotoList()
    await storages.row(used.name).locator('[data-testid^="delete-storage-button-"]').click()
    await page.locator('[data-testid^="confirm-delete-storage-button-"]').click()
    await expect(page.getByTestId('storage-in-use-alert')).toContainText(plan.name)
    await expect(storages.row(used.name)).toBeVisible()
  })
})

test.describe('Backup plans as a plain user', () => {
  test('sees no plans and is refused on the plan pages', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)
    const dashboard = new DashboardPage(page)

    await shell.goto('/')
    await expect(shell.navItem('dashboard')).toBeVisible()
    await expect(dashboard.noAccess).toBeVisible()

    await dashboard.goto('/plans')
    await expect(page).toHaveURL(/\/$/)
    await expect(dashboard.noAccess).toBeVisible()
    for (const target of ['/plans/new', '/plans/1', '/plans/1/edit']) {
      await dashboard.goto(target)
      await expect(dashboard.forbidden).toBeVisible()
    }
  })
})
