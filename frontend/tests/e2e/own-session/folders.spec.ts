import { test, expect } from '@playwright/test'
import fs from 'fs'
import os from 'os'
import path from 'path'
import { loginAs } from '../helpers/test-helpers'
import { DashboardPage } from '../pages/DashboardPage'
import { FolderEditorPage } from '../pages/FolderEditorPage'

let dir: string

test.beforeAll(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'goptimizer-e2e-'))
})

test.afterAll(() => {
  fs.rmSync(dir, { recursive: true, force: true })
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
})
