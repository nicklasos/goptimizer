import fs from 'fs'
import os from 'os'
import path from 'path'
import { test, expect } from '@playwright/test'
import { assertTestDatabasePath } from '../helpers/db-helper'
import { API_BASE, dbHelper, loginAs } from '../helpers/test-helpers'
import { DashboardPage } from '../pages/DashboardPage'
import { PlanPage } from '../pages/PlanPage'
import { StorageBrowserPage } from '../pages/StorageBrowserPage'

const DAY = 24 * 60 * 60 * 1000

/** The UTC day `daysAgo` before today, at 10:00, and its date as the dashboard keys it. */
function daysAgo(days: number): { at: Date; date: string } {
  const today = new Date()
  const at = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate(), 10) - days * DAY)
  return { at, date: at.toISOString().slice(0, 10) }
}

/** A plan with a history: successes on three days, a failure on another, and an archive in a real directory. */
async function planWithHistory() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-dash-'))
  const stamp = Date.now()
  const storage = await dbHelper.createStorage(`E2E Dash storage ${stamp}`, dir)
  const plan = await dbHelper.createPlan(`E2E Dash healthy ${stamp}`, storage.id)
  for (const [days, status] of [
    [5, 'succeeded'],
    [3, 'succeeded'],
    [2, 'failed'],
  ] as const) {
    await dbHelper.createRun(plan.id, { status, startedAt: daysAgo(days).at, sizeBytes: status === 'succeeded' ? 2048 : undefined })
  }
  const content = Buffer.from(`E2E archive bytes ${stamp}`)
  const latest = await dbHelper.createRun(plan.id, {
    status: 'succeeded',
    startedAt: daysAgo(1).at,
    sizeBytes: content.length,
    log: `E2E log line ${stamp}`,
  })
  const folder = `e2e-dash-healthy-${stamp}`
  const name = `${folder}-20261016T100000Z-${latest.id}.tar.gz`
  fs.mkdirSync(path.join(dir, folder))
  fs.writeFileSync(path.join(dir, folder, name), content)
  const archive = await dbHelper.createArchive(latest, plan.id, storage.id, `${folder}/${name}`, content)
  return { storage, plan, latest, archive, content, folder, name }
}

test.describe('The dashboard as an admin', () => {
  test('shows each plan as a card coloured by its health, and opens the plan page', async ({ page }) => {
    await loginAs(page, 'admin')
    const { storage, plan, latest, archive, content, folder, name } = await planWithHistory()
    const never = await dbHelper.createPlan(`E2E Dash never ${Date.now()}`, storage.id, { schedules: [] })
    const paused = await dbHelper.createPlan(`E2E Dash paused ${Date.now()}`, storage.id, { enabled: false })
    const dashboard = new DashboardPage(page)
    const planPage = new PlanPage(page)

    await dashboard.gotoDashboard()
    await expect(dashboard.card(plan.id)).toHaveAttribute('data-health', 'ok')
    await expect(dashboard.health(plan.id)).toHaveAttribute('data-health', 'ok')
    await expect(dashboard.card(never.id)).toHaveAttribute('data-health', 'never')
    await expect(dashboard.card(paused.id)).toHaveAttribute('data-health', 'paused')
    await expect(page.getByTestId('dashboard-summary-ok')).toBeVisible()

    await expect(dashboard.day(plan.id, daysAgo(1).date)).toHaveAttribute('data-kind', 'succeeded')
    await expect(dashboard.day(plan.id, daysAgo(2).date)).toHaveAttribute('data-kind', 'failed')
    await expect(dashboard.day(plan.id, daysAgo(4).date)).toHaveAttribute('data-kind', 'none')
    await expect(dashboard.day(plan.id, daysAgo(30).date)).toHaveCount(0)
    await dashboard.day(plan.id, daysAgo(2).date).hover()
    await expect(page.getByRole('tooltip')).toContainText(/\b0\b.*\b1\b/)

    await dashboard.open(plan.id)
    await expect(page).toHaveURL(new RegExp(`/plans/${plan.id}$`))
    await expect(planPage.health).toHaveAttribute('data-health', 'ok')

    await expect(planPage.runRows).toHaveCount(4)
    await expect(planPage.chartBars).toHaveCount(4)
    await expect(page.getByTestId(`plan-runs-chart-bar-${latest.id}`)).toHaveAttribute('data-status', 'succeeded')
    await planPage.runRow(latest.id).click()
    await expect(planPage.drawer).toBeVisible()
    await expect(planPage.log).toContainText(`E2E log line`)
    await expect(page.getByTestId(`run-archive-${archive.id}`)).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(planPage.drawer).toBeHidden()

    await planPage.openTab('archives')
    await expect(planPage.archiveRows).toHaveCount(1)
    await expect(page.getByTestId(`archive-name-${archive.id}`)).toHaveText(name)
    const downloading = page.waitForEvent('download')
    await planPage.downloadButton(archive.id).click()
    const download = await downloading
    expect(download.suggestedFilename()).toBe(name)
    expect(fs.readFileSync(await download.path())).toEqual(content)
    await expect(planPage.archiveBrowseLink(archive.id)).toHaveAttribute('href', `/storages/${storage.id}/browse?prefix=${encodeURIComponent(`${folder}/`)}`)

    await planPage.openTab('storages')
    await expect(planPage.storageBrowseLink(storage.id)).toHaveAttribute('href', `/storages/${storage.id}/browse?prefix=${encodeURIComponent(`${folder}/`)}`)
    await planPage.storageBrowseLink(storage.id).click()
    await expect(new StorageBrowserPage(page).file(name)).toBeVisible()
  })

  test('Run now on a card turns it running, then ok', async ({ page }) => {
    await loginAs(page, 'admin')
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-dash-run-'))
    const storage = await dbHelper.createStorage(`E2E Dash run storage ${Date.now()}`, dir)
    const plan = await dbHelper.createSQLitePlan(`E2E Dash run ${Date.now()}`, storage.id, assertTestDatabasePath(process.env.TEST_DATABASE_PATH))
    const dashboard = new DashboardPage(page)

    await dashboard.gotoDashboard()
    await expect(dashboard.card(plan.id)).toHaveAttribute('data-health', 'never')
    await dashboard.runNow(plan.id)
    await expect(dashboard.card(plan.id)).toHaveAttribute('data-health', 'running')
    await expect(page).toHaveURL(/\/$/)
    await expect(dashboard.card(plan.id)).toHaveAttribute('data-health', 'ok', { timeout: 30_000 })
    await expect(dashboard.day(plan.id, daysAgo(0).date)).toHaveAttribute('data-kind', 'succeeded')
  })

  test('pauses and resumes a plan from its page', async ({ page }) => {
    await loginAs(page, 'admin')
    const storage = await dbHelper.createStorage(`E2E Dash pause storage ${Date.now()}`, os.tmpdir())
    const plan = await dbHelper.createPlan(`E2E Dash pause ${Date.now()}`, storage.id)
    const planPage = new PlanPage(page)

    await planPage.gotoPlan(plan.id)
    await expect(planPage.health).toHaveAttribute('data-health', 'never')
    await expect(planPage.enabledToggle).toHaveAttribute('aria-checked', 'true')
    await planPage.enabledToggle.click()
    await expect(planPage.health).toHaveAttribute('data-health', 'paused')
    await expect(planPage.enabledToggle).toHaveAttribute('aria-checked', 'false')
    await planPage.enabledToggle.click()
    await expect(planPage.health).toHaveAttribute('data-health', 'never')
  })
})

test.describe('The dashboard as a plain user', () => {
  test('shows no plans, and the API refuses them', async ({ page }) => {
    const user = await loginAs(page, 'user')
    const dashboard = new DashboardPage(page)

    await dashboard.gotoDashboard()
    await expect(dashboard.noAccess).toBeVisible()
    await expect(dashboard.cards).toHaveCount(0)
    await dashboard.goto('/plans/1')
    await expect(dashboard.forbidden).toBeVisible()

    const login = await page.request.post(`${API_BASE}/auth/login`, { data: { email: user.email, password: user.password } })
    const { access_token } = (await login.json()).data
    const response = await page.request.get(`${API_BASE}/dashboard`, { headers: { Authorization: `Bearer ${access_token}` } })
    expect(response.status()).toBe(403)
  })
})
