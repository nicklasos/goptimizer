import { test, expect } from '@playwright/test'
import fs from 'fs'
import os from 'os'
import path from 'path'
import { utcDay, type SeedImage } from '../helpers/db-helper'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { DashboardPage } from '../pages/DashboardPage'
import { FolderPage } from '../pages/FolderPage'

let root: string
let sequence = 0

test.beforeAll(() => {
  root = fs.mkdtempSync(path.join(os.tmpdir(), 'goptimizer-e2e-page-'))
})

test.afterAll(() => {
  fs.rmSync(root, { recursive: true, force: true })
})

/** A paused folder with the given images: the test API's scheduler and optimizer leave it as seeded. */
async function seedFolder(label: string, images: SeedImage[] = []) {
  sequence += 1
  const dir = path.join(root, `${label}-${sequence}`)
  fs.mkdirSync(dir)
  const name = `E2E ${label} ${Date.now()}-${process.pid}-${sequence}`
  const id = await dbHelper.createFolder({ name, path: dir })
  const imageIds = await dbHelper.createImages(id, images)
  return { id, name, imageIds }
}

test.describe('Folder page', () => {
  test('opens from its card with the folder figures, chart and scans', async ({ page }) => {
    const { id, name } = await seedFolder('Figures', [
      { path: 'shop/a.png', status: 'optimized', size: 40_000, originalSize: 100_000 },
      { path: 'shop/b.jpg', status: 'optimized' },
      { path: 'shop/c.gif', status: 'failed', reason: 'tool_failed: gifsicle exited with 1' },
      { path: 'shop/d.png', status: 'pending' },
    ])
    await dbHelper.createDailyStats(id, [
      { daysAgo: 0, optimized: 2, bytesBefore: 300_000, bytesAfter: 100_000 },
      { daysAgo: 3, optimized: 1, bytesBefore: 50_000, bytesAfter: 30_000 },
    ])
    const [failedScan] = await dbHelper.createScans(id, [
      { kind: 'quick', trigger: 'manual', status: 'failed', startedMinutesAgo: 5, durationSeconds: 2, error: 'the folder was paused' },
      { kind: 'full', trigger: 'schedule', status: 'succeeded', startedMinutesAgo: 90, durationSeconds: 75, filesSeen: 4, filesNew: 4 },
    ])
    await loginAs(page, 'user')
    const dashboard = new DashboardPage(page)
    const folderPage = new FolderPage(page)

    await dashboard.gotoDashboard()
    await expect(dashboard.overview('failed')).toHaveAttribute('data-value', /^\d+$/)
    await expect(dashboard.savingsDays).toHaveCount(30)
    await dashboard.folderLink(dashboard.folderCard(name)).click()

    await expect(page).toHaveURL(new RegExp(`/folders/${id}$`))
    await expect(folderPage.root).toBeVisible()
    await expect(folderPage.stat('saved')).toHaveAttribute('data-value', '220000')
    await expect(folderPage.statCaption('saved')).toBeVisible()
    await expect(folderPage.savingsCard).toHaveAttribute('data-total', '220000')
    await expect(folderPage.savingsCard).toHaveAttribute('data-before', '350000')
    await expect(folderPage.stat('optimized')).toHaveAttribute('data-value', '2')
    await expect(folderPage.stat('pending')).toHaveAttribute('data-value', '1')
    await expect(folderPage.stat('failed')).toHaveAttribute('data-value', '1')

    const today = folderPage.savingsDay(utcDay(0))
    await expect(today).toHaveAttribute('data-bytes', '200000')
    await expect(today).toHaveAttribute('data-before', '300000')
    await expect(today).toHaveAttribute('data-count', '2')
    await expect(folderPage.savingsDay(utcDay(1))).toHaveAttribute('data-bytes', '0')
    await today.hover()
    await expect(page.getByTestId('savings-tooltip')).toBeVisible()

    await expect(folderPage.scanRows).toHaveCount(2)
    await expect(folderPage.scanStatus(failedScan)).toHaveAttribute('data-status', 'failed')
  })

  test('filters images by status and by path, and keeps the filters in the URL', async ({ page }) => {
    const { id, imageIds } = await seedFolder('Filters', [
      { path: 'shop/banners/hero.png', status: 'kept', reason: 'ssim_below_threshold: 0.7100, below 0.8500, at 128,64; lossless: no_saving: 1.2% smaller' },
      { path: 'shop/banners/sale.jpg', status: 'optimized' },
      { path: 'shop/products/spin.png', status: 'skipped', reason: 'apng' },
      { path: 'blog/cover.gif', status: 'kept', reason: 'no_saving: 0.4% smaller' },
      { path: 'blog/old.png', status: 'gone' },
    ])
    const [hero, sale, spin, cover] = imageIds
    await loginAs(page, 'user')
    const folderPage = new FolderPage(page)

    await folderPage.open(id)
    await expect(folderPage.imageRows).toHaveCount(5)
    await expect(folderPage.tab('kept')).toHaveAttribute('data-count', '2')

    await folderPage.tab('kept').click()
    await expect(folderPage.imageRows).toHaveCount(2)
    expect(await folderPage.imageIds()).toEqual([hero, cover])
    await expect(folderPage.reason(hero)).toHaveAttribute('data-reason-code', 'ssim_below_threshold')
    await expect(folderPage.reason(hero)).toHaveAttribute('data-lossless-code', 'no_saving')
    await expect(folderPage.reason(cover)).toHaveAttribute('data-reason-code', 'no_saving')

    await folderPage.tab('skipped').click()
    await expect(folderPage.imageRows).toHaveCount(1)
    await expect(folderPage.reason(spin)).toHaveAttribute('data-reason-code', 'apng')

    await folderPage.tab('all').click()
    await folderPage.searchInput.fill('BANNERS')
    await expect(folderPage.imageRows).toHaveCount(2)
    expect((await folderPage.imageIds()).sort()).toEqual([hero, sale].sort())

    await folderPage.tab('optimized').click()
    await expect(folderPage.imageRows).toHaveCount(1)
    await expect(folderPage.imageStatus(sale)).toHaveAttribute('data-status', 'optimized')

    await page.reload()
    await expect(folderPage.searchInput).toHaveValue('BANNERS')
    await expect(folderPage.selectedTab()).toHaveAttribute('data-testid', 'images-tab-optimized')
    await expect(folderPage.imageRows).toHaveCount(1)
    await expect(folderPage.imageRow(sale)).toBeVisible()
  })

  test('loads the next page of a long list', async ({ page }) => {
    const images: SeedImage[] = Array.from({ length: 60 }, (_, i) => ({ path: `bulk/img-${String(i).padStart(2, '0')}.png`, status: 'skipped', reason: 'symlink' }))
    const { id, imageIds } = await seedFolder('Pages', images)
    await loginAs(page, 'user')
    const folderPage = new FolderPage(page)

    await folderPage.open(id, '?status=skipped')
    await expect(folderPage.imageRows).toHaveCount(50)
    expect(await folderPage.imageIds()).toEqual(imageIds.slice(0, 50))

    await folderPage.loadMoreButton.click()
    await expect(folderPage.imageRows).toHaveCount(60)
    expect(await folderPage.imageIds()).toEqual(imageIds)
    await expect(folderPage.loadMoreButton).toBeHidden()
  })

  test('retries one failed image, then all the others', async ({ page }) => {
    const { id, imageIds } = await seedFolder('Retry', [
      { path: 'a.png', status: 'failed', reason: 'tool_failed: pngquant crashed', attempts: 1, retryInMinutes: 10 },
      { path: 'b.png', status: 'failed', reason: 'swap_failed: rename: permission denied' },
      { path: 'c.jpg', status: 'failed', reason: 'tool_failed: jpegoptim ran past 5m0s' },
      { path: 'd.jpg', status: 'kept', reason: 'no_saving' },
    ])
    const [first, second, third] = imageIds
    await loginAs(page, 'user')
    const folderPage = new FolderPage(page)

    await folderPage.open(id, '?status=failed')
    await expect(folderPage.imageRows).toHaveCount(3)
    await expect(folderPage.retryAllButton).toHaveAttribute('data-count', '3')

    await folderPage.retryButton(first).click()
    await expect(folderPage.imageRow(first)).toHaveCount(0)
    await expect(folderPage.imageRows).toHaveCount(2)
    await expect(folderPage.tab('failed')).toHaveAttribute('data-count', '2')

    await folderPage.retryAllButton.click()
    await expect(folderPage.imageRows).toHaveCount(0)
    await expect(folderPage.tab('failed')).toHaveAttribute('data-count', '0')
    await expect(folderPage.retryAllButton).toBeHidden()
    await expect(folderPage.stat('pending')).toHaveAttribute('data-value', '3')

    await folderPage.tab('pending').click()
    for (const image of [first, second, third]) {
      await expect(folderPage.imageStatus(image)).toHaveAttribute('data-status', 'pending')
    }
  })

  test('shows not found for a folder that does not exist', async ({ page }) => {
    await loginAs(page, 'user')
    const folderPage = new FolderPage(page)

    await folderPage.goto('/folders/999999999')
    await expect(folderPage.notFound).toBeVisible()
    await folderPage.goto('/folders/abc')
    await expect(folderPage.notFound).toBeVisible()
  })
})
