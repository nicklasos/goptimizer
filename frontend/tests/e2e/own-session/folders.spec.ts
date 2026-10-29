import { test, expect } from '@playwright/test'
import fs from 'fs'
import os from 'os'
import path from 'path'
import { loginAs } from '../helpers/test-helpers'
import { DashboardPage } from '../pages/DashboardPage'
import { FolderEditorPage } from '../pages/FolderEditorPage'

let dir: string
let images: string

test.beforeAll(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'goptimizer-e2e-'))
  images = fs.mkdtempSync(path.join(os.tmpdir(), 'goptimizer-e2e-images-'))
  fs.mkdirSync(path.join(images, 'nested'))
  for (const file of ['a.png', 'b.jpg', 'nested/c.gif', 'notes.txt']) {
    fs.writeFileSync(path.join(images, file), file)
  }
})

test.afterAll(() => {
  fs.rmSync(dir, { recursive: true, force: true })
  fs.rmSync(images, { recursive: true, force: true })
})

test.describe('Folders', () => {
  test('are added, paused, edited and deleted', async ({ page }) => {
    await loginAs(page, 'user')
    const dashboard = new DashboardPage(page)
    const editor = new FolderEditorPage(page)
    const name = `E2E Shop ${Date.now()}`
    const renamed = `${name} renamed`

    await dashboard.gotoDashboard()
    await dashboard.addFolderButton.click()
    await editor.fill(name, path.join(dir, 'missing'))
    await editor.saveButton.click()
    await expect(editor.pathInput).toHaveAttribute('aria-invalid', 'true')

    await editor.pathInput.fill(dir)
    await editor.saveButton.click()
    const card = dashboard.folderCard(name)
    await expect(card).toBeVisible()
    await expect(dashboard.modeTag(card)).toHaveAttribute('data-mode', 'lossy')

    await dashboard.enabledSwitch(card).click()
    await expect(dashboard.enabledSwitch(card)).toHaveAttribute('aria-checked', 'false')
    await page.reload()
    await expect(dashboard.enabledSwitch(card)).toHaveAttribute('aria-checked', 'false')

    await dashboard.editButton(card).click()
    await expect(editor.nameInput).toHaveValue(name)
    await editor.nameInput.fill(renamed)
    await editor.openCompression()
    await expect(editor.jpegQualityInput).toBeVisible()
    await editor.chooseMode('lossless')
    await expect(editor.jpegQualityInput).toBeHidden()
    await editor.saveButton.click()
    const renamedCard = dashboard.folderCard(renamed)
    await expect(dashboard.modeTag(renamedCard)).toHaveAttribute('data-mode', 'lossless')

    await dashboard.editButton(renamedCard).click()
    await editor.delete()
    await expect(dashboard.root).toBeVisible()
    await expect(dashboard.folderCard(renamed)).toHaveCount(0)
  })

  // The schedule may queue the first scan just before the click does (409); the card ends with the same count.
  test('Scan now indexes the images and shows the count on the card', async ({ page }) => {
    await loginAs(page, 'user')
    const dashboard = new DashboardPage(page)
    const editor = new FolderEditorPage(page)
    const name = `E2E Images ${Date.now()}`

    await dashboard.gotoDashboard()
    await dashboard.addFolderButton.click()
    await editor.fill(name, images)
    await editor.saveButton.click()
    const card = dashboard.folderCard(name)
    await expect(card).toBeVisible()

    const started = page.waitForResponse((r) => r.url().endsWith('/scans') && r.request().method() === 'POST')
    await dashboard.scan(card, 'full')
    expect([202, 409]).toContain((await started).status())

    await expect(dashboard.imageCount(card)).toHaveAttribute('data-count', '3', { timeout: 20_000 })
    await expect(dashboard.scanStatus(card)).toHaveAttribute('data-status', 'succeeded')
    // The files were written moments ago, within min_age_minutes, so nothing is optimized yet.
    await expect(dashboard.optimized(card)).toHaveAttribute('data-count', '0')
    await expect(dashboard.optimized(card)).toHaveAttribute('data-bytes', '0')
  })
})
