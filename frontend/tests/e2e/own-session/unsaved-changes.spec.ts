import { test, expect } from '@playwright/test'
import { loginAs } from '../helpers/test-helpers'
import { StorageEditorPage } from '../pages/StorageEditorPage'
import { StoragesPage } from '../pages/StoragesPage'

test.describe('Unsaved changes', () => {
  test('prompts when leaving a dirty editor', async ({ page }) => {
    await loginAs(page, 'admin')
    const list = new StoragesPage(page)
    const editor = new StorageEditorPage(page)
    await list.gotoList()
    await list.clickAdd()

    await editor.nameInput.fill('E2E dirty draft')
    await editor.backButton.click()

    await page.getByTestId('unsaved-changes-stay-button').click()
    await expect(editor.root).toBeVisible()
    await expect(editor.nameInput).toHaveValue('E2E dirty draft')

    await editor.backButton.click()
    await page.getByTestId('unsaved-changes-leave-button').click()
    await expect(list.root).toBeVisible({ timeout: 10000 })
  })
})
